CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` varchar(32) NOT NULL,
  `nickname` varchar(50) NOT NULL DEFAULT '',
  `avator` varchar(200) NOT NULL DEFAULT '',
  `login_account` varchar(64) NOT NULL DEFAULT '',
  `email` varchar(128) NOT NULL DEFAULT '',
  `login_pass` varchar(128) NOT NULL DEFAULT '',
  `role` tinyint NOT NULL DEFAULT 1,
  `status` int NOT NULL DEFAULT 1,
  `im_token` varchar(512) NOT NULL DEFAULT '',
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `app_key` varchar(20) NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_account` (`app_key`,`login_account`),
  UNIQUE KEY `uk_app_email` (`app_key`,`email`),
  UNIQUE KEY `uk_app_userid` (`app_key`,`user_id`),
  KEY `idx_app_key` (`app_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `customers` (
  `id` int NOT NULL AUTO_INCREMENT,
  `customer_id` varchar(32) DEFAULT NULL,
  `nickname` varchar(50) DEFAULT NULL,
  `avator` varchar(200) DEFAULT NULL,
  `phone` varchar(50) DEFAULT NULL,
  `email` varchar(50) DEFAULT NULL,
  `identifier` varchar(255) DEFAULT NULL,
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `app_key` varchar(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_id` (`app_key`,`customer_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `customerchannelrels` (
  `id` int NOT NULL AUTO_INCREMENT,
  `customer_id` varchar(32) DEFAULT '',
  `channel_id` varchar(32) DEFAULT '',
  `source_id` varchar(32) DEFAULT '',
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `app_key` varchar(20) DEFAULT '',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `tickets` (
  `id` int NOT NULL AUTO_INCREMENT,
  `ticket_id` varchar(50) DEFAULT '',
  `source_id` varchar(32) DEFAULT '',
  `assignee_id` varchar(32) DEFAULT NULL,
  `customer_id` varchar(32) DEFAULT '',
  `channel_id` varchar(32) DEFAULT '',
  `status` tinyint DEFAULT 0,
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `app_key` varchar(20) DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_ticketid` (`app_key`,`ticket_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `apps` (
  `id` int NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) DEFAULT '',
  `app_secret` varchar(50) DEFAULT '',
  `app_status` tinyint DEFAULT 0,
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `app_name` varchar(100) DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_appkey` (`app_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `appexts` (
  `id` int NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) DEFAULT NULL,
  `app_item_key` varchar(50) DEFAULT NULL,
  `app_item_value` varchar(2048) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_key` (`app_key`,`app_item_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `aibots` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `bot_id` varchar(32) DEFAULT NULL,
  `bot_name` varchar(50) DEFAULT NULL,
  `bot_portrait` varchar(200) DEFAULT NULL,
  `prompts` text,
  `owner_id` varchar(32) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `app_key` varchar(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_botid` (`app_key`,`bot_id`),
  KEY `idx_owner` (`app_key`,`owner_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_twins` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `unique_name` varchar(64) NOT NULL,
  `bot_id` varchar(32) DEFAULT NULL,
  `display_name` varchar(50) DEFAULT NULL,
  `avatar_url` varchar(200) DEFAULT NULL,
  `greeting` varchar(500) DEFAULT '',
  `prompts` text,
  `owner_id` varchar(32) DEFAULT NULL,
  `status` varchar(20) DEFAULT 'untrained',
  `active_version` varchar(20) DEFAULT '',
  `training_mode` varchar(20) DEFAULT '',
  `materials_count` int DEFAULT 0,
  `sync_status` varchar(20) DEFAULT 'pending',
  `sync_error` varchar(500) DEFAULT '',
  `last_synced_at` datetime(3) DEFAULT NULL,
  `domain` varchar(32) DEFAULT '' COMMENT '业务域: order/pre_sale/complaint/general',
  `auto_reply_enabled` tinyint NOT NULL DEFAULT 0 COMMENT '是否开启自动回复',
  `confidence_threshold` float NOT NULL DEFAULT 0.75 COMMENT '自动回复置信度阈值',
  `fallback_reply` varchar(500) DEFAULT '' COMMENT '兜底回复文案',
  `max_context_rounds` int NOT NULL DEFAULT 10 COMMENT '最大上下文轮数',
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_twin` (`app_key`,`unique_name`),
  KEY `idx_botid` (`app_key`,`bot_id`),
  KEY `idx_owner` (`app_key`,`owner_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_twin_tombstones` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `unique_name` varchar(64) NOT NULL,
  `owner_id` varchar(32) NOT NULL,
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_tomb` (`app_key`,`unique_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_materials` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `unique_name` varchar(64) NOT NULL,
  `material_id` varchar(48) NOT NULL,
  `type` varchar(32) DEFAULT NULL,
  `title` varchar(200) DEFAULT NULL,
  `source` varchar(32) DEFAULT NULL,
  `content` text,
  `url` varchar(500) DEFAULT NULL,
  `file_path` varchar(500) DEFAULT NULL,
  `size_bytes` bigint DEFAULT 0,
  `sync_status` varchar(20) DEFAULT 'pending',
  `sync_error` varchar(500) DEFAULT '',
  `last_synced_at` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_material` (`app_key`,`material_id`),
  KEY `idx_twin_material` (`app_key`,`unique_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_jobs` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `job_id` varchar(48) NOT NULL,
  `agent_job_id` varchar(64) DEFAULT '',
  `unique_name` varchar(64) NOT NULL,
  `type` varchar(32) DEFAULT NULL,
  `status` varchar(32) DEFAULT NULL,
  `progress` int DEFAULT NULL,
  `result_json` json DEFAULT NULL,
  `error_code` varchar(32) DEFAULT '',
  `error_message` varchar(500) DEFAULT '',
  `agent_created_at` datetime(3) DEFAULT NULL,
  `started_at` datetime(3) DEFAULT NULL,
  `finished_at` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_job` (`app_key`,`job_id`),
  KEY `idx_twin_job` (`app_key`,`unique_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_versions` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `unique_name` varchar(64) NOT NULL,
  `version` varchar(20) NOT NULL,
  `mode` varchar(20) DEFAULT NULL,
  `active` tinyint(1) DEFAULT 0,
  `training_job_id` varchar(48) DEFAULT NULL,
  `materials_count` int DEFAULT 0,
  `agent_created_at` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_version` (`app_key`,`unique_name`,`version`),
  KEY `idx_active_version` (`app_key`,`unique_name`,`active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_evaluations` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `evaluation_id` varchar(48) NOT NULL,
  `unique_name` varchar(64) NOT NULL,
  `version` varchar(20) DEFAULT NULL,
  `overall_score` double DEFAULT NULL,
  `dimensions_json` json DEFAULT NULL,
  `summary_md` text,
  `agent_created_at` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_eval` (`app_key`,`evaluation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_messages` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `unique_name` varchar(32) NOT NULL,
  `customer_id` varchar(64) NOT NULL,
  `im_msg_id` varchar(128) DEFAULT NULL,
  `agent_message_id` varchar(64) DEFAULT NULL,
  `session_id` varchar(64) DEFAULT NULL,
  `role` varchar(16) NOT NULL,
  `text` mediumtext NOT NULL,
  `fallback` tinyint(1) NOT NULL DEFAULT 0,
  `suggestion_status` varchar(16) DEFAULT '' COMMENT '空=非建议/pending=待审核/adopted=已采纳/edited=已编辑/rejected=已拒绝',
  `pending_source` varchar(16) DEFAULT '' COMMENT 'assist=协助模式生成/auto=自动模式发送',
  `source` varchar(32) DEFAULT NULL,
  `platform` varchar(32) DEFAULT NULL,
  `conver_type` int DEFAULT NULL,
  `raw_payload` json DEFAULT NULL,
  `msg_time` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_im_msg` (`app_key`,`im_msg_id`),
  KEY `idx_agent_message_twin_customer` (`app_key`,`unique_name`,`customer_id`,`id`),
  KEY `idx_agent_message_agent` (`app_key`,`agent_message_id`),
  KEY `idx_suggestion` (`app_key`,`session_id`,`suggestion_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_sessions` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `session_id` varchar(32) NOT NULL COMMENT '会话唯一ID',
  `unique_name` varchar(64) NOT NULL COMMENT '关联Agent',
  `customer_id` varchar(32) NOT NULL COMMENT '终端用户ID',
  `platform` varchar(32) DEFAULT '' COMMENT '平台: snailchat/douyin/feishu',
  `platform_conv_id` varchar(128) DEFAULT '' COMMENT '平台侧会话ID',
  `operator_id` varchar(32) DEFAULT '' COMMENT '当前坐席ID',
  `status` tinyint NOT NULL DEFAULT 0 COMMENT '0:等待 1:进行中 2:已关闭',
  `auto_mode` tinyint NOT NULL DEFAULT 0 COMMENT '0:人工 1:Agent接管自动回复',
  `msg_count` int DEFAULT 0 COMMENT '消息计数',
  `tags` json DEFAULT NULL COMMENT '标签',
  `summary` text COMMENT '会话小结',
  `first_msg_at` datetime(3) DEFAULT NULL COMMENT '首条消息时间',
  `last_msg_at` datetime(3) DEFAULT NULL COMMENT '末条消息时间',
  `closed_at` datetime(3) DEFAULT NULL COMMENT '关闭时间',
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_session` (`app_key`,`session_id`),
  KEY `idx_customer` (`app_key`,`customer_id`),
  KEY `idx_agent` (`app_key`,`unique_name`),
  KEY `idx_operator` (`app_key`,`operator_id`),
  KEY `idx_status` (`app_key`,`status`),
  KEY `idx_last_msg` (`app_key`,`last_msg_at`),
  KEY `idx_platform_conv` (`app_key`,`platform_conv_id`,`unique_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_feedbacks` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `feedback_id` varchar(32) NOT NULL COMMENT '反馈唯一ID',
  `session_id` varchar(32) NOT NULL COMMENT '关联会话',
  `unique_name` varchar(64) NOT NULL COMMENT '关联Agent',
  `agent_msg_id` varchar(64) NOT NULL COMMENT 'Agent消息ID',
  `agent_reply_text` text COMMENT 'Agent原始回复',
  `action` varchar(16) NOT NULL COMMENT 'adopted/edited/rejected',
  `final_reply_text` text COMMENT '最终发出内容',
  `edit_diff` text COMMENT '编辑差异',
  `reject_reason` varchar(64) DEFAULT '' COMMENT '拒绝原因',
  `operator_id` varchar(32) NOT NULL COMMENT '操作坐席',
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_feedback` (`app_key`,`feedback_id`),
  KEY `idx_agent_fb` (`app_key`,`unique_name`),
  KEY `idx_session_fb` (`app_key`,`session_id`),
  KEY `idx_action` (`app_key`,`action`),
  KEY `idx_created` (`app_key`,`created_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
