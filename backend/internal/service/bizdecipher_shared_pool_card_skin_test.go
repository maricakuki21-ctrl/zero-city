package service

import (
	"context"
	"strings"
	"testing"
)

type sharedPoolCardSkinRepoStub struct {
	BizDecipherRepository
	ownedPool      *SharedPool
	ownedRarity    string
	ownedReads     int
	publicReads    int
	writes         int
	writtenCardKey string
	writtenRarity  string
}

func (r *sharedPoolCardSkinRepoStub) GetOwnedSharedPool(context.Context, int64, int64) (*SharedPool, error) {
	r.ownedReads++
	return r.ownedPool, nil
}

func (r *sharedPoolCardSkinRepoStub) GetSharedPool(context.Context, int64) (*SharedPool, error) {
	r.publicReads++
	return nil, nil
}

func (r *sharedPoolCardSkinRepoStub) GetUserCollectibleCardRarity(context.Context, int64, string) (string, error) {
	return r.ownedRarity, nil
}

func (r *sharedPoolCardSkinRepoStub) SetPoolCardSkinTx(_ context.Context, _ int64, _ int64, cardKey, cardRarity string) error {
	r.writes++
	r.writtenCardKey = cardKey
	r.writtenRarity = cardRarity
	return nil
}

func TestSetPoolCardSkinSupportsOwnedDraftPool(t *testing.T) {
	ownerID := int64(42)
	repo := &sharedPoolCardSkinRepoStub{
		ownedPool:   &SharedPool{ID: 7, OwnerID: &ownerID, Listed: false},
		ownedRarity: "rare",
	}
	svc := NewBizDecipherService(repo, nil, nil)

	err := svc.SetPoolCardSkin(context.Background(), 7, ownerID, "card-dawn", "rare")
	if err != nil {
		t.Fatalf("SetPoolCardSkin returned error for owned draft: %v", err)
	}
	if repo.ownedReads != 1 || repo.publicReads != 0 {
		t.Fatalf("authorization reads owned=%d public=%d, want 1/0", repo.ownedReads, repo.publicReads)
	}
	if repo.writes != 1 || repo.writtenCardKey != "card-dawn" || repo.writtenRarity != "rare" {
		t.Fatalf("unexpected skin write: writes=%d key=%q rarity=%q", repo.writes, repo.writtenCardKey, repo.writtenRarity)
	}
}

func TestClearPoolCardSkinSupportsOwnedDraftPool(t *testing.T) {
	ownerID := int64(42)
	repo := &sharedPoolCardSkinRepoStub{
		ownedPool: &SharedPool{ID: 7, OwnerID: &ownerID, Listed: false},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	err := svc.ClearPoolCardSkin(context.Background(), 7, ownerID)
	if err != nil {
		t.Fatalf("ClearPoolCardSkin returned error for owned draft: %v", err)
	}
	if repo.ownedReads != 1 || repo.publicReads != 0 {
		t.Fatalf("authorization reads owned=%d public=%d, want 1/0", repo.ownedReads, repo.publicReads)
	}
	if repo.writes != 1 || repo.writtenCardKey != "" || repo.writtenRarity != "" {
		t.Fatalf("unexpected clear write: writes=%d key=%q rarity=%q", repo.writes, repo.writtenCardKey, repo.writtenRarity)
	}
}

func TestSetPoolCardSkinRejectsMissingOwnedPoolWithoutPanic(t *testing.T) {
	repo := &sharedPoolCardSkinRepoStub{ownedRarity: "rare"}
	svc := NewBizDecipherService(repo, nil, nil)

	err := svc.SetPoolCardSkin(context.Background(), 7, 42, "card-dawn", "rare")
	if err == nil || !strings.Contains(err.Error(), "not pool owner") {
		t.Fatalf("expected owner rejection, got %v", err)
	}
	if repo.writes != 0 {
		t.Fatalf("missing owned pool must not be written, got %d writes", repo.writes)
	}
}

func TestSetPoolCardSkinRejectsClientRarityMismatch(t *testing.T) {
	ownerID := int64(42)
	repo := &sharedPoolCardSkinRepoStub{
		ownedPool:   &SharedPool{ID: 7, OwnerID: &ownerID},
		ownedRarity: "legendary",
	}
	svc := NewBizDecipherService(repo, nil, nil)

	err := svc.SetPoolCardSkin(context.Background(), 7, ownerID, "card-dawn", "common")
	if err == nil || !strings.Contains(err.Error(), "rarity") {
		t.Fatalf("expected rarity mismatch, got %v", err)
	}
	if repo.writes != 0 {
		t.Fatalf("mismatched rarity must not write, got %d writes", repo.writes)
	}
}
