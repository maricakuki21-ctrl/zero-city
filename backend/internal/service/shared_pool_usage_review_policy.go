package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	SharedPoolReviewAutoReleaseMinutesDefault = 15
	SharedPoolReviewAutoReleaseMinutesMin     = 5
	SharedPoolReviewAutoReleaseMinutesMax     = 7 * 24 * 60
	SharedPoolReviewAutoReleaseMaxHoldDefault = 0.01
	SharedPoolReviewAutoReleaseMaxHoldMax     = 0.10
	SharedPoolUsageReviewBatchLimit           = 500
)

type SharedPoolUsageReviewPolicy struct {
	AutoReleaseEnabled bool    `json:"auto_release_enabled"`
	AutoReleaseMinutes int     `json:"auto_release_minutes"`
	AutoReleaseMaxHold float64 `json:"auto_release_max_hold"`
}

type UpdateSharedPoolUsageReviewPolicyInput struct {
	AutoReleaseEnabled bool
	AutoReleaseMinutes int
	AutoReleaseMaxHold float64
}

type bizDecipherSettingMultipleWriter interface {
	SetMultiple(ctx context.Context, settings map[string]string) error
}

func DefaultSharedPoolUsageReviewPolicy() SharedPoolUsageReviewPolicy {
	return SharedPoolUsageReviewPolicy{
		AutoReleaseEnabled: true,
		AutoReleaseMinutes: SharedPoolReviewAutoReleaseMinutesDefault,
		AutoReleaseMaxHold: SharedPoolReviewAutoReleaseMaxHoldDefault,
	}
}

func (p SharedPoolUsageReviewPolicy) Cutoff(now time.Time) time.Time {
	return now.Add(-time.Duration(p.AutoReleaseMinutes) * time.Minute)
}

func (s *BizDecipherService) GetSharedPoolUsageReviewPolicy(ctx context.Context) (*SharedPoolUsageReviewPolicy, error) {
	policy := DefaultSharedPoolUsageReviewPolicy()
	if s == nil || s.settingRepo == nil {
		return &policy, nil
	}

	if raw, found, err := s.loadOptionalBizSetting(ctx, SettingKeySharedPoolReviewAutoReleaseEnabled); err != nil {
		return nil, err
	} else if found {
		value, parseErr := strconv.ParseBool(strings.TrimSpace(raw))
		if parseErr != nil {
			return nil, fmt.Errorf("invalid shared pool review auto-release enabled setting: %w", parseErr)
		}
		policy.AutoReleaseEnabled = value
	}
	if raw, found, err := s.loadOptionalBizSetting(ctx, SettingKeySharedPoolReviewAutoReleaseMinutes); err != nil {
		return nil, err
	} else if found {
		value, parseErr := strconv.Atoi(strings.TrimSpace(raw))
		if parseErr != nil || value < SharedPoolReviewAutoReleaseMinutesMin || value > SharedPoolReviewAutoReleaseMinutesMax {
			return nil, errors.New("invalid shared pool review auto-release minutes setting")
		}
		policy.AutoReleaseMinutes = value
	}
	if raw, found, err := s.loadOptionalBizSetting(ctx, SettingKeySharedPoolReviewAutoReleaseMaxHold); err != nil {
		return nil, err
	} else if found {
		value, parseErr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if parseErr != nil || value < 0 || value > SharedPoolReviewAutoReleaseMaxHoldMax || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errors.New("invalid shared pool review auto-release maximum hold setting")
		}
		policy.AutoReleaseMaxHold = normalizedSharedPoolMoney(value)
	}
	return &policy, nil
}

func (s *BizDecipherService) AdminUpdateSharedPoolUsageReviewPolicy(
	ctx context.Context,
	input UpdateSharedPoolUsageReviewPolicyInput,
) (*SharedPoolUsageReviewPolicy, error) {
	if input.AutoReleaseMinutes < SharedPoolReviewAutoReleaseMinutesMin || input.AutoReleaseMinutes > SharedPoolReviewAutoReleaseMinutesMax {
		return nil, invalidSharedPoolUsageReview(fmt.Sprintf(
			"auto-release minutes must be between %d and %d",
			SharedPoolReviewAutoReleaseMinutesMin,
			SharedPoolReviewAutoReleaseMinutesMax,
		))
	}
	if input.AutoReleaseMaxHold < 0 || input.AutoReleaseMaxHold > SharedPoolReviewAutoReleaseMaxHoldMax ||
		math.IsNaN(input.AutoReleaseMaxHold) || math.IsInf(input.AutoReleaseMaxHold, 0) {
		return nil, invalidSharedPoolUsageReview("auto-release maximum hold is invalid")
	}
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("shared pool review policy setting repository is unavailable")
	}
	writer, ok := s.settingRepo.(bizDecipherSettingMultipleWriter)
	if !ok || writer == nil {
		return nil, errors.New("shared pool review policy setting writer is unavailable")
	}
	maxHold := normalizedSharedPoolMoney(input.AutoReleaseMaxHold)
	if err := writer.SetMultiple(ctx, map[string]string{
		SettingKeySharedPoolReviewAutoReleaseEnabled: strconv.FormatBool(input.AutoReleaseEnabled),
		SettingKeySharedPoolReviewAutoReleaseMinutes: strconv.Itoa(input.AutoReleaseMinutes),
		SettingKeySharedPoolReviewAutoReleaseMaxHold: strconv.FormatFloat(maxHold, 'f', 8, 64),
	}); err != nil {
		return nil, fmt.Errorf("update shared pool review policy: %w", err)
	}
	return &SharedPoolUsageReviewPolicy{
		AutoReleaseEnabled: input.AutoReleaseEnabled,
		AutoReleaseMinutes: input.AutoReleaseMinutes,
		AutoReleaseMaxHold: maxHold,
	}, nil
}

func (s *BizDecipherService) loadOptionalBizSetting(ctx context.Context, key string) (string, bool, error) {
	raw, err := s.settingRepo.GetValue(ctx, key)
	if errors.Is(err, ErrSettingNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("load setting %s: %w", key, err)
	}
	return raw, true, nil
}
