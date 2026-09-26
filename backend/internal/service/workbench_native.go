package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"golang.org/x/sync/errgroup"
)

const WorkbenchNativeVersion = "native-metered-v1"

type nativeWorkbenchUsers interface {
	GetByID(context.Context, int64) (*User, error)
}

type nativeWorkbenchKeys interface {
	GetByID(context.Context, int64) (*APIKey, error)
	ListByUserID(context.Context, int64, pagination.PaginationParams, APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error)
}

type nativeWorkbenchPricing interface {
	GetModelPricing(string) *LiteLLMModelPricing
}

// Native authorizations bind a resource selection, not a promised monetary
// ceiling. The ordinary gateway remains the only billing authority.
type nativeWorkbenchAuthorization struct {
	UserID  int64  `json:"u"`
	KeyID   int64  `json:"k"`
	GroupID int64  `json:"g"`
	Model   string `json:"m"`
	Expires int64  `json:"e"`
}

type NativeWorkbench struct {
	users   nativeWorkbenchUsers
	keys    nativeWorkbenchKeys
	pricing nativeWorkbenchPricing
	baseURL string
	client  *http.Client
	secret  []byte
	clock   WorkbenchClock
	mu      sync.Mutex
	active  map[workbench.RunID]context.CancelFunc
}

