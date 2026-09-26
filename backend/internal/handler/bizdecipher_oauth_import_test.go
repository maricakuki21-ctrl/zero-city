package handler

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type sharedPoolOAuthImportRepoStub struct {
	service.BizDecipherRepository
	createdInputs []service.SharedPoolAccountInput
}

func (r *sharedPoolOAuthImportRepoStub) FindSharedPoolAccountByFingerprint(context.Context, int64, int64, string) (*service.SharedPoolAccount, error) {
	return nil, sql.ErrNoRows
}

func (r *sharedPoolOAuthImportRepoStub) CreateSharedPoolAccount(_ context.Context, input service.SharedPoolAccountInput) (*service.SharedPoolAccount, error) {
	r.createdInputs = append(r.createdInputs, input)
	return &service.SharedPoolAccount{
		ID:                  901,
		PoolID:              input.PoolID,
		OwnerID:             input.OwnerID,
		Name:                input.Name,
		Provider:            input.Provider,
		AuthType:            input.AuthType,
		HasOAuthCredentials: input.CredentialsEncrypted != "",
		Schedulable:         input.Schedulable,
		Status:              input.Status,
	}, nil
}

type sharedPoolOAuthImportEncryptor struct {
	plaintexts []string
}

func (e *sharedPoolOAuthImportEncryptor) Encrypt(plaintext string) (string, error) {
	e.plaintexts = append(e.plaintexts, plaintext)
	return "fixture-ciphertext", nil
}

func (e *sharedPoolOAuthImportEncryptor) Decrypt(string) (string, error) {
	return "", nil
}

