package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imroc/req/v3"
)

const (
	misskeyBridgeIssuer       = "https://api.bizdecipher.com"
	misskeyBridgeDCBase       = "https://dc.hhhl.cc"
	misskeyBridgeClientID     = "bizdecipher-shared-gateway"
	misskeyBridgeAppName      = "Aura API Login"
	misskeyBridgeCallbackPath = "/api/misskey-proxy/callback"
	misskeyBridgeTTL          = 10 * time.Minute
)

type misskeyBridgeAuthSession struct {
	ID          string
	MiAuthID    string
	ClientID    string
	RedirectURI string
	State       string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type misskeyBridgeTokenSession struct {
	AccessToken string
	User        misskeyBridgeUser
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type misskeyBridgeCompletedSession struct {
	RedirectURI string
	State       string
	User        misskeyBridgeUser
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type misskeyBridgeUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Avatar   string `json:"avatarUrl"`
}

var misskeyBridgeStore = struct {
	sync.Mutex
	authByID       map[string]misskeyBridgeAuthSession
	authByMiAuthID map[string]string
	completedByID  map[string]misskeyBridgeCompletedSession
	tokenByID      map[string]misskeyBridgeTokenSession
}{
	authByID:       map[string]misskeyBridgeAuthSession{},
	authByMiAuthID: map[string]string{},
	completedByID:  map[string]misskeyBridgeCompletedSession{},
	tokenByID:      map[string]misskeyBridgeTokenSession{},
}

// MisskeyBridgeAuthorize exposes an OAuth-like authorize endpoint backed by dc.hhhl.cc MiAuth.
// It allows Sub2API's existing generic OIDC client to login without requiring a dc OAuth app client_id.
func (h *AuthHandler) MisskeyBridgeAuthorize(c *gin.Context) {
	clientID := strings.TrimSpace(c.Query("client_id"))
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	state := strings.TrimSpace(c.Query("state"))
	responseType := strings.TrimSpace(c.Query("response_type"))

	if clientID == "" || redirectURI == "" || state == "" || responseType != "code" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "missing or invalid oauth parameters"})
		return
	}
	if clientID != misskeyBridgeClientID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client", "error_description": "invalid client_id"})
		return
	}
	if !isAllowedMisskeyBridgeRedirectURI(redirectURI) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "redirect_uri is not allowed"})
		return
	}

	bridgeID, err := randomURLToken(24)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error", "error_description": "failed to create bridge session"})
		return
	}
	miauthID, err := randomURLToken(24)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error", "error_description": "failed to create miauth session"})
		return
	}

	now := time.Now()
	misskeyBridgeStore.Lock()
	misskeyBridgePruneLocked(now)
	misskeyBridgeStore.authByID[bridgeID] = misskeyBridgeAuthSession{
		ID:          bridgeID,
		MiAuthID:    miauthID,
		ClientID:    clientID,
		RedirectURI: redirectURI,
		State:       state,
		CreatedAt:   now,
		ExpiresAt:   now.Add(misskeyBridgeTTL),
	}
	misskeyBridgeStore.authByMiAuthID[miauthID] = bridgeID
	misskeyBridgeStore.Unlock()

	callbackURL := misskeyBridgeIssuer + misskeyBridgeCallbackPath + "?bridge_session=" + url.QueryEscape(bridgeID)
	miAuthURL := misskeyBridgeDCBase + "/miauth/" + url.PathEscape(miauthID)
	q := url.Values{}
	q.Set("name", misskeyBridgeAppName)
	q.Set("callback", callbackURL)
	q.Set("permission", "read:account")
	c.Redirect(http.StatusFound, miAuthURL+"?"+q.Encode())
}

