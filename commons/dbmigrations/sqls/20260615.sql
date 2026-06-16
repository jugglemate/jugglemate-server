-- session lookup by platform_conv_id
-- 方案B：按 IM 会话维度 (platform_conv_id + unique_name) 查找 session
ALTER TABLE `agent_sessions` ADD INDEX `idx_platform_conv` (`app_key`, `platform_conv_id`, `unique_name`);
