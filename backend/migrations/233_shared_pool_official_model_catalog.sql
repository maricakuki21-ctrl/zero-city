-- Keep the shared-pool model identity catalog aligned with the built-in
-- official billing catalog.  Without these identities an otherwise known
-- model is treated as owner-custom and the owner is incorrectly asked to
-- enter the platform price by hand.
INSERT INTO model_catalog (
    provider, model_name, display_name, family, tier_label,
    capability_tags, aliases, default_rate_multiplier, default_rank_weight,
    mainstream, enabled, sort_order
)
VALUES
    ('openai', 'gpt-5.6-sol', 'GPT-5.6 Sol', 'gpt', 'Frontier', '["chat","responses","reasoning","code"]', '["gpt-5.6","gpt5.6","gpt-5.6-high","gpt-5.6-max"]', 1.00, 130, TRUE, TRUE, 5),
    ('openai', 'gpt-5.6-terra', 'GPT-5.6 Terra', 'gpt', 'Frontier', '["chat","responses","reasoning","code"]', '["gpt5.6-terra"]', 1.00, 129, TRUE, TRUE, 6),
    ('openai', 'gpt-5.6-luna', 'GPT-5.6 Luna', 'gpt', 'Frontier', '["chat","responses","reasoning","code"]', '["gpt5.6-luna"]', 1.00, 128, TRUE, TRUE, 7),
    ('openai', 'gpt-5.5-pro', 'GPT-5.5 Pro', 'gpt', 'Pro', '["chat","responses","reasoning","code"]', '["gpt5.5-pro"]', 1.00, 121, TRUE, TRUE, 11),
    ('openai', 'gpt-5.3-codex', 'GPT-5.3 Codex', 'codex', 'Team', '["chat","responses","reasoning","code"]', '["gpt-5.3-codex-spark"]', 1.00, 119, TRUE, TRUE, 12),
    ('google', 'gemini-3.1-pro', 'Gemini 3.1 Pro', 'gemini', 'Pro', '["chat","responses","reasoning","vision"]', '["gemini-3-1-pro"]', 1.00, 116, TRUE, TRUE, 151),
    ('xai', 'grok-4.5', 'Grok 4.5', 'grok', 'Pro', '["chat","reasoning"]', '["grok","grok-latest","grok-4.5-latest"]', 1.00, 96, TRUE, TRUE, 181),
    ('xai', 'grok-4.3', 'Grok 4.3', 'grok', 'Pro', '["chat","reasoning"]', '["grok-4.3-latest"]', 1.00, 94, TRUE, TRUE, 182),
    ('deepseek', 'deepseek-v4-pro', 'DeepSeek V4 Pro', 'deepseek', 'Pro', '["chat","reasoning"]', '["deepseek-v4-pro"]', 1.00, 92, TRUE, TRUE, 201),
    ('deepseek', 'deepseek-v4-flash', 'DeepSeek V4 Flash', 'deepseek', 'Plus', '["chat","fast"]', '["deepseek-v4-flash","deepseek-chat","deepseek-reasoner"]', 1.00, 90, TRUE, TRUE, 202),
    ('zhipu', 'glm-5.1', 'GLM-5.1', 'glm', 'Pro', '["chat","reasoning"]', '["glm-5-1"]', 1.00, 90, TRUE, TRUE, 281),
    ('zhipu', 'glm-5', 'GLM-5', 'glm', 'Pro', '["chat","reasoning"]', '["glm5"]', 1.00, 88, TRUE, TRUE, 282),
    ('moonshot', 'kimi-k2.6', 'Kimi K2.6', 'kimi', 'Pro', '["chat","reasoning","long-context"]', '["kimi-k2-6"]', 1.00, 86, TRUE, TRUE, 261),
    ('minimax', 'minimax-m3', 'MiniMax M3', 'minimax', 'Pro', '["chat","reasoning"]', '["minimax-m-3"]', 1.00, 84, TRUE, TRUE, 291),
    ('doubao', 'doubao-embedding-vision', 'Doubao Embedding Vision', 'embedding', 'Plus', '["embedding","vision"]', '["doubao-embedding-vision-251215"]', 1.00, 70, FALSE, TRUE, 401)
ON CONFLICT (provider, model_name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    family = EXCLUDED.family,
    tier_label = EXCLUDED.tier_label,
    capability_tags = EXCLUDED.capability_tags,
    aliases = EXCLUDED.aliases,
    default_rate_multiplier = EXCLUDED.default_rate_multiplier,
    default_rank_weight = EXCLUDED.default_rank_weight,
    mainstream = EXCLUDED.mainstream,
    enabled = EXCLUDED.enabled,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW();
