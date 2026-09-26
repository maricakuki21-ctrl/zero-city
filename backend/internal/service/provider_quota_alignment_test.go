//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/schema"
)

func TestProviderQuotaPlatformSchemaAndEnforcement(t *testing.T) {
	var validate func(string) error
	for _, field := range (schema.UserPlatformQuota{}).Fields() {
		if field.Descriptor().Name == "platform" {
			for _, validator := range field.Descriptor().Validators {
				if fn, ok := validator.(func(string) error); ok {
					previous := validate
					validate = func(value string) error {
						if previous != nil {
							if err := previous(value); err != nil {
								return err
							}
						}
						return fn(value)
					}
				}
			}
		}
	}
	if validate == nil {
		t.Fatal("platform validator is missing")
	}
	for _, platform := range []string{"kimi", "zhipu", "deepseek", "minimax", "opencode_go"} {
		t.Run(platform, func(t *testing.T) {
			if !IsAllowedQuotaPlatform(platform) {
				t.Fatal("platform rejected by service")
			}
			if err := validate(platform); err != nil {
				t.Fatal(err)
			}
			limit := 5.0
			cache := &fakeFullCache{entry: &UserPlatformQuotaCacheEntry{
				DailyUsageUSD: 5, DailyLimitUSD: &limit, DailyWindowStart: currentDayStart(),
				SchemaVersion: UserPlatformQuotaCacheSchemaV1,
			}}
			s := newServiceForPreflight(t, &fakeQuotaRepo{}, cache)
			err := s.checkUserPlatformQuotaEligibility(context.Background(), 1, platform)
			if !errors.Is(err, ErrUserPlatformDailyQuotaExhausted) {
				t.Fatalf("expected exhausted quota, got %v", err)
			}
			cache.entry.DailyUsageUSD = 4
			if err := s.checkUserPlatformQuotaEligibility(context.Background(), 1, platform); err != nil {
				t.Fatal(err)
			}
		})
	}
	if IsAllowedQuotaPlatform("unconfigured-provider") || validate("unconfigured-provider") == nil {
		t.Fatal("unknown platform accepted")
	}
}