func TestImportSharedPoolOAuthPackageResponseRedactsCredentials(t *testing.T) {
	const (
		accessToken  = "fixture-access-token"
		refreshToken = "fixture-refresh-token"
		idToken      = "fixture-id-token"
	)

	repo := &sharedPoolOAuthImportRepoStub{}
	encryptor := &sharedPoolOAuthImportEncryptor{}
	svc := service.NewBizDecipherService(repo, nil, nil)
	svc.SetSecretEncryptor(encryptor)
	h := NewBizDecipherHandler(svc)

	payload := `{
		"data": {
			"type": "sub2api-data",
			"version": 1,
			"accounts": [
				{
					"name": "Fixture OAuth",
					"platform": "openai",
					"type": "oauth",
					"credentials": {
						"access_token": "` + accessToken + `",
						"refresh_token": "` + refreshToken + `",
						"id_token": "` + idToken + `",
						"account_id": "fixture-account",
						"email": "fixture-a@example.com"
					},
					"extra": {"model": "gpt-fixture"}
				},
				{
					"name": "Fixture OAuth duplicate",
					"platform": "openai",
					"type": "oauth",
					"credentials": {
						"access_token": "another-fixture-token",
						"account_id": "fixture-account",
						"email": "fixture-a@example.com"
					}
				}
			]
		},
		"update_existing": true
	}`

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/biz/pools/77/accounts/import-oauth-package", bytes.NewBufferString(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "77"}}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 88})

	h.ImportSharedPoolOAuthPackage(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"created":1`)
	require.Contains(t, w.Body.String(), `"skipped":1`)
	require.NotContains(t, w.Body.String(), accessToken)
	require.NotContains(t, w.Body.String(), refreshToken)
	require.NotContains(t, w.Body.String(), idToken)
	require.NotContains(t, w.Body.String(), "another-fixture-token")
	require.NotContains(t, w.Body.String(), "fixture-ciphertext")
	require.NotContains(t, w.Body.String(), "credentials")
	require.Len(t, repo.createdInputs, 1)
	require.Equal(t, "fixture-ciphertext", repo.createdInputs[0].CredentialsEncrypted)
	require.Empty(t, repo.createdInputs[0].UpstreamAPIKey)
	require.Len(t, encryptor.plaintexts, 2)
	require.True(t, strings.Contains(encryptor.plaintexts[0], accessToken))
	require.True(t, strings.Contains(encryptor.plaintexts[0], refreshToken))
}

func TestImportSharedPoolOAuthPackageKeepsStableIdentityAcrossTokenMetadataChanges(t *testing.T) {
	repo := &sharedPoolOAuthImportRepoStub{}
	encryptor := &sharedPoolOAuthImportEncryptor{}
	svc := service.NewBizDecipherService(repo, nil, nil)
	svc.SetSecretEncryptor(encryptor)

	result, err := svc.ImportSharedPoolOAuthPackage(context.Background(), service.ImportSharedPoolOAuthPackageInput{
		PoolID:  77,
		OwnerID: 88,
		Data: service.SharedPoolOAuthDataPackage{
			Type: "sub2api-data",
			Accounts: []service.SharedPoolOAuthDataAccount{
				{
					Name:     "Fixture with account and user",
					Platform: "openai",
					Type:     "oauth",
					Credentials: map[string]any{
						"access_token": "fixture-access-a",
						"account_id":   "fixture-account",
						"user_id":      "fixture-user",
					},
				},
				{
					Name:     "Fixture same identity with extra metadata",
					Platform: "openai",
					Type:     "oauth",
					Credentials: map[string]any{
						"access_token": "fixture-access-b",
						"account_id":   "fixture-account",
						"user_id":      "fixture-user",
						"email":        "same-user@example.com",
					},
				},
			},
		},
		UpdateExisting: true,
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Equal(t, 1, result.Skipped)
	require.Len(t, repo.createdInputs, 1)
}

func TestImportSharedPoolOAuthPackageCreatesDistinctAccountsForSharedAccountID(t *testing.T) {
	repo := &sharedPoolOAuthImportRepoStub{}
	encryptor := &sharedPoolOAuthImportEncryptor{}
	svc := service.NewBizDecipherService(repo, nil, nil)
	svc.SetSecretEncryptor(encryptor)

	result, err := svc.ImportSharedPoolOAuthPackage(context.Background(), service.ImportSharedPoolOAuthPackageInput{
		PoolID:  77,
		OwnerID: 88,
		Data: service.SharedPoolOAuthDataPackage{
			Type: "sub2api-data",
			Accounts: []service.SharedPoolOAuthDataAccount{
				{
					Name:     "Seat A",
					Platform: "openai",
					Type:     "oauth",
					Credentials: map[string]any{
						"access_token": "fixture-access-a",
						"account_id":   "shared-account",
						"email":        "seat-a@example.com",
						"expired":      "2026-07-25T10:12:06.000Z",
					},
				},
				{
					Name:     "Seat B",
					Platform: "openai",
					Type:     "oauth",
					Credentials: map[string]any{
						"access_token": "fixture-access-b",
						"account_id":   "shared-account",
						"email":        "seat-b@example.com",
						"expired":      "2026-07-25T10:12:06.000Z",
					},
				},
			},
		},
		UpdateExisting: true,
	})

	require.NoError(t, err)
	require.Equal(t, 2, result.Created)
	require.Equal(t, 0, result.Skipped)
	require.Len(t, repo.createdInputs, 2)
	require.NotEqual(t, repo.createdInputs[0].CredentialFingerprint, repo.createdInputs[1].CredentialFingerprint)
	require.NotNil(t, repo.createdInputs[0].ExpiresAt)
	require.NotNil(t, repo.createdInputs[1].ExpiresAt)
}

func TestImportSharedPoolOAuthPackageWithoutRefreshTokenForcesExpiryPause(t *testing.T) {
	expiresAt := int64(2_000_000_000)
	requestedAutoPause := false
	repo := &sharedPoolOAuthImportRepoStub{}
	encryptor := &sharedPoolOAuthImportEncryptor{}
	svc := service.NewBizDecipherService(repo, nil, nil)
	svc.SetSecretEncryptor(encryptor)

	result, err := svc.ImportSharedPoolOAuthPackage(context.Background(), service.ImportSharedPoolOAuthPackageInput{
		PoolID:  77,
		OwnerID: 88,
		Data: service.SharedPoolOAuthDataPackage{
			Type: "sub2api-data",
			Accounts: []service.SharedPoolOAuthDataAccount{{
				Name:     "Fixture without refresh token",
				Platform: "openai",
				Type:     "oauth",
				Credentials: map[string]any{
					"access_token": "fixture-access-token",
					"account_id":   "fixture-account-no-refresh",
				},
				ExpiresAt:          &expiresAt,
				AutoPauseOnExpired: &requestedAutoPause,
			}},
		},
		UpdateExisting: true,
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Len(t, result.Warnings, 1)
	require.Contains(t, result.Warnings[0].Message, "no refresh_token")
	require.Len(t, repo.createdInputs, 1)
	require.True(t, repo.createdInputs[0].AutoPauseOnExpired)
	require.True(t, repo.createdInputs[0].AutoPauseOnExpiredSet)
	require.NotNil(t, repo.createdInputs[0].ExpiresAt)
	require.True(t, repo.createdInputs[0].Schedulable)
}

func TestImportSharedPoolOAuthPackageWithRefreshTokenStillStopsAfterExpiry(t *testing.T) {
	expiresAt := int64(2_000_000_000)
	requestedAutoPause := false
	repo := &sharedPoolOAuthImportRepoStub{}
	encryptor := &sharedPoolOAuthImportEncryptor{}
	svc := service.NewBizDecipherService(repo, nil, nil)
	svc.SetSecretEncryptor(encryptor)

	result, err := svc.ImportSharedPoolOAuthPackage(context.Background(), service.ImportSharedPoolOAuthPackageInput{
		PoolID:  77,
		OwnerID: 88,
		Data: service.SharedPoolOAuthDataPackage{
			Type: "sub2api-data",
			Accounts: []service.SharedPoolOAuthDataAccount{{
				Name:     "Fixture with refresh token",
				Platform: "openai",
				Type:     "oauth",
				Credentials: map[string]any{
					"access_token":  "fixture-access-token",
					"refresh_token": "fixture-refresh-token",
					"account_id":    "fixture-account-with-refresh",
				},
				ExpiresAt:          &expiresAt,
				AutoPauseOnExpired: &requestedAutoPause,
			}},
		},
		UpdateExisting: true,
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Empty(t, result.Warnings)
	require.Len(t, repo.createdInputs, 1)
	require.True(t, repo.createdInputs[0].AutoPauseOnExpired, "OAuth cannot remain routable after access token expiry until scheduler refresh is implemented")
}

func TestImportSharedPoolOAuthPackageRequiresAuthBeforeReadingCredentials(t *testing.T) {
	h := NewBizDecipherHandler(service.NewBizDecipherService(&sharedPoolOAuthImportRepoStub{}, nil, nil))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/biz/pools/77/accounts/import-oauth-package", strings.NewReader(`{"data":{"accounts":[]}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "77"}}

	h.ImportSharedPoolOAuthPackage(c)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.NotContains(t, w.Body.String(), "accounts")
}
