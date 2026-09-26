-- Refresh BizDecipher brand defaults for installs that still carry the legacy Aura API shell.
-- This intentionally updates only public OEM branding keys; secrets and runtime data are untouched.

INSERT INTO settings (key, value, updated_at)
VALUES
    ('site_name', 'BizDecipher', NOW()),
    ('site_logo', '/logo.svg', NOW())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = NOW()
WHERE settings.key IN ('site_name', 'site_logo')
  AND (
    settings.value = ''
    OR settings.value IS NULL
    OR settings.value ILIKE '%Aura API%'
    OR settings.value ILIKE '%logo.png%'
    OR settings.key = 'site_logo'
  );
