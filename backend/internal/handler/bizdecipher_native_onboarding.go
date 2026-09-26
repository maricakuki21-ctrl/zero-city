package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const maxSharedPoolOwnerRequestBytes = 1 << 20

var (
	nativeDraftFields = map[string]struct{}{
		"name": {}, "description": {}, "supply_mode": {}, "operation_id": {},
	}
	nativeAccountFields = map[string]struct{}{
		"operation_id": {}, "name": {}, "description": {}, "provider": {}, "auth_type": {},
		"upstream_base_url": {}, "upstream_api_key": {}, "credentials": {},
	}
	nativeAccountImportFields = map[string]struct{}{"items": {}}
	nativeRepairFields        = map[string]struct{}{
		"repair_operation_id": {}, "expected_config_version": {}, "provider": {}, "auth_type": {},
		"upstream_base_url": {}, "upstream_api_key": {}, "credentials": {},
	}
)

type sharedPoolNativeAccountResponse struct {
	ID                         int64      `json:"id"`
	NativeOperationID          string     `json:"native_operation_id,omitempty"`
	NativeBindingState         string     `json:"native_binding_state,omitempty"`
	NativeBindingStep          string     `json:"native_binding_step,omitempty"`
	NativeErrorCode            string     `json:"native_error_code,omitempty"`
	NativeErrorMessage         string     `json:"native_error_message,omitempty"`
	NativeModels               []string   `json:"native_models,omitempty"`
	NativeModelsVerifiedAt     *time.Time `json:"native_models_verified_at,omitempty"`
	NativeConnectionStatus     string     `json:"native_connection_status,omitempty"`
	NativeConnectionVerifiedAt *time.Time `json:"native_connection_verified_at,omitempty"`
	NativeEvidenceStale        bool       `json:"native_evidence_stale"`
	BillingActivationRequired  bool       `json:"billing_activation_required"`
}

type sharedPoolNativeImportItemResponse struct {
	Index   int                              `json:"index"`
	Name    string                           `json:"name"`
	Created bool                             `json:"created"`
	Account *sharedPoolNativeAccountResponse `json:"account,omitempty"`
	Error   string                           `json:"error,omitempty"`
}

type sharedPoolNativeImportResponse struct {
	Total   int                                  `json:"total"`
	Created int                                  `json:"created"`
	Failed  int                                  `json:"failed"`
	Items   []sharedPoolNativeImportItemResponse `json:"items"`
}

func (req *sharedPoolAccountRequest) UnmarshalJSON(data []byte) error {
	type requestAlias sharedPoolAccountRequest
	var decoded requestAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*req = sharedPoolAccountRequest(decoded)
	req.presentFields = fields
	return nil
}

func bindSharedPoolOwnerJSON(c *gin.Context, dst any) (map[string]json.RawMessage, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, errors.New("request body is required")
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxSharedPoolOwnerRequestBytes+1))
	if err != nil {
		return nil, errors.New("request body is unreadable")
	}
	if len(body) == 0 || len(body) > maxSharedPoolOwnerRequestBytes {
		return nil, errors.New("request body is invalid")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return nil, errors.New("request body must be a JSON object")
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return nil, errors.New("request body has invalid field values")
	}
	return fields, nil
}

func requireNativeOwnerFields(fields map[string]json.RawMessage, allowed map[string]struct{}) error {
	for field := range fields {
		if _, ok := allowed[field]; !ok {
			return service.ErrOwnerNativeFieldRejected
		}
	}
	return nil
}

func reconcileNativeOperationID(c *gin.Context, bodyID string) (string, error) {
	bodyID = strings.TrimSpace(bodyID)
	headerID := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if bodyID != "" && headerID != "" && bodyID != headerID {
		return "", service.ErrOwnerNativeFieldRejected
	}
	if bodyID == "" {
		bodyID = headerID
	}
	if bodyID == "" {
		return "", service.ErrOwnerNativeFieldRejected
	}
	return bodyID, nil
}

func isNativeSharedPoolAccountRequest(c *gin.Context, req sharedPoolAccountRequest, fields map[string]json.RawMessage) bool {
	if strings.TrimSpace(req.OperationID) != "" || strings.TrimSpace(c.GetHeader("Idempotency-Key")) != "" {
		return true
	}
	_, hasCredentials := fields["credentials"]
	return hasCredentials
}

