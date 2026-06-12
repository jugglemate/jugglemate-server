CREATE TABLE IF NOT EXISTS `agent_twin_tombstones` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `unique_name` varchar(32) NOT NULL,
  `owner_id` varchar(64) DEFAULT NULL,
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_agent_twin_tombstone` (`app_key`,`unique_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_twins` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `unique_name` varchar(32) NOT NULL,
  `bot_id` varchar(32) DEFAULT NULL,
  `display_name` varchar(64) NOT NULL,
  `avatar_url` varchar(512) DEFAULT NULL,
  `greeting` text,
  `prompts` text,
  `owner_id` varchar(64) NOT NULL,
  `status` varchar(32) DEFAULT 'untrained',
  `active_version` varchar(32) DEFAULT NULL,
  `training_mode` varchar(32) DEFAULT NULL,
  `materials_count` int NOT NULL DEFAULT 0,
  `sync_status` varchar(32) NOT NULL DEFAULT 'pending',
  `sync_error` text,
  `last_synced_at` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_agent_twin_name` (`app_key`,`unique_name`),
  UNIQUE KEY `uniq_agent_twin_bot` (`app_key`,`bot_id`),
  KEY `idx_agent_twin_owner` (`app_key`,`owner_id`),
  KEY `idx_agent_twin_sync` (`app_key`,`sync_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_materials` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `unique_name` varchar(32) NOT NULL,
  `material_id` varchar(64) NOT NULL,
  `type` varchar(16) NOT NULL,
  `title` varchar(255) DEFAULT NULL,
  `source` varchar(16) NOT NULL,
  `content` mediumtext,
  `url` varchar(1024) DEFAULT NULL,
  `file_path` varchar(1024) DEFAULT NULL,
  `size_bytes` bigint NOT NULL DEFAULT 0,
  `sync_status` varchar(32) NOT NULL DEFAULT 'pending',
  `sync_error` text,
  `last_synced_at` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_agent_material` (`app_key`,`material_id`),
  KEY `idx_agent_material_twin` (`app_key`,`unique_name`,`id`),
  KEY `idx_agent_material_sync` (`app_key`,`sync_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_jobs` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `job_id` varchar(64) NOT NULL,
  `unique_name` varchar(32) NOT NULL,
  `type` varchar(32) NOT NULL,
  `status` varchar(32) NOT NULL,
  `progress` int DEFAULT NULL,
  `result_json` json DEFAULT NULL,
  `error_code` varchar(64) DEFAULT NULL,
  `error_message` text,
  `agent_created_at` datetime(3) DEFAULT NULL,
  `started_at` datetime(3) DEFAULT NULL,
  `finished_at` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_agent_job` (`app_key`,`job_id`),
  KEY `idx_agent_job_twin` (`app_key`,`unique_name`,`id`),
  KEY `idx_agent_job_status` (`app_key`,`type`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_versions` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `unique_name` varchar(32) NOT NULL,
  `version` varchar(32) NOT NULL,
  `mode` varchar(32) NOT NULL,
  `active` tinyint(1) NOT NULL DEFAULT 0,
  `training_job_id` varchar(64) DEFAULT NULL,
  `materials_count` int NOT NULL DEFAULT 0,
  `agent_created_at` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_agent_version` (`app_key`,`unique_name`,`version`),
  KEY `idx_agent_version_twin` (`app_key`,`unique_name`,`id`),
  KEY `idx_agent_version_active` (`app_key`,`unique_name`,`active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `agent_evaluations` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `app_key` varchar(20) NOT NULL,
  `evaluation_id` varchar(64) NOT NULL,
  `unique_name` varchar(32) NOT NULL,
  `version` varchar(32) DEFAULT NULL,
  `overall_score` decimal(4,2) DEFAULT NULL,
  `dimensions_json` json DEFAULT NULL,
  `summary_md` mediumtext,
  `agent_created_at` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_agent_evaluation` (`app_key`,`evaluation_id`),
  KEY `idx_agent_evaluation_twin` (`app_key`,`unique_name`,`id`)
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
  `source` varchar(32) DEFAULT NULL,
  `platform` varchar(32) DEFAULT NULL,
  `conver_type` int DEFAULT NULL,
  `raw_payload` json DEFAULT NULL,
  `msg_time` datetime(3) DEFAULT NULL,
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_agent_message_im` (`app_key`,`im_msg_id`,`role`),
  KEY `idx_agent_message_twin_customer` (`app_key`,`unique_name`,`customer_id`,`id`),
  KEY `idx_agent_message_agent` (`app_key`,`agent_message_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
