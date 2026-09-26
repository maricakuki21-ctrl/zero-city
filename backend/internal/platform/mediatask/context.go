package mediatask

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalidCreateContext = errors.New("media create context is invalid")

type CreateContext struct {
	businessEventID string
	idempotencyKey  string
	requestHash     string
}

func NewCreateContext(businessEventID, idempotencyKey string, requestBody []byte) (CreateContext, error) {
	businessEventID = strings.TrimSpace(businessEventID)
	if businessEventID == "" {
		return CreateContext{}, ErrInvalidCreateContext
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = businessEventID
	}
	hash := sha256.Sum256(requestBody)
	return CreateContext{
		businessEventID: businessEventID,
		idempotencyKey:  idempotencyKey,
		requestHash:     hex.EncodeToString(hash[:]),
	}, nil
}

func RestoreCreateContext(businessEventID, idempotencyKey, requestHash string) (CreateContext, error) {
	businessEventID = strings.TrimSpace(businessEventID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	requestHash = strings.ToLower(strings.TrimSpace(requestHash))
	if businessEventID == "" || idempotencyKey == "" || len(requestHash) != sha256.Size*2 {
		return CreateContext{}, ErrInvalidCreateContext
	}
	if _, err := hex.DecodeString(requestHash); err != nil {
		return CreateContext{}, ErrInvalidCreateContext
	}
	return CreateContext{businessEventID: businessEventID, idempotencyKey: idempotencyKey, requestHash: requestHash}, nil
}

func (c CreateContext) BusinessEventID() string { return c.businessEventID }
func (c CreateContext) IdempotencyKey() string  { return c.idempotencyKey }
func (c CreateContext) RequestHash() string     { return c.requestHash }

type createContextKey struct{}

func WithCreateContext(ctx context.Context, value CreateContext) context.Context {
	return context.WithValue(ctx, createContextKey{}, value)
}

func CreateContextFrom(ctx context.Context) (CreateContext, bool) {
	value, ok := ctx.Value(createContextKey{}).(CreateContext)
	return value, ok
}
