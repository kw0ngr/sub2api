-- Add ttft_sample_count to ops_metrics_hourly / ops_metrics_daily.
--
-- first_token_ms (TTFT) is recorded only for streaming requests. Dashboard
-- pre-aggregation must weight merged TTFT percentiles by rows that actually
-- contributed TTFT samples, not all successful requests.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE ops_metrics_hourly
    ADD COLUMN IF NOT EXISTS ttft_sample_count BIGINT NOT NULL DEFAULT 0;

ALTER TABLE ops_metrics_daily
    ADD COLUMN IF NOT EXISTS ttft_sample_count BIGINT NOT NULL DEFAULT 0;