func hasNativeSharedPoolAccountItems(items []sharedPoolAccountRequest) bool {
	for _, item := range items {
		_, hasCredentials := item.presentFields["credentials"]
		if strings.TrimSpace(item.OperationID) != "" || hasCredentials {
			return true
		}
	}
	return false
}

func (h *BizDecipherHandler) onboardSharedPoolNativeAccount(
	c *gin.Context,
	poolID int64,
	ownerID int64,
	req sharedPoolAccountRequest,
	fields map[string]json.RawMessage,
) (*service.SharedPoolAccount, error) {
	if err := requireNativeOwnerFields(fields, nativeAccountFields); err != nil {
		return nil, err
	}
	operationID, err := reconcileNativeOperationID(c, req.OperationID)
	if err != nil {
		return nil, err
	}
	input, err := service.NewSharedPoolNativeAccountInput(service.SharedPoolNativeAccountRequest{
		PoolID:          poolID,
		OwnerID:         ownerID,
		OperationID:     operationID,
		Name:            req.Name,
		Description:     req.Description,
		Provider:        req.Provider,
		AuthType:        req.AuthType,
		UpstreamBaseURL: req.UpstreamBaseURL,
		APIKey:          req.UpstreamAPIKey,
		Credentials:     req.Credentials,
	})
	if err != nil {
		return nil, err
	}
	return h.bizService.OnboardSharedPoolNativeAccount(c.Request.Context(), input)
}

func (h *BizDecipherHandler) importSharedPoolNativeAccounts(
	c *gin.Context,
	poolID int64,
	ownerID int64,
	items []sharedPoolAccountRequest,
) (*sharedPoolNativeImportResponse, error) {
	if len(items) == 0 || len(items) > 100 || strings.TrimSpace(c.GetHeader("Idempotency-Key")) != "" {
		return nil, service.ErrOwnerNativeFieldRejected
	}
	result := &sharedPoolNativeImportResponse{
		Total: len(items),
		Items: make([]sharedPoolNativeImportItemResponse, 0, len(items)),
	}
	for index, item := range items {
		itemResult := sharedPoolNativeImportItemResponse{Index: index, Name: strings.TrimSpace(item.Name)}
		if err := requireNativeOwnerFields(item.presentFields, nativeAccountFields); err != nil {
			return nil, err
		}
		if strings.TrimSpace(item.OperationID) == "" {
			return nil, service.ErrOwnerNativeFieldRejected
		}
		input, err := service.NewSharedPoolNativeAccountInput(service.SharedPoolNativeAccountRequest{
			PoolID: poolID, OwnerID: ownerID, OperationID: item.OperationID,
			Name: item.Name, Description: item.Description, Provider: item.Provider, AuthType: item.AuthType,
			UpstreamBaseURL: item.UpstreamBaseURL, APIKey: item.UpstreamAPIKey, Credentials: item.Credentials,
		})
		if err == nil {
			var account *service.SharedPoolAccount
			account, err = h.bizService.OnboardSharedPoolNativeAccount(c.Request.Context(), input)
			if err == nil {
				itemResult.Created = true
				itemResult.Account = newSharedPoolNativeAccountResponse(account)
				result.Created++
			}
		}
		if err != nil {
			if errors.Is(err, service.ErrPoolForbidden) || errors.Is(err, service.ErrNativeOperationConflict) {
				return nil, err
			}
			_, status := infraerrors.ToHTTP(err)
			itemResult.Error = status.Reason
			if itemResult.Error == "" {
				itemResult.Error = "native_onboarding_failed"
			}
			result.Failed++
		}
		result.Items = append(result.Items, itemResult)
	}
	return result, nil
}

func (h *BizDecipherHandler) repairSharedPoolNativeAccount(
	c *gin.Context,
	poolID int64,
	accountID int64,
	ownerID int64,
	req sharedPoolAccountRequest,
	fields map[string]json.RawMessage,
) (*service.SharedPoolAccount, error) {
	if err := requireNativeOwnerFields(fields, nativeRepairFields); err != nil {
		return nil, err
	}
	operationID, err := reconcileNativeOperationID(c, req.RepairOperationID)
	if err != nil {
		return nil, err
	}
	input, err := service.NewSharedPoolNativeRepairInput(service.SharedPoolNativeRepairRequest{
		PoolID: poolID, OwnerID: ownerID, SharedAccountID: accountID,
		RepairOperationID: operationID, ExpectedConfigVersion: req.ExpectedConfigVersion,
		Provider: req.Provider, AuthType: req.AuthType, UpstreamBaseURL: req.UpstreamBaseURL,
		APIKey: req.UpstreamAPIKey, Credentials: req.Credentials,
	})
	if err != nil {
		return nil, err
	}
	return h.bizService.RepairSharedPoolNativeAccount(c.Request.Context(), input)
}

