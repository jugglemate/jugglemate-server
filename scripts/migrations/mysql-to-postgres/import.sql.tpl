\set ON_ERROR_STOP on

BEGIN;
SET LOCAL TIME ZONE 'Asia/Shanghai';

CREATE TEMP TABLE stage_apps (id BIGINT, app_key_hex TEXT, app_secret_hex TEXT, app_status SMALLINT, created_time TIMESTAMPTZ, updated_time TIMESTAMPTZ, app_name_hex TEXT) ON COMMIT DROP;
CREATE TEMP TABLE stage_appexts (id BIGINT, app_key_hex TEXT, app_item_key_hex TEXT, app_item_value_hex TEXT, updated_time TIMESTAMPTZ) ON COMMIT DROP;
CREATE TEMP TABLE stage_users (id BIGINT, user_id_hex TEXT, nickname_hex TEXT, avator_hex TEXT, login_account_hex TEXT, email_hex TEXT, login_pass_hex TEXT, role SMALLINT, status INTEGER, im_token_hex TEXT, created_time TIMESTAMPTZ, updated_time TIMESTAMPTZ, app_key_hex TEXT) ON COMMIT DROP;
CREATE TEMP TABLE stage_customers (id BIGINT, customer_id_hex TEXT, nickname_hex TEXT, avator_hex TEXT, phone_hex TEXT, email_hex TEXT, identifier_hex TEXT, created_time TIMESTAMPTZ, updated_time TIMESTAMPTZ, app_key_hex TEXT) ON COMMIT DROP;
CREATE TEMP TABLE stage_inboxes (id BIGINT, inbox_id_hex TEXT, channel_type_hex TEXT, channel_conf_hex TEXT, name_hex TEXT, created_time TIMESTAMPTZ, updated_time TIMESTAMPTZ, app_key_hex TEXT) ON COMMIT DROP;
CREATE TEMP TABLE stage_customerinboxrels (id BIGINT, customer_id_hex TEXT, inbox_id_hex TEXT, source_id_hex TEXT, created_time TIMESTAMPTZ, updated_time TIMESTAMPTZ, app_key_hex TEXT) ON COMMIT DROP;
CREATE TEMP TABLE stage_inboxmembers (id BIGINT, inbox_id_hex TEXT, member_id_hex TEXT, created_time TIMESTAMPTZ, app_key_hex TEXT) ON COMMIT DROP;
CREATE TEMP TABLE stage_tickets (id BIGINT, ticket_id_hex TEXT, source_id_hex TEXT, assignee_id_hex TEXT, customer_id_hex TEXT, inbox_id_hex TEXT, channel_type_hex TEXT, status SMALLINT, created_time TIMESTAMPTZ, updated_time TIMESTAMPTZ, app_key_hex TEXT) ON COMMIT DROP;

\copy stage_apps FROM '__DATA_DIR__/apps.tsv' WITH (FORMAT text, DELIMITER E'\t', NULL '\N')
\copy stage_appexts FROM '__DATA_DIR__/appexts.tsv' WITH (FORMAT text, DELIMITER E'\t', NULL '\N')
\copy stage_users FROM '__DATA_DIR__/users.tsv' WITH (FORMAT text, DELIMITER E'\t', NULL '\N')
\copy stage_customers FROM '__DATA_DIR__/customers.tsv' WITH (FORMAT text, DELIMITER E'\t', NULL '\N')
\copy stage_inboxes FROM '__DATA_DIR__/inboxes.tsv' WITH (FORMAT text, DELIMITER E'\t', NULL '\N')
\copy stage_customerinboxrels FROM '__DATA_DIR__/customerinboxrels.tsv' WITH (FORMAT text, DELIMITER E'\t', NULL '\N')
\copy stage_inboxmembers FROM '__DATA_DIR__/inboxmembers.tsv' WITH (FORMAT text, DELIMITER E'\t', NULL '\N')
\copy stage_tickets FROM '__DATA_DIR__/tickets.tsv' WITH (FORMAT text, DELIMITER E'\t', NULL '\N')

