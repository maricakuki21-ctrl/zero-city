-- Historical milestone claims keep the default cycle_no=1 from migration 201.
-- The exact original cycle cannot be inferred safely from mutable daily checkin rows.
DROP INDEX IF EXISTS idx_checkin_milestone_claims_user_cycle;