func newSharedPoolNativeAccountResponse(account *service.SharedPoolAccount) *sharedPoolNativeAccountResponse {
	if account == nil {
		return nil
	}
	return &sharedPoolNativeAccountResponse{
		ID: account.ID, NativeOperationID: account.NativeOperationID,
		NativeBindingState: account.NativeBindingState, NativeBindingStep: account.NativeBindingStep,
		NativeErrorCode: account.NativeErrorCode, NativeErrorMessage: account.NativeErrorMessage,
		NativeModels: account.NativeModels, NativeModelsVerifiedAt: account.NativeModelsVerifiedAt,
		NativeConnectionStatus:     account.NativeConnectionStatus,
		NativeConnectionVerifiedAt: account.NativeConnectionVerifiedAt,
		NativeEvidenceStale:        account.NativeEvidenceStale,
		BillingActivationRequired:  account.BillingActivationRequired,
	}
}

func redactSharedPoolNativeAccounts(accounts []service.SharedPoolAccount) []any {
	result := make([]any, 0, len(accounts))
	for index := range accounts {
		account := &accounts[index]
		if account.NativeBindingState != "" || account.NativeOperationID != "" {
			result = append(result, newSharedPoolNativeAccountResponse(account))
			continue
		}
		result = append(result, account)
	}
	return result
}

func writeSharedPoolNativeError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrPoolForbidden) || errors.Is(err, sql.ErrNoRows) {
		response.ErrorWithDetails(c, http.StatusForbidden, "Shared pool account is not accessible", "native_account_forbidden", nil)
		return
	}
	response.ErrorFrom(c, err)
}

type nativeReadinessRequest struct {
	ExpectedConfigVersion int64 `json:"expected_config_version"`
}

type nativeActivationRequest struct {
	OperationID           string `json:"operation_id"`
	ExpectedConfigVersion int64  `json:"expected_config_version"`
}

func (h *BizDecipherHandler) ActivateSharedPoolNativeBilling(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	var req nativeActivationRequest
	fields, err := bindSharedPoolOwnerJSON(c, &req)
	if err != nil {
		response.ErrorFrom(c, service.ErrOwnerNativeFieldRejected)
		return
	}
	if err := requireNativeOwnerFields(fields, map[string]struct{}{"operation_id": {}, "expected_config_version": {}}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	operationID, err := reconcileNativeOperationID(c, req.OperationID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	input, err := service.NewSharedPoolNativeActivationInput(poolID, subject.UserID, operationID, req.ExpectedConfigVersion)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.bizService.ActivateSharedPoolNativeBilling(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *BizDecipherHandler) VerifySharedPoolNativeReadiness(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.ErrorFrom(c, service.ErrOwnerNativeFieldRejected)
		return
	}
	accountID, err := strconv.ParseInt(c.Param("accountId"), 10, 64)
	if err != nil || accountID <= 0 {
		response.ErrorFrom(c, service.ErrOwnerNativeFieldRejected)
		return
	}
	var req nativeReadinessRequest
	fields, err := bindSharedPoolOwnerJSON(c, &req)
	if err != nil {
		response.ErrorFrom(c, service.ErrOwnerNativeFieldRejected)
		return
	}
	if err := requireNativeOwnerFields(fields, map[string]struct{}{"expected_config_version": {}}); err != nil || req.ExpectedConfigVersion <= 0 {
		response.ErrorFrom(c, service.ErrOwnerNativeFieldRejected)
		return
	}
	account, err := h.bizService.VerifySharedPoolNativeReadiness(c.Request.Context(), service.SharedPoolNativeReadinessInput{
		PoolID: poolID, OwnerID: subject.UserID, SharedAccountID: accountID, ExpectedConfigVersion: req.ExpectedConfigVersion,
	})
	if err != nil {
		writeSharedPoolNativeError(c, err)
		return
	}
	response.Success(c, newSharedPoolNativeAccountResponse(account))
}