INSERT INTO apps (id,app_key,app_secret,app_status,created_time,updated_time,app_name) SELECT id, convert_from(decode(app_key_hex,'hex'),'UTF8'), convert_from(decode(app_secret_hex,'hex'),'UTF8'), app_status, created_time, updated_time, convert_from(decode(app_name_hex,'hex'),'UTF8') FROM stage_apps;
INSERT INTO appexts (id,app_key,app_item_key,app_item_value,updated_time) SELECT id, convert_from(decode(app_key_hex,'hex'),'UTF8'), convert_from(decode(app_item_key_hex,'hex'),'UTF8'), convert_from(decode(app_item_value_hex,'hex'),'UTF8'), updated_time FROM stage_appexts;
INSERT INTO users (id,user_id,nickname,avator,login_account,email,login_pass,role,status,im_token,created_time,updated_time,app_key) SELECT id, convert_from(decode(user_id_hex,'hex'),'UTF8'), convert_from(decode(nickname_hex,'hex'),'UTF8'), convert_from(decode(avator_hex,'hex'),'UTF8'), convert_from(decode(login_account_hex,'hex'),'UTF8'), convert_from(decode(email_hex,'hex'),'UTF8'), convert_from(decode(login_pass_hex,'hex'),'UTF8'), role, status, convert_from(decode(im_token_hex,'hex'),'UTF8'), created_time, updated_time, convert_from(decode(app_key_hex,'hex'),'UTF8') FROM stage_users;
INSERT INTO customers (id,customer_id,nickname,avator,phone,email,identifier,created_time,updated_time,app_key) SELECT id, convert_from(decode(customer_id_hex,'hex'),'UTF8'), convert_from(decode(nickname_hex,'hex'),'UTF8'), convert_from(decode(avator_hex,'hex'),'UTF8'), convert_from(decode(phone_hex,'hex'),'UTF8'), convert_from(decode(email_hex,'hex'),'UTF8'), convert_from(decode(identifier_hex,'hex'),'UTF8'), created_time, updated_time, convert_from(decode(app_key_hex,'hex'),'UTF8') FROM stage_customers;
INSERT INTO inboxes (id,inbox_id,channel_type,channel_conf,name,created_time,updated_time,app_key) SELECT id, convert_from(decode(inbox_id_hex,'hex'),'UTF8'), convert_from(decode(channel_type_hex,'hex'),'UTF8'), convert_from(decode(channel_conf_hex,'hex'),'UTF8'), convert_from(decode(name_hex,'hex'),'UTF8'), created_time, updated_time, convert_from(decode(app_key_hex,'hex'),'UTF8') FROM stage_inboxes;
INSERT INTO customerinboxrels (id,customer_id,inbox_id,source_id,created_time,updated_time,app_key) SELECT id, convert_from(decode(customer_id_hex,'hex'),'UTF8'), convert_from(decode(inbox_id_hex,'hex'),'UTF8'), convert_from(decode(source_id_hex,'hex'),'UTF8'), created_time, updated_time, convert_from(decode(app_key_hex,'hex'),'UTF8') FROM stage_customerinboxrels;
INSERT INTO inboxmembers (id,inbox_id,member_id,created_time,app_key) SELECT id, convert_from(decode(inbox_id_hex,'hex'),'UTF8'), convert_from(decode(member_id_hex,'hex'),'UTF8'), created_time, convert_from(decode(app_key_hex,'hex'),'UTF8') FROM stage_inboxmembers;
INSERT INTO tickets (id,ticket_id,source_id,assignee_id,customer_id,inbox_id,channel_type,status,created_time,updated_time,app_key) SELECT id, convert_from(decode(ticket_id_hex,'hex'),'UTF8'), convert_from(decode(source_id_hex,'hex'),'UTF8'), convert_from(decode(assignee_id_hex,'hex'),'UTF8'), convert_from(decode(customer_id_hex,'hex'),'UTF8'), convert_from(decode(inbox_id_hex,'hex'),'UTF8'), convert_from(decode(channel_type_hex,'hex'),'UTF8'), status, created_time, updated_time, convert_from(decode(app_key_hex,'hex'),'UTF8') FROM stage_tickets;

