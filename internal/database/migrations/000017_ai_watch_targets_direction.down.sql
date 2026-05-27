ALTER TABLE ai_watch_targets
    DROP CONSTRAINT IF EXISTS ai_watch_targets_user_id_skill_name_market_code_symbol_code_period_direction_key;

ALTER TABLE ai_watch_targets
    ADD UNIQUE (user_id, skill_name, market_code, symbol_code, period);

ALTER TABLE ai_watch_targets
    DROP COLUMN IF EXISTS direction;