// MisskeyBridgeCallback receives dc.hhhl.cc MiAuth callback, checks the session, then redirects back to Sub2API OIDC callback.
func (h *AuthHandler) MisskeyBridgeCallback(c *gin.Context) {
	bridgeID := strings.TrimSpace(c.Query("bridge_session"))
	miAuthID := strings.TrimSpace(c.Query("session"))

	now := time.Now()
	misskeyBridgeStore.Lock()
	if bridgeID == "" && miAuthID != "" {
		bridgeID = misskeyBridgeStore.authByMiAuthID[miAuthID]
	}
	if bridgeID == "" {
		misskeyBridgeStore.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "missing bridge_session"})
		return
	}
	session, ok := misskeyBridgeStore.authByID[bridgeID]
	if !ok || now.After(session.ExpiresAt) {
		if ok {
			delete(misskeyBridgeStore.authByID, bridgeID)
			delete(misskeyBridgeStore.authByMiAuthID, session.MiAuthID)
		}
		completed, completedOK := misskeyBridgeStore.completedByID[bridgeID]
		misskeyBridgeStore.Unlock()
		if completedOK && !now.After(completed.ExpiresAt) {
			redirectMisskeyBridgeCompleted(c, completed)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "bridge session expired"})
		return
	}
	misskeyBridgeStore.Unlock()

	user, err := checkDCMiAuthSession(session.MiAuthID)
	if err != nil {
		redirectBridgeError(c, session.RedirectURI, session.State, "miauth_failed", err.Error())
		return
	}
	if strings.TrimSpace(user.ID) == "" {
		redirectBridgeError(c, session.RedirectURI, session.State, "missing_user", "dc user id missing")
		return
	}

	code, err := randomURLToken(32)
	if err != nil {
		redirectBridgeError(c, session.RedirectURI, session.State, "server_error", "failed to create oauth code")
		return
	}
	accessToken, err := randomURLToken(32)
	if err != nil {
		redirectBridgeError(c, session.RedirectURI, session.State, "server_error", "failed to create access token")
		return
	}

	misskeyBridgeStore.Lock()
	delete(misskeyBridgeStore.authByID, bridgeID)
	delete(misskeyBridgeStore.authByMiAuthID, session.MiAuthID)
	misskeyBridgeStore.completedByID[bridgeID] = misskeyBridgeCompletedSession{
		RedirectURI: session.RedirectURI,
		State:       session.State,
		User:        user,
		CreatedAt:   now,
		ExpiresAt:   now.Add(misskeyBridgeTTL),
	}
	misskeyBridgeStore.tokenByID[code] = misskeyBridgeTokenSession{
		AccessToken: accessToken,
		User:        user,
		CreatedAt:   now,
		ExpiresAt:   now.Add(misskeyBridgeTTL),
	}
	misskeyBridgeStore.tokenByID[accessToken] = misskeyBridgeTokenSession{
		AccessToken: accessToken,
		User:        user,
		CreatedAt:   now,
		ExpiresAt:   now.Add(misskeyBridgeTTL),
	}
	misskeyBridgeStore.Unlock()

	redirectBridgeCode(c, session.RedirectURI, session.State, code)
}

// MisskeyBridgeToken exchanges an OAuth code for an access token.
func (h *AuthHandler) MisskeyBridgeToken(c *gin.Context) {
	grantType := strings.TrimSpace(c.PostForm("grant_type"))
	code := strings.TrimSpace(c.PostForm("code"))
	clientID := strings.TrimSpace(c.PostForm("client_id"))
	if grantType != "authorization_code" || code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "invalid token request"})
		return
	}
	if clientID != "" && clientID != misskeyBridgeClientID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client", "error_description": "invalid client_id"})
		return
	}

	now := time.Now()
	misskeyBridgeStore.Lock()
	misskeyBridgePruneLocked(now)
	session, ok := misskeyBridgeStore.tokenByID[code]
	if ok {
		delete(misskeyBridgeStore.tokenByID, code)
	}
	misskeyBridgeStore.Unlock()
	if !ok || now.After(session.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant", "error_description": "code expired or invalid"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": session.AccessToken,
		"token_type":   "Bearer",
		"expires_in":   int(misskeyBridgeTTL.Seconds()),
		"scope":        "openid profile",
	})
}

// MisskeyBridgeUserInfo returns OIDC-like userinfo claims for Sub2API's OIDC client.
func (h *AuthHandler) MisskeyBridgeUserInfo(c *gin.Context) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	parts := strings.Fields(auth)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token", "error_description": "missing bearer token"})
		return
	}
	accessToken := strings.TrimSpace(parts[1])

	now := time.Now()
	misskeyBridgeStore.Lock()
	misskeyBridgePruneLocked(now)
	session, ok := misskeyBridgeStore.tokenByID[accessToken]
	misskeyBridgeStore.Unlock()
	if !ok || now.After(session.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token", "error_description": "token expired or invalid"})
		return
	}

	username := strings.TrimSpace(session.User.Username)
	if username == "" {
		username = strings.TrimSpace(session.User.Name)
	}
	if username == "" {
		username = "dc_" + strings.TrimSpace(session.User.ID)
	}
	c.JSON(http.StatusOK, gin.H{
		"sub":                strings.TrimSpace(session.User.ID),
		"preferred_username": username,
		"name":               firstNonEmpty(strings.TrimSpace(session.User.Name), username),
		"picture":            strings.TrimSpace(session.User.Avatar),
		"email_verified":     true,
	})
}

