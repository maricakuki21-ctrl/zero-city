-- Migration 266 was reserved during the tavern commerce split and never
-- carried a schema change. Keep the immutable post-234 registry contiguous
-- without inventing business data or mutating existing environments.
SELECT 1;
