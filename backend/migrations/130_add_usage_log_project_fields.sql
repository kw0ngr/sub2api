-- Add optional project attribution for usage analytics.
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS project_key VARCHAR(64);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS project_label VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_usage_logs_project_key_created_at
	ON usage_logs(project_key, created_at)
	WHERE project_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_logs_user_project_created_at
	ON usage_logs(user_id, project_key, created_at)
	WHERE project_key IS NOT NULL;
