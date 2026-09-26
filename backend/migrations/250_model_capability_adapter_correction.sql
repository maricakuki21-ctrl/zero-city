-- Correct the adapter backfill from 249.
--
-- The Responses protocol tag may only upgrade OpenAI-provider models. A
-- non-OpenAI provider keeps its own wire protocol even when an upstream
-- compatibility tag mentions "responses", otherwise the declared adapter would
-- not match the provider that actually serves the model.

UPDATE model_catalog
   SET adapter_kind = CASE provider
        WHEN 'anthropic' THEN 'anthropic_messages'
        WHEN 'google'    THEN 'google_generate_content'
        ELSE 'openai_chat'
   END,
       updated_at = NOW()
 WHERE provider <> 'openai'
   AND adapter_kind = 'openai_responses';
