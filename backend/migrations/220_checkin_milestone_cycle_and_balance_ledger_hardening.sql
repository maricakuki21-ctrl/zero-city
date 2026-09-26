-- Repair legacy milestone uniqueness by column shape, then backfill only the
-- missing audit ledger. Historical rewards have already changed users.balance;
-- this migration must never replay that balance mutation.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

DO $$
DECLARE
    constraint_row RECORD;
    correct_constraint_count INT;
    legacy_constraint_count INT;
BEGIN
    SELECT COUNT(*)
    INTO correct_constraint_count
    FROM pg_constraint c
    JOIN pg_index i ON i.indexrelid = c.conindid
    WHERE c.conrelid = 'checkin_milestone_claims'::regclass
      AND c.contype = 'u'
      AND c.convalidated
      AND i.indisvalid
      AND i.indisready
      AND ARRAY(
          SELECT a.attname::text
          FROM unnest(c.conkey) WITH ORDINALITY AS cols(attnum, position)
          JOIN pg_attribute a
            ON a.attrelid = c.conrelid
           AND a.attnum = cols.attnum
          ORDER BY cols.position
      ) = ARRAY['user_id', 'cycle_no', 'milestone_days']::text[];

    IF correct_constraint_count <> 1 THEN
        RAISE EXCEPTION
            'expected exactly one valid unique constraint on checkin_milestone_claims(user_id, cycle_no, milestone_days), found %',
            correct_constraint_count;
    END IF;

    FOR constraint_row IN
        SELECT c.conname
        FROM pg_constraint c
        WHERE c.conrelid = 'checkin_milestone_claims'::regclass
          AND c.contype = 'u'
          AND ARRAY(
              SELECT a.attname::text
              FROM unnest(c.conkey) WITH ORDINALITY AS cols(attnum, position)
              JOIN pg_attribute a
                ON a.attrelid = c.conrelid
               AND a.attnum = cols.attnum
              ORDER BY cols.position
          ) = ARRAY['user_id', 'milestone_days']::text[]
    LOOP
        EXECUTE format(
            'ALTER TABLE checkin_milestone_claims DROP CONSTRAINT %I',
            constraint_row.conname
        );
    END LOOP;

    SELECT COUNT(*)
    INTO legacy_constraint_count
    FROM pg_constraint c
    WHERE c.conrelid = 'checkin_milestone_claims'::regclass
      AND c.contype = 'u'
      AND ARRAY(
          SELECT a.attname::text
          FROM unnest(c.conkey) WITH ORDINALITY AS cols(attnum, position)
          JOIN pg_attribute a
            ON a.attrelid = c.conrelid
           AND a.attnum = cols.attnum
          ORDER BY cols.position
      ) = ARRAY['user_id', 'milestone_days']::text[];

    IF legacy_constraint_count <> 0 THEN
        RAISE EXCEPTION
            'legacy unique constraints on checkin_milestone_claims(user_id, milestone_days) remain: %',
            legacy_constraint_count;
    END IF;
END
$$;

ALTER TABLE user_balance_ledger
    DROP CONSTRAINT IF EXISTS user_balance_ledger_source_type_check,
    ADD CONSTRAINT user_balance_ledger_source_type_check CHECK (source_type IN (
        'daily_checkin',
        'checkin_jackpot_share',
        'credit_lottery_jackpot_win',
        'credit_lottery_jackpot_share',
        'shared_pool_owner_wallet_transfer',
        'checkin_milestone'
    ));

INSERT INTO user_balance_ledger (
    user_id,
    source_type,
    source_id,
    amount,
    balance_after,
    status,
    note,
    created_at,
    posted_at
)
SELECT
    claim.user_id,
    'checkin_milestone',
    claim.cycle_no::text || ':' || claim.milestone_days::text || ':' || claim.id::text,
    claim.balance_reward,
    claim.balance_after,
    'posted',
    'Daily fortune cycle ' || claim.cycle_no::text || ' ' || claim.milestone_days::text || '-day milestone gift',
    claim.claimed_at,
    claim.claimed_at
FROM checkin_milestone_claims claim
WHERE claim.balance_reward > 0
ON CONFLICT DO NOTHING;
