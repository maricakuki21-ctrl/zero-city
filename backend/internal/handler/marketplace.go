package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type MarketplaceApplication interface {
	ListListings(context.Context, service.MarketplaceListingQuery) (service.MarketplaceListingPage, error)
	GetListing(context.Context, int64, int64) (*service.MarketplaceListing, error)
	CreateListing(context.Context, int64, service.MarketplaceListingInput) (*service.MarketplaceListing, error)
	UpdateListing(context.Context, int64, int64, service.MarketplaceListingInput) (*service.MarketplaceListing, error)
	ArchiveListing(context.Context, int64, int64) (*service.MarketplaceListing, error)
	ListOwnerListings(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceListingPage, error)
	CreateInquiry(context.Context, int64, int64) (*service.MarketplaceInquiry, error)
	ListInquiries(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceInquiryPage, error)
	ListMessages(context.Context, int64, int64, service.MarketplacePageQuery) (service.MarketplaceMessagePage, error)
	PostMessage(context.Context, int64, int64, service.MarketplaceMessageInput) (*service.MarketplaceMessage, error)
	QuoteOrder(context.Context, int64, int64, service.MarketplaceOrderQuoteInput) (*service.MarketplaceOrder, error)
	ListOrders(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceOrderPage, error)
	GetOrder(context.Context, int64, int64) (*service.MarketplaceOrder, error)
	ApplyOrderAction(context.Context, int64, int64, service.MarketplaceOrderActionInput) (*service.MarketplaceOrder, error)
	AdminSetListingStatus(context.Context, int64, string) (*service.MarketplaceListing, error)
	ListDisputes(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceOrderPage, error)
	GetDispute(context.Context, int64, int64) (*service.MarketplaceOrder, error)
	ResolveDispute(context.Context, int64, int64, service.MarketplaceDisputeResolutionInput) (*service.MarketplaceOrder, error)
}

type MarketplaceHandler struct{ app MarketplaceApplication }

func NewMarketplaceHandler(app *service.MarketplaceService) *MarketplaceHandler {
	return &MarketplaceHandler{app: app}
}

func (h *MarketplaceHandler) ListListings(c *gin.Context) {
	page, err := h.app.ListListings(c.Request.Context(), service.MarketplaceListingQuery{
		MarketplacePageQuery: marketplacePageQuery(c),
		Kind:                 c.Query("kind"), Category: c.Query("category"), Tag: c.Query("tag"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

func (h *MarketplaceHandler) GetListing(c *gin.Context) {
	listingID, ok := marketplaceID(c)
	if !ok {
		return
	}
	viewerID := int64(0)
	if subject, authenticated := middleware.GetAuthSubjectFromContext(c); authenticated {
		viewerID = subject.UserID
	}
	listing, err := h.app.GetListing(c.Request.Context(), listingID, viewerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, listing)
}

func (h *MarketplaceHandler) CreateListing(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	var input service.MarketplaceListingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid marketplace listing request")
		return
	}
	listing, err := h.app.CreateListing(c.Request.Context(), subject.UserID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, listing)
}

func (h *MarketplaceHandler) UpdateListing(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	listingID, ok := marketplaceID(c)
	if !ok {
		return
	}
	var input service.MarketplaceListingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid marketplace listing request")
		return
	}
	listing, err := h.app.UpdateListing(c.Request.Context(), listingID, subject.UserID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, listing)
}

func (h *MarketplaceHandler) ArchiveListing(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	listingID, ok := marketplaceID(c)
	if !ok {
		return
	}
	listing, err := h.app.ArchiveListing(c.Request.Context(), listingID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, listing)
}

func (h *MarketplaceHandler) ListOwnerListings(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	page, err := h.app.ListOwnerListings(c.Request.Context(), subject.UserID, marketplacePageQuery(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

func (h *MarketplaceHandler) CreateInquiry(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	listingID, ok := marketplaceID(c)
	if !ok {
		return
	}
	inquiry, err := h.app.CreateInquiry(c.Request.Context(), listingID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, inquiry)
}

func (h *MarketplaceHandler) ListInquiries(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	page, err := h.app.ListInquiries(c.Request.Context(), subject.UserID, marketplacePageQuery(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

func (h *MarketplaceHandler) ListMessages(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	inquiryID, ok := marketplaceID(c)
	if !ok {
		return
	}
	page, err := h.app.ListMessages(c.Request.Context(), inquiryID, subject.UserID, marketplacePageQuery(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

func (h *MarketplaceHandler) PostMessage(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	inquiryID, ok := marketplaceID(c)
	if !ok {
		return
	}
	var input service.MarketplaceMessageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid marketplace message request")
		return
	}
	message, err := h.app.PostMessage(c.Request.Context(), inquiryID, subject.UserID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, message)
}

func (h *MarketplaceHandler) QuoteOrder(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	inquiryID, ok := marketplaceID(c)
	if !ok {
		return
	}
	var input service.MarketplaceOrderQuoteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid marketplace order quote request")
		return
	}
	order, err := h.app.QuoteOrder(c.Request.Context(), inquiryID, subject.UserID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, order)
}

func (h *MarketplaceHandler) ListOrders(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	page, err := h.app.ListOrders(c.Request.Context(), subject.UserID, marketplacePageQuery(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

func (h *MarketplaceHandler) GetOrder(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	orderID, ok := marketplaceID(c)
	if !ok {
		return
	}
	order, err := h.app.GetOrder(c.Request.Context(), orderID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, order)
}

func (h *MarketplaceHandler) ApplyOrderAction(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	orderID, ok := marketplaceID(c)
	if !ok {
		return
	}
	var input service.MarketplaceOrderActionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid marketplace order action request")
		return
	}
	order, err := h.app.ApplyOrderAction(c.Request.Context(), orderID, subject.UserID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, order)
}

func (h *MarketplaceHandler) AdminSetListingStatus(c *gin.Context) {
	listingID, ok := marketplaceID(c)
	if !ok {
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid marketplace status request")
		return
	}
	listing, err := h.app.AdminSetListingStatus(c.Request.Context(), listingID, input.Status)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, listing)
}

func marketplaceSubject(c *gin.Context) (middleware.AuthSubject, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
	}
	return subject, ok
}

func marketplaceID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid marketplace id")
		return 0, false
	}
	return id, true
}

func marketplacePageQuery(c *gin.Context) service.MarketplacePageQuery {
	cursor, _ := strconv.ParseInt(strings.TrimSpace(c.Query("cursor")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	return service.MarketplacePageQuery{Cursor: cursor, Limit: limit}
}
