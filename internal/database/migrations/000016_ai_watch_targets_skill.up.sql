-- 重命名 agent_type 为 skill_name
ALTER TABLE ai_watch_targets RENAME COLUMN agent_type TO skill_name;

-- 更新唯一约束名称
ALTER TABLE ai_watch_targets DROP CONSTRAINT IF EXISTS ai_watch_targets_user_id_agent_type_market_code_symbol_code_period_key;
ALTER TABLE ai_watch_targets ADD UNIQUE (user_id, skill_name, market_code, symbol_code, period);
