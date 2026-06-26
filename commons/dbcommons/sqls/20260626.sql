CREATE TABLE IF NOT EXISTS `inboxes` (
  `id` int NOT NULL AUTO_INCREMENT,
  `inbox_id` varchar(32) DEFAULT NULL,
  `channel_type` varchar(50) DEFAULT NULL,
  `channel_conf` varchar(2000) DEFAULT NULL,
  `name` varchar(50) DEFAULT NULL,
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `app_key` varchar(20) DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_inboxid` (`app_key`,`inbox_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `customerinboxrels` (
  `id` int NOT NULL AUTO_INCREMENT,
  `customer_id` varchar(32) DEFAULT '',
  `inbox_id` varchar(32) DEFAULT '',
  `source_id` varchar(32) DEFAULT '',
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `updated_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `app_key` varchar(20) DEFAULT '',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `inboxmembers` (
  `id` int NOT NULL AUTO_INCREMENT,
  `inbox_id` varchar(45) DEFAULT NULL,
  `member_id` varchar(45) DEFAULT NULL,
  `created_time` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `app_key` varchar(20) DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_memberid` (`app_key`,`inbox_id`,`member_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
