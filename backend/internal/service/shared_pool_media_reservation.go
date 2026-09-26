package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

type PreparedSharedPoolMediaRequest struct {
	RequestID   string
	HoldAmount  float64
	Reservation *SharedPoolUsageReservation
}

// PrepareSharedPoolMediaRequest reserves a server-computed upper bound for a
// media request. requestedUnits is the parsed image count for image modes and
// the parsed duration in seconds for video modes. Callers must validate and
// clamp protocol fields before invoking this method.
func (s *OpenAIGatewayService) PrepareSharedPoolMediaRequest(
	ctx context.Context,
	accessKey *SharedPoolAccessKey,
	endpointType string,
	body []byte,
	requestedUnits int,
	expiresAt time.Time,
) (*PreparedSharedPoolMediaRequest, error) {
	if s == nil || s.bizDecipherService == nil || accessKey == nil {
		return nil, errors.New("shared pool billing is unavailable")
	}
	quote := accessKey.PriceQuote
	if err := ValidateSharedPoolPriceQuote(quote); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSharedPoolPricingNotConfigured, err)
	}
	endpointType = normalizeSharedPoolEndpoint(endpointType)
	if endpointType == "" || requestedUnits <= 0 {
		return nil, ErrSharedPoolUnsafeCostEstimate
	}
	if err := validateSharedPoolBillingModeForEndpoint(endpointType, quote.BasePrice.BillingMode); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSharedPoolPricingNotConfigured, err)
	}
	holdAmount, err := calculateSharedPoolMediaCharge(quote, endpointType, requestedUnits)
	if err != nil {
		return nil, err
	}
	requestID := sharedPoolReservationRequestID(sharedPoolReservationAttemptBase(ctx, resolveUsageBillingRequestID(ctx, "")), accessKey.ID)
	priceSnapshot := MarshalSharedPoolPriceSnapshot(quote)
	fingerprint := sharedPoolReservationFingerprint(requestID, accessKey, endpointType, body, priceSnapshot, holdAmount)
	reservation, err := s.bizDecipherService.ReserveSharedPoolUsage(ctx, SharedPoolUsageReservationInput{
		RequestID: requestID, RequestFingerprint: fingerprint, AccessKeyID: accessKey.ID,
		PoolID: accessKey.PoolID, AccountID: accessKey.AccountID, UserID: accessKey.UserID,
		PriceVersionID: quote.PriceVersionID, EndpointType: endpointType,
		Model: sharedPoolPublishedModel(accessKey), PricingSource: quote.PricingSource,
		PriceSnapshot: priceSnapshot, HoldAmount: holdAmount, ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, err
	}
	return &PreparedSharedPoolMediaRequest{RequestID: requestID, HoldAmount: holdAmount, Reservation: reservation}, nil
}

func calculateSharedPoolMediaCharge(quote *SharedPoolPriceQuote, endpointType string, units int) (float64, error) {
	if err := ValidateSharedPoolPriceQuote(quote); err != nil || units <= 0 {
		if err != nil {
			return 0, err
		}
		return 0, ErrSharedPoolUnsafeCostEstimate
	}
	var cost float64
	switch quote.BasePrice.BillingMode {
	case "per_request":
		cost = derefSharedPoolPrice(quote.BasePrice.PerRequestPrice) * quote.Multiplier
	case "image":
		if endpointType != SharedPoolEndpointImageGeneration && endpointType != SharedPoolEndpointImageEdit {
			return 0, ErrSharedPoolPricingUnavailable
		}
		cost = derefSharedPoolPrice(quote.BasePrice.ImageItemPrice) * float64(units) * quote.Multiplier
	case "video":
		if endpointType != SharedPoolEndpointVideo {
			return 0, ErrSharedPoolPricingUnavailable
		}
		cost = derefSharedPoolPrice(quote.BasePrice.VideoSecondPrice) * float64(units) * quote.Multiplier
	default:
		return 0, ErrSharedPoolPricingUnavailable
	}
	cost = applySharedPoolChargeBounds(cost, quote.BasePrice.MinimumCharge, quote.BasePrice.MaximumCharge)
	if cost < 0 || math.IsNaN(cost) || math.IsInf(cost, 0) {
		return 0, ErrSharedPoolUnsafeCostEstimate
	}
	return sharedPoolQuoteBuyerCharge(quote, cost), nil
}