func ProvideNativeWorkbench(users UserRepository, keys APIKeyRepository, pricing *PricingService, cfg *config.Config) (*NativeWorkbench, error) {
	if cfg == nil || cfg.Server.Port <= 0 || cfg.JWT.Secret == "" {
		return nil, ErrWorkbenchRuntimeUnavailable
	}
	return &NativeWorkbench{
		users: users, keys: keys, pricing: pricing, baseURL: "http://127.0.0.1:" + strconv.Itoa(cfg.Server.Port),
		client: &http.Client{
			Timeout:       3 * time.Minute,
			Transport:     &http.Transport{Proxy: nil},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		secret: []byte(cfg.JWT.Secret), clock: systemWorkbenchClock{},
		active: make(map[workbench.RunID]context.CancelFunc),
	}, nil
}

func (n *NativeWorkbench) ListWorkbenchCapabilities(ctx context.Context, identity workbench.Identity) ([]workbench.Capability, error) {
	if err := identity.Validate(); err != nil {
		return nil, err
	}
	user, err := n.users.GetByID(ctx, int64(identity.ActorID))
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsActive() {
		return nil, ErrWorkbenchCatalogUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	out := []workbench.Capability{}
	for page := 1; ; page++ {
		keys, paging, err := n.keys.ListByUserID(ctx, user.ID, pagination.PaginationParams{Page: page, PageSize: 100}, APIKeyListFilters{Status: StatusAPIKeyActive})
		if err != nil {
			return nil, err
		}
		// Probe each key independently: shared-pool memberships and restrictions
		// differ even within one group. Preserve key/model order despite concurrency.
		batches := make([][]workbench.Capability, len(keys))
		group, probeCtx := errgroup.WithContext(ctx)
		group.SetLimit(6)
		for i, listed := range keys {
			group.Go(func() error {
				key, err := n.keys.GetByID(probeCtx, listed.ID)
				if err != nil || !nativeWorkbenchKeyUsable(key, user.ID) {
					return nil
				}
				keyCtx, keyCancel := context.WithTimeout(probeCtx, 4*time.Second)
				defer keyCancel()
				models, err := n.models(keyCtx, key.Key)
				if err != nil {
					return nil
				}
				for _, model := range models {
					if !n.textModel(model) {
						continue
					}
					auth := nativeWorkbenchAuthorization{UserID: user.ID, KeyID: key.ID, GroupID: *key.GroupID, Model: model, Expires: n.clock.Now().Add(15 * time.Minute).Unix()}
					batches[i] = append(batches[i], n.capability(auth, key.Name))
				}
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, batch := range batches {
			out = append(out, batch...)
		}
		if len(keys) == 0 || (paging != nil && page >= paging.Pages) || (paging == nil && len(keys) < 100) {
			break
		}
	}
	return out, nil
}

func nativeWorkbenchKeyUsable(key *APIKey, userID int64) bool {
	return key != nil && key.UserID == userID && key.IsActive() && !key.IsExpired() &&
		!key.IsQuotaExhausted() && key.GroupID != nil && *key.GroupID > 0 && key.Key != ""
}

func (n *NativeWorkbench) capability(auth nativeWorkbenchAuthorization, keyName string) workbench.Capability {
	body, _ := json.Marshal(auth)
	mac := hmac.New(sha256.New, n.secret)
	_, _ = mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	id := "native." + base64.RawURLEncoding.EncodeToString(body) + "." + signature
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%d:%s", auth.UserID, auth.KeyID, auth.GroupID, auth.Model)))
	capDigest := hex.EncodeToString(digest[:])
	return workbench.Capability{
		ID: workbench.CapabilityID("native_" + capDigest[:24]), Version: WorkbenchNativeVersion,
		Digest: workbench.Digest(capDigest), CanonicalModelID: auth.Model, CanonicalModelVersion: WorkbenchNativeVersion,
		Title: auth.Model + " · " + keyName, Summary: "使用所选资源的现行计费规则，按网关实际用量结算，无固定费用封顶。",
		Protocol: "chat", AcceptedQuoteID: workbench.QuoteID(id), AcceptedQuoteSHA: workbench.Digest(signature),
	}
}

func (n *NativeWorkbench) ResolveWorkbenchCatalog(ctx context.Context, identity workbench.Identity, capID workbench.CapabilityID, quoteID workbench.QuoteID) (WorkbenchCatalogRecord, error) {
	if err := identity.Validate(); err != nil {
		return WorkbenchCatalogRecord{}, err
	}
	parts := strings.Split(string(quoteID), ".")
	if len(parts) != 3 || parts[0] != "native" || len(parts[1]) > 4096 {
		return WorkbenchCatalogRecord{}, ErrWorkbenchCatalogMismatch
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return WorkbenchCatalogRecord{}, ErrWorkbenchCatalogMismatch
	}
	sig, err := hex.DecodeString(parts[2])
	if err != nil {
		return WorkbenchCatalogRecord{}, ErrWorkbenchCatalogMismatch
	}
	mac := hmac.New(sha256.New, n.secret)
	_, _ = mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return WorkbenchCatalogRecord{}, ErrWorkbenchCatalogMismatch
	}
	var auth nativeWorkbenchAuthorization
	if json.Unmarshal(body, &auth) != nil || auth.UserID != int64(identity.ActorID) ||
		auth.Expires <= n.clock.Now().Unix() || strings.TrimSpace(auth.Model) == "" || !n.textModel(auth.Model) {
		return WorkbenchCatalogRecord{}, ErrWorkbenchCatalogUnavailable
	}
	user, err := n.users.GetByID(ctx, auth.UserID)
	if err != nil {
		return WorkbenchCatalogRecord{}, err
	}
	key, err := n.keys.GetByID(ctx, auth.KeyID)
	if err != nil {
		return WorkbenchCatalogRecord{}, err
	}
	if user == nil || !user.IsActive() || !nativeWorkbenchKeyUsable(key, user.ID) || *key.GroupID != auth.GroupID {
		return WorkbenchCatalogRecord{}, ErrWorkbenchCatalogUnavailable
	}
	capability := n.capability(auth, key.Name)
	if capability.ID != capID {
		return WorkbenchCatalogRecord{}, ErrWorkbenchCatalogMismatch
	}
	return WorkbenchCatalogRecord{
		Capability: capability, User: user, APIKey: key,
		Quote: WorkbenchAcceptedQuote{ID: quoteID, SHA256: capability.AcceptedQuoteSHA, ModelID: auth.Model,
			ModelVersion: WorkbenchNativeVersion, GroupID: auth.GroupID, ExpiresAt: time.Unix(auth.Expires, 0)},
	}, nil
}

func (n *NativeWorkbench) textModel(model string) bool {
	if n.pricing != nil {
		if price := n.pricing.GetModelPricing(model); price != nil && price.Mode != "" {
			switch strings.ToLower(price.Mode) {
			case "chat", "completion", "responses":
				return true
			default:
				return false
			}
		}
	}
	// Legacy manifests omit modality. Exclude known dedicated non-text model
	// families; unknown aliases still undergo the gateway's protocol checks.
	name := strings.ToLower(model)
	for _, marker := range []string{"gpt-image", "dall-e", "imagen", "imagine-image", "imagine-video", "sora", "veo-", "embedding", "whisper", "tts-", "rerank"} {
		if strings.Contains(name, marker) {
			return false
		}
	}
	return true
}

var _ WorkbenchCatalogSource = (*NativeWorkbench)(nil)
var _ WorkbenchCanonicalExecutor = (*NativeWorkbench)(nil)
