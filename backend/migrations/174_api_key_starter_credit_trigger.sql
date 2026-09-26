-- Ensure creating the first API key grants starter credits when registration did not.
-- This is a database-level guard for OAuth/OIDC flows where credit_balance starts at 0.

CREATE OR REPLACE FUNCTION grant_starter_credit_on_first_api_key()
RETURNS trigger AS $$
DECLARE
    configured_amount numeric := 0;
    current_credit numeric := 0;
    topup_amount numeric := 0;
    user_role text := '';
BEGIN
    IF NEW.deleted_at IS NOT NULL THEN
        RETURN NEW;
    END IF;

    SELECT COALESCE(NULLIF(value, '')::numeric, 0)
    INTO configured_amount
    FROM settings
    WHERE key = 'signup_point_balance';

    IF configured_amount <= 0 THEN
        RETURN NEW;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM api_keys
        WHERE user_id = NEW.user_id
          AND deleted_at IS NULL
          AND id <> NEW.id
        LIMIT 1
    ) THEN
        RETURN NEW;
    END IF;

    SELECT role, credit_balance
    INTO user_role, current_credit
    FROM users
    WHERE id = NEW.user_id
      AND deleted_at IS NULL
    FOR UPDATE;

    IF NOT FOUND OR user_role = 'admin' THEN
        RETURN NEW;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM credit_ledger
        WHERE user_id = NEW.user_id
          AND source_type = 'starter'
        LIMIT 1
    ) THEN
        RETURN NEW;
    END IF;

    topup_amount := GREATEST(0, configured_amount - COALESCE(current_credit, 0));
    IF topup_amount <= 0 THEN
        RETURN NEW;
    END IF;

    UPDATE users
    SET credit_balance = credit_balance + topup_amount,
        updated_at = now()
    WHERE id = NEW.user_id;

    INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, created_at, posted_at)
    VALUES (
        NEW.user_id,
        'starter',
        'first_api_key',
        topup_amount,
        current_credit + topup_amount,
        'posted',
        '创建首个 Key 自动赠送积分',
        now(),
        now()
    )
    ON CONFLICT DO NOTHING;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_grant_starter_credit_on_first_api_key ON api_keys;
CREATE TRIGGER trg_grant_starter_credit_on_first_api_key
AFTER INSERT ON api_keys
FOR EACH ROW
EXECUTE FUNCTION grant_starter_credit_on_first_api_key();
