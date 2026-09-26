package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
)

func (n *NativeWorkbench) models(ctx context.Context, secret string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	res, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, ErrWorkbenchCatalogUnavailable
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&result); err != nil {
		return nil, err
	}
	models := []string{}
	seen := map[string]bool{}
	for _, item := range result.Data {
		model := strings.TrimSpace(item.ID)
		if model != "" && len(model) <= 160 && !seen[model] {
			seen[model] = true
			models = append(models, model)
		}
	}
	return models, nil
}

func (n *NativeWorkbench) Execute(ctx context.Context, request WorkbenchCanonicalRequest) (WorkbenchCanonicalResult, error) {
	if err := validateWorkbenchCanonicalRequest(request); err != nil {
		return WorkbenchCanonicalResult{}, err
	}
	if request.Resolved.Capability.CanonicalModelVersion != WorkbenchNativeVersion || len(request.Intent) > 128*1024 {
		return WorkbenchCanonicalResult{}, workbench.ErrInvalidCommand
	}
	// Revalidate the signed selection at dispatch, never trust an API key
	// supplied by a caller of the executor.
	record, err := n.ResolveWorkbenchCatalog(ctx, workbench.Identity{ActorID: workbench.ActorID(request.Resolved.User.ID)},
		request.Resolved.Capability.ID, request.Resolved.Quote.ID)
	if err != nil {
		return WorkbenchCanonicalResult{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	n.mu.Lock()
	if n.active == nil {
		n.active = make(map[workbench.RunID]context.CancelFunc)
	}
	if _, exists := n.active[request.RunID]; exists {
		n.mu.Unlock()
		cancel()
		return WorkbenchCanonicalResult{}, workbench.ErrIdempotencyConflict
	}
	n.active[request.RunID] = cancel
	n.mu.Unlock()
	defer func() {
		cancel()
		n.mu.Lock()
		delete(n.active, request.RunID)
		n.mu.Unlock()
	}()
	// /v1/messages routes all supported platform groups through their ordinary
	// authentication, membership, quota, concurrency and billing middleware.
	body, err := json.Marshal(map[string]any{
		"model":      record.Capability.CanonicalModelID,
		"max_tokens": 4096, "stream": false,
		"messages": []map[string]string{{"role": "user", "content": request.Intent}},
	})
	if err != nil {
		return WorkbenchCanonicalResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return WorkbenchCanonicalResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+record.APIKey.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Idempotency-Key", string(request.RunID))
	req.Header.Set("X-Client-Request-ID", string(request.RunID))
	res, err := n.client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return WorkbenchCanonicalResult{}, context.Canceled
		}
		return WorkbenchCanonicalResult{}, fmt.Errorf("网关连接未确认，已产生的用量以账单为准；请勿自动重复发起")
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
	if err != nil {
		return WorkbenchCanonicalResult{}, err
	}
	if len(raw) > 4<<20 {
		return WorkbenchCanonicalResult{}, fmt.Errorf("模型结果超过4MB限制，请查看用量账单")
	}
	if res.StatusCode != http.StatusOK {
		// Do not expose provider details, credentials or raw upstream prompts.
		return WorkbenchCanonicalResult{}, fmt.Errorf("模型网关拒绝本次请求（HTTP %d），请检查资源权限、支持的模型和余额", res.StatusCode)
	}
	var payload struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return WorkbenchCanonicalResult{}, fmt.Errorf("模型未返回有效文本响应，用量以网关账单为准")
	}
	var parts []string
	for _, block := range payload.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	if len(parts) == 0 {
		return WorkbenchCanonicalResult{}, fmt.Errorf("本次模型没有文本结果，用量以网关账单为准")
	}
	requestID := res.Header.Get("request-id")
	if requestID == "" {
		requestID = res.Header.Get("x-request-id")
	}
	if requestID == "" {
		requestID = payload.ID
	}
	return WorkbenchCanonicalResult{
		CanonicalRequestID: requestID, Body: []byte(strings.Join(parts, "\n\n")),
		ContentType: "text/plain; charset=utf-8",
		// Native gateway accounting owns the charge. Do not mint a second ledger
		// journal or label a guessed amount as a finalized cost.
	}, nil
}

func (n *NativeWorkbench) Cancel(ctx context.Context, runID workbench.RunID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	n.mu.Lock()
	cancel := n.active[runID]
	n.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}
