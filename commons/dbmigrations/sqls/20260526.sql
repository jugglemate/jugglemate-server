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