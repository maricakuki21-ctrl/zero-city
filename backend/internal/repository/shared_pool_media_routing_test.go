package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type captureSharedPoolMediaRoutingQueryMatcher struct {
	actual *string
}

func (m captureSharedPoolMediaRoutingQueryMatcher) Match(_ string, actual string) error {
	*m.actual = actual
	return nil
}

func TestGetSharedPoolMediaAccessKeyFiltersBeforePoolRanking(t *testing.T) {
	var query string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(captureSharedPoolMediaRoutingQueryMatcher{actual: &query}))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectQuery("endpoint-aware shared-pool media route").
		WithArgs(int64(17), "grok-imagine", service.SharedPoolEndpointImageGeneration).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	route, err := repo.GetSharedPoolMediaAccessKeyByAPIKeyID(
		context.Background(), 17, "grok-imagine", service.SharedPoolEndpointImageGeneration,
	)
	if err != nil {
		t.Fatalf("route lookup: %v", err)
	}
	if route != nil {
		t.Fatalf("expected empty fixture to return no route, got %#v", route)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}

	// These predicates live inside each pool candidate's model LATERAL join,
	// before the global ORDER/LIMIT. Therefore a high-ranked ineligible pool is
	// removed and the same unified key can fall through to the next eligible one.
	for _, fragment := range []string{
		"JOIN shared_pool_model_endpoints media_endpoint",
		"media_endpoint.endpoint_type = $3",
		"media_endpoint.enabled = TRUE",
		"media_endpoint.gate_status = 'passed'",
		"media_endpoint.media_probe_expires_at > NOW()",
		"JOIN shared_pool_media_endpoint_probes media_probe",
		"media_probe.id = media_endpoint.last_media_probe_id",
		"media_probe.endpoint_type = media_endpoint.endpoint_type",
		"media_probe.pool_config_version = sp.config_version",
		"media_probe.result_status = 'passed'",
		"media_probe.output_observed = TRUE",
		"media_probe.expires_at > NOW()",
		"spm.pricing_source = 'official_catalog'",
		"spm.pricing_source = 'owner_custom'",
		"media_endpoint.pricing_status = 'ready'",
		"FROM shared_pool_price_versions media_price",
		"media_price.effective_from <= NOW()",
		"AND spm_req.model_name IS NOT NULL",
		"ORDER BY sp.rank_weight DESC",
		"LIMIT 1",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("endpoint-aware route query missing %q\n%s", fragment, query)
		}
	}
	if strings.Index(query, "AND spm_req.model_name IS NOT NULL") > strings.Index(query, "ORDER BY sp.rank_weight DESC") {
		t.Fatalf("media eligibility must filter candidates before ranking\n%s", query)
	}
	if strings.Contains(strings.ToLower(query), "verification_mode") || strings.Contains(strings.ToLower(query), "verification_exemption") {
		t.Fatalf("professional-review metadata must never bypass endpoint-specific media evidence\n%s", query)
	}
}

func TestGetSharedPoolMediaAccessKeyBindsLastProbeAccountAndCredentialMode(t *testing.T) {
	var query string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(captureSharedPoolMediaRoutingQueryMatcher{actual: &query}))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectQuery("account-bound shared-pool media route").
		WithArgs(int64(17), "grok-imagine-video", service.SharedPoolEndpointVideo).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err = repo.GetSharedPoolMediaAccessKeyByAPIKeyID(
		context.Background(), 17, "grok-imagine-video", service.SharedPoolEndpointVideo,
	)
	if err != nil {
		t.Fatalf("route lookup: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}

	for _, fragment := range []string{
		"COALESCE(media_probe.account_id, 0) AS media_probe_account_id",
		"COALESCE(media_probe.account_config_version, 0) AS media_probe_account_config_version",
		"spa.id = NULLIF(spm_req.media_probe_account_id, 0)",
		"spa.config_version = spm_req.media_probe_account_config_version",
		"spa_req.id = spm_req.media_probe_account_id",
		"spm_req.media_probe_account_id = 0",
		"spm_req.media_probe_account_config_version = 0",
		"media_probe.endpoint_type <> 'video' OR media_probe.async_terminal_observed = TRUE",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("account-bound route query missing %q\n%s", fragment, query)
		}
	}
}
