package handler

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PayhipHandler struct {
	payhipService *service.PayhipService
}

func NewPayhipHandler(payhipService *service.PayhipService) *PayhipHandler {
	return &PayhipHandler{payhipService: payhipService}
}

type payhipProductInfoResponse struct {
	ProductKey   string  `json:"product_key"`
	PurchaseURL  string  `json:"purchase_url"`
	CreditAmount float64 `json:"credit_amount"`
	CreditCents  int64   `json:"credit_cents"`
	Currency     string  `json:"currency"`
	Title        string  `json:"title,omitempty"`
}

type payhipInfoResponse struct {
	Enabled      bool                        `json:"enabled"`
	PurchaseURL  string                      `json:"purchase_url"`
	ProductKey   string                      `json:"product_key"`
	CreditAmount float64                     `json:"credit_amount"`
	CreditCents  int64                       `json:"credit_cents"`
	Currency     string                      `json:"currency"`
	Products     []payhipProductInfoResponse `json:"products"`
}

func (h *PayhipHandler) Info(c *gin.Context) {
	cfg := h.payhipService.Config()
	products := make([]payhipProductInfoResponse, 0, len(cfg.Products))
	for _, product := range cfg.Products {
		products = append(products, payhipProductInfoResponse{
			ProductKey:   product.ProductKey,
			PurchaseURL:  product.PurchaseURL,
			CreditAmount: product.CreditAmount,
			CreditCents:  product.CreditCents,
			Currency:     product.Currency,
			Title:        product.Title,
		})
	}
	response.Success(c, payhipInfoResponse{
		Enabled:      cfg.Enabled(),
		PurchaseURL:  cfg.PurchaseURL,
		ProductKey:   cfg.ProductKey,
		CreditAmount: cfg.CreditAmount,
		CreditCents:  cfg.CreditCents,
		Currency:     cfg.Currency,
		Products:     products,
	})
}

func (h *PayhipHandler) Webhook(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBodySize))
	if err != nil {
		c.String(http.StatusBadRequest, "failed to read body")
		return
	}
	order, err := h.payhipService.HandleWebhook(c.Request.Context(), body)
	if err != nil {
		slog.Warn("Payhip webhook rejected", "error", err.Error(), "body_len", len(body))
		response.ErrorFrom(c, err)
		return
	}
	if order == nil {
		c.String(http.StatusOK, "success")
		return
	}
	c.String(http.StatusOK, "success")
}

type PayhipClaimRequest struct {
	TransactionID string `json:"transaction_id" binding:"required"`
	Email         string `json:"email" binding:"required,email"`
}

type PayhipClaimResponse struct {
	TransactionID  string          `json:"transaction_id"`
	Status         string          `json:"status"`
	CreditAmount   float64         `json:"credit_amount"`
	RedeemCode     *dto.RedeemCode `json:"redeem_code,omitempty"`
	AlreadyClaimed bool            `json:"already_claimed"`
}

func (h *PayhipHandler) Claim(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req PayhipClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	order, redeemed, err := h.payhipService.Claim(c.Request.Context(), subject.UserID, req.TransactionID, req.Email)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := PayhipClaimResponse{
		TransactionID:  order.TransactionID,
		Status:         order.Status,
		CreditAmount:   order.CreditAmount,
		AlreadyClaimed: redeemed == nil,
	}
	if redeemed != nil {
		out.RedeemCode = dto.RedeemCodeFromService(redeemed)
	}
	if strings.EqualFold(order.Status, service.PayhipOrderStatusRefunded) {
		out.AlreadyClaimed = false
	}
	response.Success(c, out)
}
