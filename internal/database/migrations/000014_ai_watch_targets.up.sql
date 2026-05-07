CREATE TABLE IF NOT EXISTS ai_watch_targets (
    id              SERIAL PRIMARY KEY,
    user_id         INTEGER REFERENCES users(id),
    agent_type      VARCHAR(40) NOT NULL,
    market_code     VARCHAR(20) NOT NULL,
    symbol_code     VARCHAR(40) NOT NULL,
    symbol_id       INTEGER REFERENCES symbols(id),
    period          VARCHAR(10) NOT NULL,
    limit_count     INTEGER NOT NULL DEFAULT 120,
    send_feishu     BOOLEAN NOT NULL DEFAULT false,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    data_status     VARCHAR(30) NOT NULL DEFAULT 'pending',
    error_message   TEXT NOT NULL DEFAULT '',
    last_run_at     BIGINT,
    result_json     JSONB,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, agent_type, market_code, symbol_code, period)
);

CREATE INDEX IF NOT EXISTS idx_ai_watch_targets_user_agent ON ai_watch_targets(user_id, agent_type);
CREATE INDEX IF NOT EXISTS idx_ai_watch_targets_enabled ON ai_watch_targets(enabled) WHERE enabled = true;
