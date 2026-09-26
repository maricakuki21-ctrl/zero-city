package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const creditLotteryExpiryTimeout = 30 * time.Second

// CreditLotteryExpiryService settles expired lottery sessions in the background.
type CreditLotteryExpiryService struct {
	lottery  *CreditLotteryService
	interval time.Duration
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewCreditLotteryExpiryService(lottery *CreditLotteryService, interval time.Duration) *CreditLotteryExpiryService {
	return &CreditLotteryExpiryService{
		lottery:  lottery,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (s *CreditLotteryExpiryService) Start() {
	if s == nil || s.lottery == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *CreditLotteryExpiryService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *CreditLotteryExpiryService) runOnce() {
	if s == nil || s.lottery == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), creditLotteryExpiryTimeout)
	defer cancel()

	settled, err := s.lottery.SettleExpiredSessions(ctx, 100)
	if err != nil {
		slog.Warn("credit lottery expiry settlement failed", "error", err)
		return
	}
	if len(settled) > 0 {
		slog.Info("credit lottery expired sessions settled", "count", len(settled))
	}
}
