-- 回滚: 重命名 skill_name 为 agent_type
ALTER TABLE ai_watch_targets RENAME COLUMN skill_name TO agent_type;

-- 恢复唯一约束
ALTER TABLE ai_watch_targets DROP CONSTRAINT IF EXISTS ai_watch_targets_user_id_skill_name_market_code_symbol_code_period_key;
ALTER TABLE ai_watch_targets ADD UNIQUE (user_id, agent_type, market_code, symbol_code, period);
