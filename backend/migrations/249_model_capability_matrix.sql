-- Model capability matrix.
--
-- Declares what each catalog model can consume and produce, which provider
-- protocol adapts it, and whether it may orchestrate other models. Workbench
-- steps, uploaded packages and marketplace listings declare required
-- capabilities against this matrix instead of assuming every model is
-- universally compatible.

ALTER TABLE model_catalog
    ADD COLUMN IF NOT EXISTS modalities_in  JSONB NOT NULL DEFAULT '["text"]'::jsonb,
    ADD COLUMN IF NOT EXISTS modalities_out JSONB NOT NULL DEFAULT '["text"]'::jsonb,
    ADD COLUMN IF NOT EXISTS adapter_kind   VARCHAR(32) NOT NULL DEFAULT 'openai_chat',
    ADD COLUMN IF NOT EXISTS context_window INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS orchestrator   BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS runtime_role   VARCHAR(24) NOT NULL DEFAULT 'text';

CREATE INDEX IF NOT EXISTS idx_model_catalog_runtime_role
    ON model_catalog(runtime_role, enabled);

-- Conservative backfill derived only from data that is already in the table.
-- Vision-tagged models accept images; embedding models emit vectors; reasoning
-- models at Pro/Frontier/Team tier may drive other models; the provider decides
-- the wire protocol, with the Responses tag winning over the chat default.
UPDATE model_catalog
   SET modalities_in = '["text", "image"]'::jsonb
 WHERE capability_tags ? 'vision'
   AND modalities_in = '["text"]'::jsonb;

UPDATE model_catalog
   SET runtime_role = 'embedding',
       modalities_out = '["embedding"]'::jsonb
 WHERE capability_tags ? 'embedding';

UPDATE model_catalog
   SET orchestrator = TRUE
 WHERE tier_label IN ('Pro', 'Frontier', 'Team')
   AND capability_tags ? 'reasoning';

UPDATE model_catalog
   SET adapter_kind = CASE provider
        WHEN 'anthropic' THEN 'anthropic_messages'
        WHEN 'google'    THEN 'google_generate_content'
        ELSE 'openai_chat'
   END
 WHERE adapter_kind = 'openai_chat';

UPDATE model_catalog
   SET adapter_kind = 'openai_responses'
 WHERE capability_tags ? 'responses';
