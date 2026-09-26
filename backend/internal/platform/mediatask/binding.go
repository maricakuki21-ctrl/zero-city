package mediatask

import "strings"

type BindingState string

const (
	BindingPending  BindingState = "pending"
	BindingAccepted BindingState = "accepted"
)

type Binding struct {
	Context        CreateContext
	APIKeyID       int64
	UserID         int64
	GroupID        int64
	AccountID      int64
	Endpoint       string
	State          BindingState
	UpstreamTaskID string
}

func (b Binding) Accepted() bool {
	return b.State == BindingAccepted && strings.TrimSpace(b.UpstreamTaskID) != ""
}

type ClaimBindingInput struct {
	Context   CreateContext
	APIKeyID  int64
	UserID    int64
	GroupID   int64
	AccountID int64
	Endpoint  string
}
