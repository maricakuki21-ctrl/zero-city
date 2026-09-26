-- Saving a result appends an event and advances counters. Terminal business
-- fields, including inputs, outputs' lineage and settled money, stay immutable.
CREATE OR REPLACE FUNCTION protect_workbench_run_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.run_id IS DISTINCT FROM OLD.run_id
       OR NEW.owner_user_id IS DISTINCT FROM OLD.owner_user_id
       OR NEW.workspace_id IS DISTINCT FROM OLD.workspace_id
       OR NEW.capability IS DISTINCT FROM OLD.capability
       OR NEW.input_snapshot IS DISTINCT FROM OLD.input_snapshot
       OR NEW.input_sha256 IS DISTINCT FROM OLD.input_sha256
       OR NEW.replay_of_run_id IS DISTINCT FROM OLD.replay_of_run_id
       OR NEW.forked_from_run_id IS DISTINCT FROM OLD.forked_from_run_id
       OR NEW.accepted_quote_id IS DISTINCT FROM OLD.accepted_quote_id
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'workbench run immutable fields cannot change';
    END IF;
    IF OLD.state IN ('succeeded', 'failed', 'cancelled') THEN
        IF (to_jsonb(NEW) - ARRAY['version','next_event_seq','updated_at'])
              IS DISTINCT FROM (to_jsonb(OLD) - ARRAY['version','next_event_seq','updated_at'])
           OR NEW.version <> OLD.version + 1
           OR NEW.next_event_seq <> OLD.next_event_seq + 1
           OR NEW.updated_at < OLD.updated_at THEN
            RAISE EXCEPTION 'workbench terminal lineage is immutable';
        END IF;
        RETURN NEW;
    END IF;
    IF OLD.canonical_request_id IS NOT NULL AND NEW.canonical_request_id IS DISTINCT FROM OLD.canonical_request_id
       OR OLD.canonical_usage_event_id IS NOT NULL AND NEW.canonical_usage_event_id IS DISTINCT FROM OLD.canonical_usage_event_id
       OR OLD.ledger_journal_id IS NOT NULL AND NEW.ledger_journal_id IS DISTINCT FROM OLD.ledger_journal_id
       OR OLD.canonical_media_business_event_id IS NOT NULL AND NEW.canonical_media_business_event_id IS DISTINCT FROM OLD.canonical_media_business_event_id THEN
        RAISE EXCEPTION 'workbench canonical lineage is write-once';
    END IF;
    IF NEW.version <> OLD.version + 1 THEN
        RAISE EXCEPTION 'workbench run version must advance exactly once';
    END IF;
    IF NEW.next_event_seq < OLD.next_event_seq OR NEW.lease_epoch < OLD.lease_epoch THEN
        RAISE EXCEPTION 'workbench run fencing values cannot move backward';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
