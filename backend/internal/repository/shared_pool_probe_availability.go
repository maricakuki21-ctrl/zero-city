package repository

// sharedPoolProbeAvailabilityAssignments projects persisted pool-level probes,
// not a synthetic score. Both completion paths use the same 24h/7d denominator.
// Account-mode pools have their own eligible-account rollup. Empty windows use
// the existing numeric zero sentinel; consumers must check sample evidence
// rather than treating that sentinel as a measured failure rate.
//
// $1 is the pool ID. Stale jobs never insert a history row, and future-dated
// samples must not improve the current window. statement_timestamp(), not
// transaction-start NOW(), includes a result recorded earlier in this same job
// transaction after BeginTx.
const sharedPoolProbeAvailabilityAssignments = `
	(today_availability, seven_day_availability) = (
		SELECT
			COALESCE(ROUND(
				100.0 * COUNT(*) FILTER (WHERE success AND checked_at >= statement_timestamp() - INTERVAL '24 hours')
				/ NULLIF(COUNT(*) FILTER (WHERE checked_at >= statement_timestamp() - INTERVAL '24 hours'), 0), 2), 0),
			COALESCE(ROUND(
				100.0 * COUNT(*) FILTER (WHERE success)
				/ NULLIF(COUNT(*), 0), 2), 0)
		FROM shared_pool_probe_histories
		WHERE pool_id = $1
		  AND account_id IS NULL
		  AND checked_at >= statement_timestamp() - INTERVAL '7 days'
		  AND checked_at <= statement_timestamp()
	),`
