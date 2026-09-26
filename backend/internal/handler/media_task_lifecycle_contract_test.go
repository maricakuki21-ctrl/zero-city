package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/platform/mediatask"
	"github.com/stretchr/testify/require"
)

func TestMediaTaskCreateContextBindsBusinessEventAndClientKey(t *testing.T) {
	createContext, err := mediatask.NewCreateContext("reservation-request-1", "client-key-1", []byte(`{"prompt":"cat"}`))
	require.NoError(t, err)
	require.Equal(t, "reservation-request-1", createContext.BusinessEventID())
	require.Equal(t, "client-key-1", createContext.IdempotencyKey())
	require.NotEmpty(t, createContext.RequestHash())
}
