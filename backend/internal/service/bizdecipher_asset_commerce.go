package service

import (
	"context"
	"encoding/json"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrAssetCommerceInvalid   = infraerrors.BadRequest("ASSET_COMMERCE_INVALID", "Invalid asset purchase request")
	ErrAssetCommerceForbidden = infraerrors.Forbidden("ASSET_COMMERCE_FORBIDDEN", "Asset purchase or delivery is not permitted")
	ErrAssetCommerceClosed    = infraerrors.Forbidden("ASSET_COMMERCE_CLOSED", "Asset purchases are not enabled")
	ErrAssetCommerceConflict  = infraerrors.Conflict("ASSET_COMMERCE_CONFLICT", "Asset price, purchase state, or operation payload changed")
	ErrAssetCommerceFunds     = infraerrors.Conflict("ASSET_COMMERCE_FUNDS", "Insufficient available balance")
)

type AssetCommerceInfo struct {
	AssetID     int64  `json:"asset_id"`
	OwnerUserID int64  `json:"owner_user_id"`
	PricingType string `json:"pricing_type"`
	Price       string `json:"price"`
	CanDownload bool   `json:"can_download"`
	PurchaseID  int64  `json:"purchase_id"`
	HasPackage  bool   `json:"has_package"`
	Status      string `json:"status"`
}
type AssetPurchase struct {
	ID            int64      `json:"id"`
	AssetID       int64      `json:"asset_id"`
	BuyerUserID   int64      `json:"buyer_user_id"`
	OwnerUserID   int64      `json:"owner_user_id"`
	OperationID   string     `json:"operation_id"`
	AssetTitle    string     `json:"asset_title"`
	Amount        string     `json:"amount"`
	PlatformFee   string     `json:"platform_fee"`
	CreatorAmount string     `json:"creator_amount"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	RefundedAt    *time.Time `json:"refunded_at,omitempty"`
	RefundReason  string     `json:"refund_reason"`
}
type assetCommerceRepository interface {
	GetAssetCommercePolicy(context.Context) (*ColumnCommercePolicy, error)
	SetAssetCommercePolicy(context.Context, int64, bool, string) (*ColumnCommercePolicy, error)
	GetAssetCommerce(context.Context, int64, int64, bool) (*AssetCommerceInfo, error)
	SetAssetPricing(context.Context, int64, int64, ColumnPricingInput) (*AssetCommerceInfo, error)
	PurchaseAsset(context.Context, int64, int64, ColumnPurchaseInput) (*AssetPurchase, error)
	RefundAssetPurchase(context.Context, int64, int64, int64, ColumnRefundInput) (*AssetPurchase, error)
	ListAssetPurchases(context.Context, int64, CreatorColumnQuery) ([]AssetPurchase, error)
}

func (s *BizDecipherService) AssetCommerceRepo() (assetCommerceRepository, error) {
	if s != nil && s.repo != nil {
		if r, ok := s.repo.(assetCommerceRepository); ok {
			return r, nil
		}
	}
	return nil, ErrCreatorColumnUnavailable
}
func (s *BizDecipherService) PurchaseAsset(ctx context.Context, id, buyer int64, input ColumnPurchaseInput) (*AssetPurchase, error) {
	r, err := s.AssetCommerceRepo()
	if err != nil {
		return nil, err
	}
	if err = s.columnCommerceInvalidateBalance(ctx, buyer); err != nil {
		return nil, err
	}
	p, err := r.PurchaseAsset(ctx, id, buyer, input)
	if err != nil {
		return nil, err
	}
	if err = s.columnCommerceInvalidateBalance(ctx, buyer); err != nil {
		return nil, err
	}
	return p, nil
}
func (s *BizDecipherService) RefundAssetPurchase(ctx context.Context, id, purchaseID, actor int64, input ColumnRefundInput) (*AssetPurchase, error) {
	r, err := s.AssetCommerceRepo()
	if err != nil {
		return nil, err
	}
	items, err := r.ListAssetPurchases(ctx, id, CreatorColumnQuery{ViewerID: actor, Admin: true, Cursor: purchaseID + 1, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(items) != 1 || items[0].ID != purchaseID {
		return nil, ErrCapabilityAssetPackageNotFound
	}
	buyer := items[0].BuyerUserID
	if err = s.columnCommerceInvalidateBalance(ctx, buyer); err != nil {
		return nil, err
	}
	p, err := r.RefundAssetPurchase(ctx, id, purchaseID, actor, input)
	if err != nil {
		return nil, err
	}
	if err = s.columnCommerceInvalidateBalance(ctx, buyer); err != nil {
		return nil, err
	}
	return p, nil
}
func (s *BizDecipherService) assetPackageEntitled(ctx context.Context, asset *CapabilityAsset, viewer int64) (bool, error) {
	if asset == nil {
		return false, ErrCapabilityAssetPackageNotFound
	}
	if asset.UserID == viewer {
		return true, nil
	}
	if capabilityAssetHasPublicPackageDelivery(asset.PricingType) {
		return true, nil
	}
	if asset.PricingType != "paid" {
		return false, nil
	}
	r, err := s.AssetCommerceRepo()
	if err != nil {
		return false, nil
	}
	info, err := r.GetAssetCommerce(ctx, asset.ID, viewer, false)
	if err != nil {
		return false, err
	}
	return info.CanDownload, nil
}
func hidePaidAssetManifest(versions []CapabilityAssetVersion) {
	for i := range versions {
		versions[i].Manifest = json.RawMessage(`{}`)
	}
}
func hidePaidAssetLinks(asset *CapabilityAsset) {
	if asset != nil && asset.PricingType == "paid" {
		asset.SourceURL = ""
		asset.TemplateURL = ""
		asset.DemoURL = ""
	}
}
func (s *BizDecipherService) ReuseCapabilityAssetPackage(ctx context.Context, id, viewer int64, version, operation string) (*CapabilityAssetPackage, error) {
	if !columnOperationPattern.MatchString(operation) {
		return nil, ErrAssetCommerceInvalid
	}
	pkg, err := s.DownloadCapabilityAssetPackage(ctx, id, viewer, version)
	if err != nil {
		return nil, err
	}
	if err = s.RecordCapabilityAssetUse(ctx, id, viewer, "derive", "asset_package", operation); err != nil {
		return nil, err
	}
	return pkg, nil
}