-- TIPS: 000002 在 MySQL 用户导入前执行，历史 Agent/Bot 的 app_key 暂时为空。
-- 必须先证明每条历史记录都只有一个候选 AppKey，再在本事务内回填；无法映射或跨应用
-- 冲突时直接回滚全部业务数据，禁止让无租户归属的 Agent/Bot 进入新版本运行态。
CREATE TEMP TABLE stage_agent_app_keys ON COMMIT DROP AS
SELECT a.id, MIN(u.app_key) AS app_key, COUNT(DISTINCT u.app_key) AS candidate_count
FROM agents a
LEFT JOIN users u ON u.user_id = a.owner_id
WHERE a.app_key = ''
GROUP BY a.id;

DO $$
DECLARE invalid_count BIGINT;
BEGIN
	SELECT COUNT(*) INTO invalid_count FROM stage_agent_app_keys WHERE candidate_count <> 1;
	IF invalid_count > 0 THEN
		RAISE EXCEPTION '存在 % 个历史 Agent 无法唯一映射 AppKey，迁移已回滚', invalid_count;
	END IF;
END $$;

UPDATE agents a
SET app_key = mapped.app_key, updated_at = now()
FROM stage_agent_app_keys mapped
WHERE a.id = mapped.id;

CREATE TEMP TABLE stage_bot_app_keys ON COMMIT DROP AS
SELECT b.id, MIN(candidate.app_key) AS app_key, COUNT(DISTINCT candidate.app_key) AS candidate_count
FROM bots b
LEFT JOIN (
	SELECT binding.bot_id, a.app_key
	FROM bot_agent_bindings binding
	JOIN agents a ON a.id = binding.agent_id
	WHERE a.app_key <> ''
	UNION
	SELECT owned.id AS bot_id, u.app_key
	FROM bots owned
	JOIN users u ON u.user_id = owned.owner_id
) candidate ON candidate.bot_id = b.id
WHERE b.app_key = ''
GROUP BY b.id;

DO $$
DECLARE invalid_count BIGINT;
BEGIN
	SELECT COUNT(*) INTO invalid_count FROM stage_bot_app_keys WHERE candidate_count <> 1;
	IF invalid_count > 0 THEN
		RAISE EXCEPTION '存在 % 个历史 Bot 无法唯一映射 AppKey，迁移已回滚', invalid_count;
	END IF;
END $$;

UPDATE bots b
SET app_key = mapped.app_key, updated_at = now()
FROM stage_bot_app_keys mapped
WHERE b.id = mapped.id;

SELECT setval(pg_get_serial_sequence('apps','id'), COALESCE(MAX(id),1), MAX(id) IS NOT NULL) FROM apps;
SELECT setval(pg_get_serial_sequence('appexts','id'), COALESCE(MAX(id),1), MAX(id) IS NOT NULL) FROM appexts;
SELECT setval(pg_get_serial_sequence('users','id'), COALESCE(MAX(id),1), MAX(id) IS NOT NULL) FROM users;
SELECT setval(pg_get_serial_sequence('customers','id'), COALESCE(MAX(id),1), MAX(id) IS NOT NULL) FROM customers;
SELECT setval(pg_get_serial_sequence('inboxes','id'), COALESCE(MAX(id),1), MAX(id) IS NOT NULL) FROM inboxes;
SELECT setval(pg_get_serial_sequence('customerinboxrels','id'), COALESCE(MAX(id),1), MAX(id) IS NOT NULL) FROM customerinboxrels;
SELECT setval(pg_get_serial_sequence('inboxmembers','id'), COALESCE(MAX(id),1), MAX(id) IS NOT NULL) FROM inboxmembers;
SELECT setval(pg_get_serial_sequence('tickets','id'), COALESCE(MAX(id),1), MAX(id) IS NOT NULL) FROM tickets;

COMMIT;
