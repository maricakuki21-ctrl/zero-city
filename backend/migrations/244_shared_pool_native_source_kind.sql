-- Native draft creation writes owner_native_r1; preserve all existing source kinds.
ALTER TABLE shared_pools
    DROP CONSTRAINT IF EXISTS shared_pools_source_kind_check,
    ADD CONSTRAINT shared_pools_source_kind_check
        CHECK (source_kind IN ('user', 'legacy_seed', 'imported', 'owner_native_r1'));
