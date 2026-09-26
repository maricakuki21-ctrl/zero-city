package repository

import (
	"context"
	"fmt"
	"strings"
)

// HasSharedPoolOAuthCompactCapability reads the latest compact result that is
// bound to the selected account and the pool's current config version. Older
// successful probes cannot authorize a changed account or pool.
func (r *bizDecipherRepository) HasSharedPoolOAuthCompactCapability(
	ctx context.Context,
	poolID int64,
	accountID int64,
	publishedModel string,
	upstreamModel string,
) (bool, error) {
	publishedModel = strings.TrimSpace(publishedModel)
	upstreamModel = strings.TrimSpace(upstreamModel)
	if r == nil || r.db == nil || poolID <= 0 || accountID <= 0 || publishedModel == "" || upstreamModel == "" {
		return false, nil
	}
	var ready bool
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE((
			SELECT item.success = TRUE AND item.http_status BETWEEN 200 AND 299
			FROM shared_pool_probe_jobs job
			JOIN shared_pool_probe_job_items item ON item.job_id = job.id
			JOIN shared_pools pool ON pool.id = job.pool_id
			JOIN shared_pool_accounts account
			  ON account.id = job.account_id
			 AND account.pool_id = job.pool_id
			WHERE job.pool_id = $1
			  AND job.account_id = $2
			  AND job.status = 'succeeded'
			  AND job.config_version = pool.config_version
			  AND LOWER(BTRIM(job.model_name)) = LOWER(BTRIM($3))
			  AND LOWER(BTRIM(job.upstream_model_name)) = LOWER(BTRIM($4))
			  AND account.deleted_at IS NULL
			  AND LOWER(account.auth_type) = 'oauth'
			  AND item.check_id = 'oauth_responses_compact'
			ORDER BY job.finished_at DESC NULLS LAST, job.created_at DESC, job.id DESC
			LIMIT 1
		), FALSE)`, poolID, accountID, publishedModel, upstreamModel).Scan(&ready)
	return ready, err
}

// sharedPoolOAuthCompactAccountFilterSQL is injected only into the dedicated
// compact scheduler. The ordinary shared-pool query intentionally never calls
// this helper, so existing Responses and Chat Completions routing is unchanged.
//
// The latest optional compact result is fenced to the pool's current config
// version and to both sides of the selected model mapping. This prevents a
// successful probe for one published/upstream pair from authorizing another.
func sharedPoolOAuthCompactAccountFilterSQL() string {
	upstreamModel := `COALESCE(
			NULLIF(TRIM(cfg.upstream_model_name), ''),
			spm_req.upstream_model_name,
			spm_req.model_name,
			''
		)`
	return fmt.Sprintf(`
			AND LOWER(TRIM(spa.auth_type)) = 'oauth'
			AND BTRIM(spm_req.model_name) <> ''
			AND BTRIM(%[1]s) <> ''
			AND COALESCE((
				SELECT item.success = TRUE AND item.http_status BETWEEN 200 AND 299
				FROM shared_pool_probe_jobs job
				JOIN shared_pool_probe_job_items item ON item.job_id = job.id
				WHERE job.pool_id = sp.id
				  AND job.account_id = spa.id
				  AND job.status = 'succeeded'
				  AND job.config_version = sp.config_version
				  AND LOWER(BTRIM(job.model_name)) = LOWER(BTRIM(spm_req.model_name))
				  AND LOWER(BTRIM(job.upstream_model_name)) = LOWER(BTRIM(%[1]s))
				  AND item.check_id = 'oauth_responses_compact'
				ORDER BY job.finished_at DESC NULLS LAST, job.created_at DESC, job.id DESC
				LIMIT 1
			), FALSE)`, upstreamModel)
}