func checkDCMiAuthSession(miauthID string) (misskeyBridgeUser, error) {
	client := req.C().SetTimeout(20 * time.Second)
	if proxyURL := strings.TrimSpace(os.Getenv("MISSKEY_BRIDGE_PROXY_URL")); proxyURL != "" {
		client.SetProxyURL(proxyURL)
	}
	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]any{}).
		Post(misskeyBridgeDCBase + "/api/miauth/" + url.PathEscape(miauthID) + "/check")
	if err != nil {
		return misskeyBridgeUser{}, err
	}
	if !resp.IsSuccessState() && strings.Contains(strings.ToLower(resp.GetHeader("Content-Type")), "text/html") {
		return misskeyBridgeUser{}, fmt.Errorf("dc miauth check returned HTML status %d", resp.StatusCode)
	}
	var payload struct {
		OK    bool              `json:"ok"`
		Token string            `json:"token"`
		User  misskeyBridgeUser `json:"user"`
		Error string            `json:"error"`
	}
	if err := json.Unmarshal(resp.Bytes(), &payload); err != nil {
		return misskeyBridgeUser{}, err
	}
	if !resp.IsSuccessState() || !payload.OK {
		if strings.TrimSpace(payload.Error) != "" {
			return misskeyBridgeUser{}, &misskeyBridgeProviderError{Message: payload.Error}
		}
		return misskeyBridgeUser{}, &misskeyBridgeProviderError{Message: "dc miauth check failed"}
	}
	return payload.User, nil
}

type misskeyBridgeProviderError struct{ Message string }

func (e *misskeyBridgeProviderError) Error() string { return e.Message }

func randomURLToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base64.URLEncoding.EncodeToString(buf), "="), nil
}

func isAllowedMisskeyBridgeRedirectURI(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	if u.Host != "bizdecipher.com" && u.Host != "api.bizdecipher.com" {
		return false
	}
	return u.Path == "/api/v1/auth/oauth/oidc/callback"
}

func redirectBridgeCode(c *gin.Context, redirectURI, state, code string) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "invalid redirect_uri"})
		return
	}
	q := u.Query()
	q.Set("code", code)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, u.String())
}

func redirectMisskeyBridgeCompleted(c *gin.Context, completed misskeyBridgeCompletedSession) {
	code, err := randomURLToken(32)
	if err != nil {
		redirectBridgeError(c, completed.RedirectURI, completed.State, "server_error", "failed to recreate oauth code")
		return
	}
	accessToken, err := randomURLToken(32)
	if err != nil {
		redirectBridgeError(c, completed.RedirectURI, completed.State, "server_error", "failed to recreate access token")
		return
	}
	now := time.Now()
	misskeyBridgeStore.Lock()
	misskeyBridgeStore.tokenByID[code] = misskeyBridgeTokenSession{
		AccessToken: accessToken,
		User:        completed.User,
		CreatedAt:   now,
		ExpiresAt:   now.Add(misskeyBridgeTTL),
	}
	misskeyBridgeStore.tokenByID[accessToken] = misskeyBridgeTokenSession{
		AccessToken: accessToken,
		User:        completed.User,
		CreatedAt:   now,
		ExpiresAt:   now.Add(misskeyBridgeTTL),
	}
	misskeyBridgeStore.Unlock()
	redirectBridgeCode(c, completed.RedirectURI, completed.State, code)
}

func redirectBridgeError(c *gin.Context, redirectURI, state, code, desc string) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": code, "error_description": desc})
		return
	}
	q := u.Query()
	q.Set("error", code)
	q.Set("error_description", desc)
	if strings.TrimSpace(state) != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, u.String())
}

func misskeyBridgePruneLocked(now time.Time) {
	for k, v := range misskeyBridgeStore.authByID {
		if now.After(v.ExpiresAt) {
			delete(misskeyBridgeStore.authByID, k)
			delete(misskeyBridgeStore.authByMiAuthID, v.MiAuthID)
		}
	}
	for k, v := range misskeyBridgeStore.completedByID {
		if now.After(v.ExpiresAt) {
			delete(misskeyBridgeStore.completedByID, k)
		}
	}
	for k, v := range misskeyBridgeStore.tokenByID {
		if now.After(v.ExpiresAt) {
			delete(misskeyBridgeStore.tokenByID, k)
		}
	}
}
