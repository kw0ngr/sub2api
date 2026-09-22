-- Configurable billing multipliers keyed by the final forwarded reasoning effort.
ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS reasoning_effort_multipliers JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE channel_account_stats_model_pricing
    ADD COLUMN IF NOT EXISTS reasoning_effort_multipliers JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN channel_model_pricing.reasoning_effort_multipliers IS
    'Billing multipliers by final reasoning effort; omitted efforts use 1x';
COMMENT ON COLUMN channel_account_stats_model_pricing.reasoning_effort_multipliers IS
    'Account statistics multipliers by final reasoning effort; omitted efforts use 1x';
