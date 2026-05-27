ALTER TABLE ai_watch_targets
    ADD COLUMN IF NOT EXISTS direction VARCHAR(10) NOT NULL DEFAULT 'long';

UPDATE ai_watch_targets
SET direction = 'long'
WHERE direction IS NULL OR direction = '';

ALTER TABLE ai_watch_targets
    DROP CONSTRAINT IF EXISTS ai_watch_targets_user_id_skill_name_market_code_symbol_code_period_key;

ALTER TABLE ai_watch_targets
    ADD UNIQUE (user_id, skill_name, market_code, symbol_code, period, direction);
