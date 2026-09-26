package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolOwnerOnboarding_ReadinessRejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "accountId", Value: "2"}}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Request = httptest.NewRequest(http.MethodPost, "/pools/1/accounts/2/native-readiness", strings.NewReader(`{"expected_config_version":1,"extra":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	NewBizDecipherHandler(nil).VerifySharedPoolNativeReadiness(c)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestSharedPoolOwnerOnboarding_ReadinessRequiresExpectedConfigVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "accountId", Value: "2"}}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Request = httptest.NewRequest(http.MethodPost, "/pools/1/accounts/2/native-readiness", strings.NewReader(`{"expected_config_version":0}`))
	c.Request.Header.Set("Content-Type", "application/json")
	NewBizDecipherHandler(nil).VerifySharedPoolNativeReadiness(c)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestSharedPoolOwnerOnboarding_ReadinessResponseRedactsNativeCredentials(t *testing.T) {
	account := &service.SharedPoolAccount{
		ID: 7, NativeBindingState: "attached", NativeModels: []string{"model-a"},
		NativeConnectionStatus:    service.NativeConnectionAuthenticatedMetadataReachable,
		BillingActivationRequired: true, UpstreamBaseURL: "https://sensitive-upstream.example",
		KeyPreview: "sensitive-key", ProxyURL: "https://sensitive-proxy.example", CredentialFingerprint: "sensitive-fingerprint",
	}
	body, err := json.Marshal(newSharedPoolNativeAccountResponse(account))
	require.NoError(t, err)
	require.NotContains(t, string(body), "sensitive")
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Contains(t, payload, "native_models")
	require.Contains(t, payload, "native_connection_status")
	require.NotContains(t, payload, "upstream_base_url")
	require.NotContains(t, payload, "key_preview")
	require.NotContains(t, payload, "proxy_url")
}

func TestSharedPoolOwnerOnboarding_ActivationValidatesCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name, body, header string
		status             int
	}{
		{"unknown field", `{"operation_id":"op","expected_config_version":1,"listed":true}`, "", http.StatusUnprocessableEntity},
		{"missing version", `{"operation_id":"op"}`, "", http.StatusUnprocessableEntity},
		{"negative version", `{"operation_id":"op","expected_config_version":-1}`, "", http.StatusUnprocessableEntity},
		{"missing operation", `{"expected_config_version":1}`, "", http.StatusUnprocessableEntity},
		{"mismatched operation", `{"operation_id":"op","expected_config_version":1}`, "other", http.StatusUnprocessableEntity},
		{"null body", `null`, "", http.StatusUnprocessableEntity},
		{"array body", `[]`, "", http.StatusUnprocessableEntity},
		{"oversized body", `{"operation_id":"` + strings.Repeat("x", maxSharedPoolOwnerRequestBytes) + `"}`, "", http.StatusUnprocessableEntity},
		// A valid command reaches the unavailable service rather than failing field validation.
		{"body operation", `{"operation_id":"op","expected_config_version":1}`, "", http.StatusServiceUnavailable},
		{"header operation", `{"expected_config_version":1}`, "op", http.StatusServiceUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Params = gin.Params{{Key: "id", Value: "1"}}
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
			c.Request = httptest.NewRequest(http.MethodPost, "/pools/1/native-activation", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("Idempotency-Key", tc.header)
			NewBizDecipherHandler(nil).ActivateSharedPoolNativeBilling(c)
			require.Equal(t, tc.status, rec.Code)
		})
	}
}
