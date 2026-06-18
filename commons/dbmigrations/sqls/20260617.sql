-- 重建 agent_messages 表：新增 pending_source 字段，区分协助/自动模式
DROP TABLE IF EXISTS `agent_messages`;

CREATE TABLE `agent_messages` (
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

-- agent_jobs 表新增 agent_job_id 列，用于存储 agent 服务返回的真实 job_id
ALTER TABLE `agent_jobs` ADD COLUMN IF NOT EXISTS `agent_job_id` VARCHAR(64) DEFAULT '' AFTER `job_id`;
