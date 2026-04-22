-- Fix timestamp timezone issue
-- Change timestamp without time zone to timestamp with time zone to avoid ambiguity

-- 1. Change trade_tracks table timestamps
ALTER TABLE trade_tracks
    ALTER COLUMN entry_time TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN exit_time TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN updated_at TYPE TIMESTAMP WITH TIME ZONE;

-- 2. Change signals table timestamps
ALTER TABLE signals
    ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN updated_at TYPE TIMESTAMP WITH TIME ZONE;

-- 3. Change trading_opportunities table timestamps
ALTER TABLE trading_opportunities
    ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN updated_at TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN first_signal_at TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN last_signal_at TYPE TIMESTAMP WITH TIME ZONE;

-- 4. Change klines table timestamps (if applicable)
ALTER TABLE klines
    ALTER COLUMN open_time TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN close_time TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE;

-- 5. Change backtest_runs table timestamps
ALTER TABLE backtest_runs
    ALTER COLUMN start_time TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN end_time TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE;

-- 6. Change price_boxes table timestamps
ALTER TABLE price_boxes
    ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE,
    ALTER COLUMN updated_at TYPE TIMESTAMP WITH TIME ZONE;

-- Verify the changes
SELECT table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND column_name IN ('entry_time', 'exit_time', 'created_at', 'updated_at',
                      'open_time', 'close_time', 'first_signal_at', 'last_signal_at',
                      'start_time', 'end_time')
ORDER BY table_name, column_name;
