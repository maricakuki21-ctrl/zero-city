package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func normalizeSharedPoolPlatformFeePercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) ||
		value < SharedPoolPlatformFeePercentMin || value > SharedPoolPlatformFeePercentMax {
		return SharedPoolPlatformFeePercentDefault
	}
	return value
}

func parseSharedPoolPlatformFeePercent(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return SharedPoolPlatformFeePercentDefault
	}
	return normalizeSharedPoolPlatformFeePercent(value)
}

func (s *BizDecipherService) loadSharedPoolDefaultPlatformFeePercent(ctx context.Context) (float64, error) {
	if s == nil || s.settingRepo == nil {
		return SharedPoolPlatformFeePercentDefault, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeySharedPoolDefaultPlatformFeePercent)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return SharedPoolPlatformFeePercentDefault, nil
		}
		return 0, fmt.Errorf("load shared pool default platform fee percent: %w", err)
	}
	return parseSharedPoolPlatformFeePercent(raw), nil
}
