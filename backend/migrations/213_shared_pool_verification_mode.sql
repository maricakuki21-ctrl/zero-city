-- Migration: 213_shared_pool_verification_mode
-- Adds explicit shared-pool verification policy fields so marketplace/UI can
-- distinguish full-check verified pools from professionally reviewed pools.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS verification_mode VARCHAR(32) NOT NULL DEFAULT 'full_check',
    ADD COLUMN IF NOT EXISTS verification_exemption_reason TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'shared_pools_verification_mode_check'
          AND conrelid = 'shared_pools'::regclass
    ) THEN
        ALTER TABLE shared_pools
            ADD CONSTRAINT shared_pools_verification_mode_check
            CHECK (verification_mode IN ('full_check', 'professional_review'));
    END IF;
END
$$;

UPDATE shared_pools
SET verification_mode = CASE
        WHEN COALESCE(NULLIF(TRIM(verification_mode), ''), '') IN ('full_check', 'professional_review')
            THEN verification_mode
        ELSE 'full_check'
    END,
    verification_exemption_reason = COALESCE(verification_exemption_reason, '')
WHERE TRUE;

COMMENT ON COLUMN shared_pools.verification_mode IS
    'Verification policy for listing trust badge: full_check | professional_review';
COMMENT ON COLUMN shared_pools.verification_exemption_reason IS
    'Operator-provided reason when a non-mainstream pool uses professional review instead of full capability check';
