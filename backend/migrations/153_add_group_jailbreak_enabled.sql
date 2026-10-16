-- 153: 分组级破限开关（仅 grok 平台分组使用）
-- 当 jailbreak_enabled=true 且分组平台为 grok 时，该分组下的请求走破限链：
-- S7 system 注入 + remap 改写 + 拒绝检测 + ladder 同账号重试。
-- 策略本体（提示词/词表/模板）存在 settings 表 jailbreak_grok_policy key 下，热更新。
ALTER TABLE groups ADD COLUMN IF NOT EXISTS jailbreak_enabled BOOLEAN NOT NULL DEFAULT FALSE;
