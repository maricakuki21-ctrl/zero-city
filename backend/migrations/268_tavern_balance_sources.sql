ALTER TABLE user_balance_ledger
    DROP CONSTRAINT user_balance_ledger_source_type_check;
ALTER TABLE user_balance_ledger
    ADD CONSTRAINT user_balance_ledger_source_type_check CHECK (source_type IN (
        'daily_checkin',
        'checkin_jackpot_share',
        'credit_lottery_jackpot_win',
        'credit_lottery_jackpot_share',
        'shared_pool_owner_wallet_transfer',
        'checkin_milestone',
        'creator_column_purchase',
        'creator_column_refund',
        'capability_asset_purchase',
        'capability_asset_refund',
        'marketplace_order_payment',
        'marketplace_order_refund',
        'tavern_ticket_purchase',
        'tavern_ticket_refund'
    ));
