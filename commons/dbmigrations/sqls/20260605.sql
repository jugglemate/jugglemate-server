CREATE TABLE IF NOT EXISTS users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    nickname VARCHAR(128) NOT NULL DEFAULT '',
    user_portrait VARCHAR(512) NOT NULL DEFAULT '',
    login_account VARCHAR(64) NOT NULL DEFAULT '',
    login_pass VARCHAR(128) NOT NULL DEFAULT '',
    status INT NOT NULL DEFAULT 1,
    im_token VARCHAR(512) NOT NULL DEFAULT '',
    created_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    app_key VARCHAR(64) NOT NULL DEFAULT '',
    UNIQUE KEY uk_app_account (app_key, login_account),
    UNIQUE KEY uk_app_userid (app_key, user_id),
    KEY idx_app_key (app_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;