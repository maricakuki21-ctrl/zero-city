package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProvideServiceBuildInfo(t *testing.T) {
	in := handler.BuildInfo{
		Version:   "v-test",
		BuildType: "release",
	}
	out := provideServiceBuildInfo(in)
	require.Equal(t, in.Version, out.Version)
	require.Equal(t, in.BuildType, out.BuildType)
}

func TestBackgroundRuntimeLogMarker(t *testing.T) {
	active := config.ProvideBackgroundRuntime(&config.Config{BackgroundRuntimeRole: config.BackgroundRuntimeRoleActive})
	standby := config.ProvideBackgroundRuntime(&config.Config{BackgroundRuntimeRole: config.BackgroundRuntimeRoleStandby})
	require.Equal(t, "background-runtime role=active autonomous-global-workers=enabled", backgroundRuntimeLogMarker(active))
	require.Equal(t, "background-runtime role=standby autonomous-global-workers=disabled", backgroundRuntimeLogMarker(standby))
}

func TestGeneratedWireIncludesPoolSeatBillingService(t *testing.T) {
	content, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err)
	require.True(t, strings.Contains(string(content), "service.ProvidePoolSeatBillingService("), "wire_gen.go must instantiate PoolSeatBillingService")
}

func TestGeneratedWireInjectsBackgroundRuntime(t *testing.T) {
	content, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err)
	generated := string(content)
	require.Contains(t, generated, "backgroundRuntime := config.ProvideBackgroundRuntime(configConfig)")
	require.Contains(t, generated, "BackgroundRuntime: backgroundRuntime")
	require.Contains(t, generated, "service.ProvidePoolSeatBillingService(bizDecipherService, leaderLockCache, db, backgroundRuntime)")
}

func TestProvideCleanup_WithMinimalDependencies_NoPanic(t *testing.T) {
	cfg := &config.Config{}

	oauthSvc := service.NewOAuthService(nil, nil)
	openAIOAuthSvc := service.NewOpenAIOAuthService(nil, nil)
	geminiOAuthSvc := service.NewGeminiOAuthService(nil, nil, nil, nil, cfg)
	antigravityOAuthSvc := service.NewAntigravityOAuthService(nil)

	tokenRefreshSvc := service.NewTokenRefreshService(
		nil,
		oauthSvc,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		nil,
		nil,
		cfg,
		nil,
	)
	accountExpirySvc := service.NewAccountExpiryService(nil, time.Second)
	proxyExpirySvc := service.NewProxyExpiryService(nil, time.Second)
	subscriptionExpirySvc := service.NewSubscriptionExpiryService(nil, time.Second)
	pricingSvc := service.NewPricingService(cfg, nil)
	emailQueueSvc := service.NewEmailQueueService(nil, 1)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	idempotencyCleanupSvc := service.NewIdempotencyCleanupService(nil, cfg)
	schedulerSnapshotSvc := service.NewSchedulerSnapshotService(nil, nil, nil, nil, cfg)
	opsSystemLogSinkSvc := service.NewOpsSystemLogSink(nil)

	cleanup := provideCleanup(
		nil, // entClient
		nil, // redis
		&service.OpsMetricsCollector{},
		&service.OpsAggregationService{},
		&service.OpsAlertEvaluatorService{},
		&service.OpsCleanupService{},
		&service.OpsScheduledReportService{},
		opsSystemLogSinkSvc,
		nil, // opsService
		nil, // opsIngressRejectAggregator
		nil, // apiKeyService
		nil, // authCacheInvalidationWorker
		schedulerSnapshotSvc,
		tokenRefreshSvc,
		accountExpirySvc,
		proxyExpirySvc,
		subscriptionExpirySvc,
		&service.UsageCleanupService{},
		idempotencyCleanupSvc,
		&service.BatchImageCleanupService{},
		nil, // batchImageWorker
		pricingSvc,
		emailQueueSvc,
		billingCacheSvc,
		&service.UsageRecordWorkerPool{},
		&service.SubscriptionService{},
		oauthSvc,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		nil, // grokOAuth
		nil, // openAIGateway
		nil, // scheduledTestRunner
		nil, // backupSvc
		nil, // paymentOrderExpiry
		nil, // creditLotteryExpiry
		nil, // channelMonitorRunner
		nil, // poolSeatBilling
		nil, // quotaFlusher
		nil, // upstreamBillingProbe
		nil, // auditLog
		nil, // promptAudit
	)

	require.NotPanics(t, func() {
		cleanup()
	})
}
