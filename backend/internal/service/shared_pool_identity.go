package service

import "errors"

type CanonicalGroupID int64

type CanonicalAccountID int64

type SharedPoolSupplySourceKind string

const (
	SharedPoolSupplyPoolDefault SharedPoolSupplySourceKind = "pool_default"
	SharedPoolSupplyPoolAccount SharedPoolSupplySourceKind = "pool_account"
)

type SharedPoolSupplyReference struct {
	Kind SharedPoolSupplySourceKind
	ID   int64
}

type SharedPoolIdentityQuery struct {
	PoolID  int64
	OwnerID int64
	Supply  *SharedPoolSupplyReference
}

type SharedPoolCanonicalIdentity struct {
	PoolID    int64
	OwnerID   int64
	GroupID   CanonicalGroupID
	AccountID *CanonicalAccountID
	Lifecycle string
}

var (
	ErrSharedPoolIdentityQueryInvalid = errors.New("shared pool identity query is invalid")
	ErrSharedPoolIdentityUnmapped     = errors.New("shared pool has no canonical Sub2 identity binding")
	ErrSharedPoolIdentityQuarantined  = errors.New("shared pool canonical Sub2 identity is quarantined")
)
