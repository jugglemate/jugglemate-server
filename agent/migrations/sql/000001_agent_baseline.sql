-- Agent Server PostgreSQL 最终态基线
-- 来源提交: b52b0072d92a
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;


CREATE TABLE IF NOT EXISTS llm_providers (
	id UUID DEFAULT gen_random_uuid() NOT NULL,
	name VARCHAR(100) NOT NULL,
	protocol VARCHAR(50) NOT NULL,
	status VARCHAR(20) NOT NULL,
	api_key_encrypted TEXT NOT NULL,
	base_url VARCHAR(500),
	timeout_seconds INTEGER NOT NULL,
	retry_count INTEGER NOT NULL,
	custom_headers JSONB,
	proxy_url VARCHAR(500),
	description TEXT,
	created_by UUID,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_llm_providers_status CHECK (status IN ('draft', 'active', 'disabled', 'archived')),
	CONSTRAINT uq_llm_providers_name UNIQUE (name)
);
CREATE INDEX IF NOT EXISTS idx_providers_protocol ON llm_providers (protocol);
CREATE INDEX IF NOT EXISTS idx_providers_status ON llm_providers (status);


CREATE TABLE IF NOT EXISTS system_config (
	config_key VARCHAR(100) NOT NULL,
	config_value JSONB NOT NULL,
	config_category VARCHAR(50),
	description TEXT,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (config_key)
);
CREATE INDEX IF NOT EXISTS idx_system_config_category ON system_config (config_category);


CREATE TABLE IF NOT EXISTS agents (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	name VARCHAR(128) NOT NULL,
	type VARCHAR(32) NOT NULL,
	prompt TEXT,
	llm_model VARCHAR(100),
	llm_provider_id VARCHAR(64),
	status VARCHAR(20) NOT NULL,
	react_config JSONB NOT NULL,
	memory_config JSONB NOT NULL,
	total_invocations BIGINT NOT NULL,
	success_count BIGINT NOT NULL,
	fail_count BIGINT NOT NULL,
	failure_rate FLOAT NOT NULL,
	activated_at TIMESTAMP WITH TIME ZONE,
	paused_at TIMESTAMP WITH TIME ZONE,
	archived_at TIMESTAMP WITH TIME ZONE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_agents_status CHECK (status IN ('draft', 'active', 'paused', 'error', 'archived'))
);
CREATE INDEX IF NOT EXISTS idx_agents_llm_provider ON agents (llm_provider_id);
CREATE INDEX IF NOT EXISTS idx_agents_owner ON agents (owner_id);
CREATE INDEX IF NOT EXISTS idx_agents_owner_status ON agents (owner_id, status);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents (status);


CREATE TABLE IF NOT EXISTS tool_decision_records (
	id VARCHAR(64) NOT NULL,
	trace_id VARCHAR(64) NOT NULL,
	conversation_id VARCHAR(64),
	agent_id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	intent_summary TEXT NOT NULL,
	candidate_tools JSONB NOT NULL,
	selected_tools JSONB NOT NULL,
	decision_action VARCHAR(32) NOT NULL,
	termination_reason VARCHAR(128),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_tool_decision_records_action CHECK (decision_action IN ('call', 'no_call', 'ask_clarification', 'blocked'))
);
CREATE INDEX IF NOT EXISTS idx_tool_decision_trace ON tool_decision_records (trace_id);


CREATE TABLE IF NOT EXISTS tool_providers (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	provider_type VARCHAR(32) NOT NULL,
	name VARCHAR(128) NOT NULL,
	display_name VARCHAR(128),
	connection_config JSONB NOT NULL,
	auth_type VARCHAR(40) NOT NULL,
	auth_schema JSONB NOT NULL,
	status VARCHAR(20) NOT NULL,
	visibility VARCHAR(20) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	deleted_at TIMESTAMP WITH TIME ZONE,
	PRIMARY KEY (id),
	CONSTRAINT ck_tool_providers_status CHECK (status IN ('draft', 'testing', 'active', 'deprecated', 'deleted')),
	CONSTRAINT ck_tool_providers_type CHECK (provider_type IN ('plugin', 'custom')),
	CONSTRAINT ck_tool_providers_auth_type CHECK (auth_type IN ('none','api_key','bearer','basic','oauth2_client_credentials')),
	CONSTRAINT ck_tool_providers_visibility CHECK (visibility IN ('system', 'private'))
);
CREATE INDEX IF NOT EXISTS idx_tool_providers_auth_type_status ON tool_providers (auth_type, status);
CREATE INDEX IF NOT EXISTS idx_tool_providers_created_at ON tool_providers (created_at);
CREATE INDEX IF NOT EXISTS idx_tool_providers_deleted_at ON tool_providers (deleted_at);
CREATE INDEX IF NOT EXISTS idx_tool_providers_visibility_status ON tool_providers (visibility, status);


CREATE TABLE IF NOT EXISTS skills (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	name VARCHAR(128) NOT NULL,
	description TEXT,
	visibility VARCHAR(20) NOT NULL,
	status VARCHAR(20) NOT NULL,
	category VARCHAR(64),
	input_schema JSONB,
	output_schema JSONB,
	risk_level VARCHAR(20) NOT NULL,
	requires_approval BOOLEAN NOT NULL,
	call_count BIGINT NOT NULL,
	success_rate FLOAT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	deprecated_at TIMESTAMP WITH TIME ZONE,
	deleted_at TIMESTAMP WITH TIME ZONE,
	PRIMARY KEY (id),
	CONSTRAINT ck_skills_status CHECK (status IN ('draft', 'testing', 'active', 'deprecated', 'deleted')),
	CONSTRAINT ck_skills_visibility CHECK (visibility IN ('private', 'public')),
	CONSTRAINT ck_skills_risk_level CHECK (risk_level IN ('low', 'medium', 'high', 'critical'))
);
CREATE INDEX IF NOT EXISTS idx_skills_category ON skills (category);
CREATE INDEX IF NOT EXISTS idx_skills_owner_id ON skills (owner_id);
CREATE INDEX IF NOT EXISTS idx_skills_risk_level ON skills (risk_level);
CREATE INDEX IF NOT EXISTS idx_skills_status ON skills (status);
CREATE INDEX IF NOT EXISTS idx_skills_updated_at ON skills (updated_at);
CREATE INDEX IF NOT EXISTS idx_skills_visibility ON skills (visibility);


CREATE TABLE IF NOT EXISTS tools (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	name VARCHAR(128) NOT NULL,
	description TEXT,
	visibility VARCHAR(20) NOT NULL,
	status VARCHAR(20) NOT NULL,
	api_endpoint VARCHAR(512) NOT NULL,
	http_method VARCHAR(16) NOT NULL,
	input_schema JSONB,
	output_schema JSONB,
	risk_level VARCHAR(20) NOT NULL,
	requires_approval BOOLEAN NOT NULL,
	timeout_seconds BIGINT NOT NULL,
	call_count BIGINT NOT NULL,
	success_rate FLOAT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	deprecated_at TIMESTAMP WITH TIME ZONE,
	deleted_at TIMESTAMP WITH TIME ZONE,
	PRIMARY KEY (id),
	CONSTRAINT ck_tools_status CHECK (status IN ('draft', 'testing', 'active', 'deprecated', 'deleted')),
	CONSTRAINT ck_tools_visibility CHECK (visibility IN ('private', 'public')),
	CONSTRAINT ck_tools_risk_level CHECK (risk_level IN ('low', 'medium', 'high', 'critical'))
);
CREATE INDEX IF NOT EXISTS idx_tools_owner_id ON tools (owner_id);
CREATE INDEX IF NOT EXISTS idx_tools_risk_level ON tools (risk_level);
CREATE INDEX IF NOT EXISTS idx_tools_status ON tools (status);
CREATE INDEX IF NOT EXISTS idx_tools_updated_at ON tools (updated_at);
CREATE INDEX IF NOT EXISTS idx_tools_visibility ON tools (visibility);


CREATE TABLE IF NOT EXISTS knowledge (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	name VARCHAR(200) NOT NULL,
	type VARCHAR(32) NOT NULL,
	source_type VARCHAR(16) NOT NULL,
	source_url VARCHAR(2000),
	file_id VARCHAR(64),
	status VARCHAR(20) NOT NULL,
	chunk_size INTEGER NOT NULL,
	overlap_window INTEGER NOT NULL,
	document_count INTEGER NOT NULL,
	vector_count INTEGER NOT NULL,
	hit_rate FLOAT NOT NULL,
	last_calculated_at TIMESTAMP WITH TIME ZONE,
	warning_low_hit_rate BOOLEAN NOT NULL,
	last_error_code VARCHAR(64),
	last_error_message TEXT,
	current_task_id VARCHAR(100),
	failed_at TIMESTAMP WITH TIME ZONE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	deleted_at TIMESTAMP WITH TIME ZONE,
	PRIMARY KEY (id),
	CONSTRAINT ck_knowledge_status CHECK (status IN ('draft', 'pending', 'processing', 'ready', 'failed', 'archived')),
	CONSTRAINT ck_knowledge_type CHECK (type IN ('offline_document', 'web_crawler', 'api_sync')),
	CONSTRAINT ck_knowledge_source_type CHECK (source_type IN ('url', 'file'))
);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_id ON knowledge (owner_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_source_type ON knowledge (source_type);
CREATE INDEX IF NOT EXISTS idx_knowledge_status ON knowledge (status);
CREATE INDEX IF NOT EXISTS idx_knowledge_updated_at ON knowledge (updated_at);


CREATE TABLE IF NOT EXISTS knowledge_file (
	id VARCHAR(64) NOT NULL,
	knowledge_id VARCHAR(64) NOT NULL,
	original_name VARCHAR(200) NOT NULL,
	stored_path VARCHAR(500) NOT NULL,
	file_size BIGINT NOT NULL,
	mime_type VARCHAR(64) NOT NULL,
	content_hash VARCHAR(64),
	text_content TEXT,
	text_extracted_at TIMESTAMP WITH TIME ZONE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_file_content_hash ON knowledge_file (content_hash);
CREATE INDEX IF NOT EXISTS idx_knowledge_file_knowledge_id ON knowledge_file (knowledge_id);


CREATE TABLE IF NOT EXISTS knowledge_notifications (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	title VARCHAR(200) NOT NULL,
	content TEXT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_notifications_created_at ON knowledge_notifications (created_at);
CREATE INDEX IF NOT EXISTS idx_knowledge_notifications_owner_id ON knowledge_notifications (owner_id);


CREATE TABLE IF NOT EXISTS knowledge_retrieval_logs (
	id VARCHAR(64) NOT NULL,
	knowledge_id VARCHAR(64) NOT NULL,
	is_hit BOOLEAN NOT NULL,
	retrieved_at TIMESTAMP WITH TIME ZONE NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_knowledge_id ON knowledge_retrieval_logs (knowledge_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_retrieved_at ON knowledge_retrieval_logs (retrieved_at);


CREATE TABLE IF NOT EXISTS knowledge_stats (
	id VARCHAR(64) NOT NULL,
	knowledge_id VARCHAR(64) NOT NULL,
	hit_rate FLOAT NOT NULL,
	document_count INTEGER NOT NULL,
	vector_count INTEGER NOT NULL,
	calculated_at TIMESTAMP WITH TIME ZONE NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_stats_knowledge_id ON knowledge_stats (knowledge_id);


CREATE TABLE IF NOT EXISTS knowledge_task (
	id VARCHAR(64) NOT NULL,
	knowledge_id VARCHAR(64) NOT NULL,
	status VARCHAR(20) NOT NULL,
	stage VARCHAR(32) NOT NULL,
	progress FLOAT NOT NULL,
	retry_count INTEGER NOT NULL,
	error_code VARCHAR(64),
	error_message TEXT,
	started_at TIMESTAMP WITH TIME ZONE,
	completed_at TIMESTAMP WITH TIME ZONE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_knowledge_task_status CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
	CONSTRAINT ck_knowledge_task_progress CHECK (progress >= 0 AND progress <= 100)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_task_created_at ON knowledge_task (created_at);
CREATE INDEX IF NOT EXISTS idx_knowledge_task_knowledge_id ON knowledge_task (knowledge_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_task_status ON knowledge_task (status);


CREATE TABLE IF NOT EXISTS knowledge_vectors (
	id VARCHAR(64) NOT NULL,
	knowledge_id VARCHAR(64) NOT NULL,
	chunk_id VARCHAR(64),
	document_index INTEGER NOT NULL,
	chunk_index INTEGER NOT NULL,
	content_chunk TEXT NOT NULL,
	embedding vector(1536),
	source_position JSONB,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_vectors_chunk_id ON knowledge_vectors (chunk_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_vectors_document_index ON knowledge_vectors (document_index);
CREATE INDEX IF NOT EXISTS idx_knowledge_vectors_knowledge_id ON knowledge_vectors (knowledge_id);


CREATE TABLE IF NOT EXISTS bots (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	invite_code VARCHAR(64) NOT NULL,
	bot_user_id VARCHAR(64) NOT NULL,
	bot_name VARCHAR(128) NOT NULL,
	token TEXT NOT NULL,
	status VARCHAR(16) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_bots_status CHECK (status IN ('active', 'inactive'))
);
CREATE INDEX IF NOT EXISTS idx_bots_owner_status ON bots (owner_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bots_user_unique ON bots (bot_user_id);


CREATE TABLE IF NOT EXISTS conversations (
	id VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL,
	user_id VARCHAR(64) NOT NULL,
	invite_code VARCHAR(64),
	user_name VARCHAR(128),
	pic TEXT,
	summary_id VARCHAR(64),
	context_token_budget INTEGER,
	status VARCHAR(20) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_conversations_status CHECK (status IN ('active', 'closed', 'archived'))
);
CREATE INDEX IF NOT EXISTS idx_conversations_agent_user ON conversations (agent_id, user_id);


CREATE TABLE IF NOT EXISTS consumption_records (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL,
	provider_id VARCHAR(64) NOT NULL,
	model_id VARCHAR(64) NOT NULL,
	session_id VARCHAR(64) NOT NULL,
	event_type VARCHAR(20) NOT NULL,
	event_date DATE NOT NULL,
	input_tokens INTEGER NOT NULL,
	output_tokens INTEGER NOT NULL,
	total_tokens INTEGER NOT NULL,
	model_coefficient NUMERIC(10, 5),
	credits_consumed NUMERIC(12, 5) NOT NULL,
	status VARCHAR(16) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_consumption_records_event_type CHECK (event_type IN ('llm_call', 'tool_call', 'knowledge_search', 'embedding')),
	CONSTRAINT ck_consumption_records_status CHECK (status IN ('pending', 'settled'))
);
CREATE INDEX IF NOT EXISTS idx_cr_owner_agent_date ON consumption_records (owner_id, agent_id, event_date);
CREATE INDEX IF NOT EXISTS idx_cr_owner_date ON consumption_records (owner_id, event_date);
CREATE INDEX IF NOT EXISTS idx_cr_owner_session_status ON consumption_records (owner_id, session_id, status);


CREATE TABLE IF NOT EXISTS credit_wallets (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	balance NUMERIC(12, 5) NOT NULL,
	total_recharged NUMERIC(12, 5) NOT NULL,
	total_consumed NUMERIC(12, 5) NOT NULL,
	status VARCHAR(16) NOT NULL,
	version INTEGER NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_credit_wallets_status CHECK (status IN ('active', 'frozen')),
	CONSTRAINT ck_credit_wallets_version_positive CHECK (version > 0),
	CONSTRAINT ck_credit_wallets_balance_non_negative CHECK (balance >= 0),
	UNIQUE (owner_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cw_owner_unique ON credit_wallets (owner_id);


CREATE TABLE IF NOT EXISTS recharge_orders (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	credit_amount NUMERIC(12, 5) NOT NULL,
	expires_at TIMESTAMP WITH TIME ZONE,
	status VARCHAR(16) NOT NULL,
	confirmed_by VARCHAR(64),
	cancelled_by VARCHAR(64),
	reason VARCHAR(500),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_recharge_orders_status CHECK (status IN ('pending', 'completed', 'cancelled'))
);


CREATE TABLE IF NOT EXISTS user_subscriptions (
	id VARCHAR(64) NOT NULL,
	user_id VARCHAR(64) NOT NULL,
	plan_type VARCHAR(16) NOT NULL,
	status VARCHAR(16) NOT NULL,
	daily_quota_tokens INTEGER NOT NULL,
	period_start_at TIMESTAMP WITH TIME ZONE NOT NULL,
	period_end_at TIMESTAMP WITH TIME ZONE NOT NULL,
	replaced_by VARCHAR(64),
	created_by VARCHAR(64) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_user_subscriptions_plan_type CHECK (plan_type IN ('monthly', 'yearly')),
	CONSTRAINT ck_user_subscriptions_status CHECK (status IN ('pending', 'active', 'cancelled', 'expired')),
	CONSTRAINT ck_user_subscriptions_daily_quota_positive CHECK (daily_quota_tokens > 0)
);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_period ON user_subscriptions (user_id, period_start_at, period_end_at);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_status ON user_subscriptions (user_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_subscriptions_user_active ON user_subscriptions (user_id) WHERE status = 'active';


CREATE TABLE IF NOT EXISTS user_token_quotas (
	id VARCHAR(64) NOT NULL,
	user_id VARCHAR(64) NOT NULL,
	quota_date DATE NOT NULL,
	quota_type VARCHAR(32) NOT NULL,
	grant_tokens INTEGER NOT NULL,
	grant_source_status VARCHAR(32) NOT NULL,
	status VARCHAR(16) NOT NULL,
	reason VARCHAR(128),
	issued_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_user_token_quotas_type CHECK (quota_type IN ('free_daily', 'subscription_daily')),
	CONSTRAINT ck_user_token_quotas_source_status CHECK (grant_source_status IN ('gifted_free', 'paid_subscription')),
	CONSTRAINT ck_user_token_quotas_status CHECK (status IN ('issued', 'invalidated')),
	CONSTRAINT ck_user_token_quotas_grant_tokens_non_negative CHECK (grant_tokens >= 0)
);
CREATE INDEX IF NOT EXISTS idx_user_token_quotas_source ON user_token_quotas (grant_source_status);
CREATE INDEX IF NOT EXISTS idx_user_token_quotas_user_date ON user_token_quotas (user_id, quota_date);
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_token_quotas_user_date_type_issued ON user_token_quotas (user_id, quota_date, quota_type) WHERE status = 'issued';


CREATE TABLE IF NOT EXISTS admin_audit_logs (
	id VARCHAR(64) NOT NULL,
	admin_id VARCHAR(64) NOT NULL,
	operation VARCHAR(64) NOT NULL,
	target_type VARCHAR(32) NOT NULL,
	target_id VARCHAR(64),
	details JSONB NOT NULL,
	ip_address VARCHAR(64),
	user_agent TEXT,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_admin_id ON admin_audit_logs (admin_id);
CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_created_at ON admin_audit_logs (created_at);
CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_operation ON admin_audit_logs (operation);
CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_target ON admin_audit_logs (target_type, target_id);


CREATE TABLE IF NOT EXISTS billing_rules (
	id VARCHAR(64) NOT NULL,
	status VARCHAR(16) NOT NULL,
	version INTEGER NOT NULL,
	token_unit_price NUMERIC(10, 4) NOT NULL,
	tool_pricing JSONB NOT NULL,
	min_recharge_amount INTEGER NOT NULL,
	precision INTEGER NOT NULL,
	validity_years INTEGER,
	reason TEXT,
	updated_by VARCHAR(64),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_billing_rules_status CHECK (status IN ('active', 'draft')),
	CONSTRAINT ck_billing_rules_version_positive CHECK (version > 0),
	CONSTRAINT ck_billing_rules_token_price_positive CHECK (token_unit_price > 0),
	CONSTRAINT ck_billing_rules_precision_range CHECK (precision >= 0 AND precision <= 5)
);
CREATE INDEX IF NOT EXISTS idx_billing_rules_created_at ON billing_rules (created_at);
CREATE INDEX IF NOT EXISTS idx_billing_rules_status ON billing_rules (status);
CREATE INDEX IF NOT EXISTS idx_billing_rules_version ON billing_rules (version);
CREATE UNIQUE INDEX IF NOT EXISTS uq_billing_rules_active ON billing_rules (status) WHERE status = 'active';


CREATE TABLE IF NOT EXISTS config_change_logs (
	id VARCHAR(64) NOT NULL,
	config_type VARCHAR(32) NOT NULL,
	config_id VARCHAR(64) NOT NULL,
	old_value JSONB,
	new_value JSONB NOT NULL,
	changed_by VARCHAR(64) NOT NULL,
	reason TEXT,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_config_change_logs_type CHECK (config_type IN ('billing', 'model', 'policy'))
);
CREATE INDEX IF NOT EXISTS idx_config_change_logs_created_at ON config_change_logs (created_at);
CREATE INDEX IF NOT EXISTS idx_config_change_logs_type ON config_change_logs (config_type);


CREATE TABLE IF NOT EXISTS model_coefficients (
	id VARCHAR(64) NOT NULL,
	status VARCHAR(16) NOT NULL,
	version INTEGER NOT NULL,
	coefficients JSONB NOT NULL,
	updated_by VARCHAR(64),
	reason VARCHAR(500),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_model_coefficients_status CHECK (status IN ('active', 'draft')),
	CONSTRAINT ck_model_coefficients_version_positive CHECK (version > 0)
);
CREATE INDEX IF NOT EXISTS idx_model_coefficients_created_at ON model_coefficients (created_at);
CREATE INDEX IF NOT EXISTS idx_model_coefficients_status ON model_coefficients (status);
CREATE INDEX IF NOT EXISTS idx_model_coefficients_version ON model_coefficients (version);
CREATE UNIQUE INDEX IF NOT EXISTS uq_model_coefficients_active ON model_coefficients (status) WHERE status = 'active';


CREATE TABLE IF NOT EXISTS platform_stats_snapshots (
	id VARCHAR(64) NOT NULL,
	date DATE NOT NULL,
	total_calls BIGINT NOT NULL,
	total_revenue BIGINT NOT NULL,
	error_count BIGINT NOT NULL,
	error_rate FLOAT NOT NULL,
	active_agents INTEGER NOT NULL,
	active_owners INTEGER NOT NULL,
	active_users INTEGER NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_platform_stats_snapshots_date ON platform_stats_snapshots (date);


CREATE TABLE IF NOT EXISTS policy_rules (
	id VARCHAR(64) NOT NULL,
	status VARCHAR(16) NOT NULL,
	version INTEGER NOT NULL,
	high_risk_operations JSONB NOT NULL,
	transaction_limits JSONB NOT NULL,
	rate_limits JSONB NOT NULL,
	approval_flow VARCHAR(32) NOT NULL,
	updated_by VARCHAR(64),
	reason VARCHAR(500),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_policy_rules_status CHECK (status IN ('active', 'draft')),
	CONSTRAINT ck_policy_rules_version_positive CHECK (version > 0),
	CONSTRAINT ck_policy_rules_approval_flow CHECK (approval_flow IN ('user_confirm', 'owner_approve', 'platform_approve'))
);
CREATE INDEX IF NOT EXISTS idx_policy_rules_created_at ON policy_rules (created_at);
CREATE INDEX IF NOT EXISTS idx_policy_rules_status ON policy_rules (status);
CREATE INDEX IF NOT EXISTS idx_policy_rules_version ON policy_rules (version);
CREATE UNIQUE INDEX IF NOT EXISTS uq_policy_rules_active ON policy_rules (status) WHERE status = 'active';


CREATE TABLE IF NOT EXISTS conversation_summaries (
	id VARCHAR(64) NOT NULL,
	conversation_id VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL,
	user_id VARCHAR(64) NOT NULL,
	summary_text TEXT NOT NULL,
	covered_message_ids JSONB,
	covered_message_count INTEGER NOT NULL,
	token_count INTEGER NOT NULL,
	status VARCHAR(20) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_conversation_summaries_status CHECK (status IN ('pending', 'completed', 'failed'))
);
CREATE INDEX IF NOT EXISTS idx_conv_summaries_conv_id ON conversation_summaries (conversation_id);


CREATE TABLE IF NOT EXISTS long_term_memories (
	id VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL,
	user_id VARCHAR(64) NOT NULL,
	memory_type VARCHAR(20) NOT NULL,
	content TEXT NOT NULL,
	content_vector FLOAT[],
	importance VARCHAR(20) NOT NULL,
	source_conversation_id VARCHAR(64),
	status VARCHAR(20) NOT NULL,
	expires_at TIMESTAMP WITH TIME ZONE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_long_term_memories_type CHECK (memory_type IN ('preference', 'fact', 'context')),
	CONSTRAINT ck_long_term_memories_importance CHECK (importance IN ('high', 'medium', 'low')),
	CONSTRAINT ck_long_term_memories_status CHECK (status IN ('active', 'expired', 'superseded'))
);
CREATE INDEX IF NOT EXISTS idx_ltm_agent_user ON long_term_memories (agent_id, user_id);
CREATE INDEX IF NOT EXISTS idx_ltm_status ON long_term_memories (status);


CREATE TABLE IF NOT EXISTS deposit_orders (
	id VARCHAR(10) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	amount NUMERIC(12, 2) NOT NULL,
	currency VARCHAR(8) NOT NULL,
	status VARCHAR(16) NOT NULL,
	remark VARCHAR(500),
	callback_code VARCHAR(16),
	callback_message VARCHAR(500),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_deposit_orders_status CHECK (status IN ('pending', 'success', 'fail')),
	CONSTRAINT ck_deposit_orders_currency CHECK (currency IN ('U'))
);
CREATE INDEX IF NOT EXISTS ix_deposit_orders_owner_id ON deposit_orders (owner_id);


CREATE TABLE IF NOT EXISTS llm_models (
	id UUID DEFAULT gen_random_uuid() NOT NULL,
	model_id VARCHAR(100) NOT NULL,
	display_name VARCHAR(200) NOT NULL,
	provider_id UUID NOT NULL,
	status VARCHAR(20) NOT NULL,
	model_types VARCHAR(50)[] NOT NULL,
	tags VARCHAR(50)[],
	context_length INTEGER NOT NULL,
	max_output INTEGER NOT NULL,
	supports_streaming BOOLEAN NOT NULL,
	supports_function_calling BOOLEAN NOT NULL,
	supports_vision BOOLEAN NOT NULL,
	supports_tools BOOLEAN NOT NULL,
	input_price_per_1k NUMERIC(10, 6) NOT NULL,
	output_price_per_1k NUMERIC(10, 6) NOT NULL,
	cost_tier VARCHAR(20),
	api_model_name VARCHAR(200) NOT NULL,
	api_version VARCHAR(50),
	provider_config JSONB,
	total_calls BIGINT NOT NULL,
	total_input_tokens BIGINT NOT NULL,
	total_output_tokens BIGINT NOT NULL,
	total_cost NUMERIC(15, 2) NOT NULL,
	avg_response_time NUMERIC(10, 3),
	error_rate NUMERIC(5, 4),
	description TEXT,
	created_by UUID,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_llm_models_status CHECK (status IN ('draft', 'active', 'disabled', 'archived')),
	CONSTRAINT ck_llm_models_cost_tier CHECK (cost_tier IS NULL OR cost_tier IN ('high', 'medium', 'low')),
	CONSTRAINT uq_llm_models_provider_model UNIQUE (provider_id, model_id),
	FOREIGN KEY(provider_id) REFERENCES llm_providers (id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_models_provider ON llm_models (provider_id);
CREATE INDEX IF NOT EXISTS idx_models_status ON llm_models (status);
CREATE INDEX IF NOT EXISTS idx_models_tags ON llm_models USING gin (tags);
CREATE INDEX IF NOT EXISTS idx_models_types ON llm_models USING gin (model_types);


CREATE TABLE IF NOT EXISTS llm_model_calls (
	id UUID DEFAULT gen_random_uuid() NOT NULL,
	model_id VARCHAR(100) NOT NULL,
	provider_id UUID NOT NULL,
	agent_id UUID,
	conversation_id UUID,
	call_type VARCHAR(20) NOT NULL,
	request_time TIMESTAMP WITH TIME ZONE NOT NULL,
	response_time NUMERIC(10, 3),
	status VARCHAR(20) NOT NULL,
	input_tokens INTEGER,
	output_tokens INTEGER,
	total_tokens INTEGER,
	cost NUMERIC(10, 6),
	error_code VARCHAR(50),
	error_message TEXT,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_llm_model_calls_call_type CHECK (call_type IN ('reasoning', 'summary', 'embedding')),
	CONSTRAINT ck_llm_model_calls_status CHECK (status IN ('success', 'failed', 'timeout')),
	FOREIGN KEY(provider_id) REFERENCES llm_providers (id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_model_calls_agent ON llm_model_calls (agent_id);
CREATE INDEX IF NOT EXISTS idx_model_calls_conv_time_status ON llm_model_calls (conversation_id, request_time, status);
CREATE INDEX IF NOT EXISTS idx_model_calls_model ON llm_model_calls (model_id);
CREATE INDEX IF NOT EXISTS idx_model_calls_provider ON llm_model_calls (provider_id);
CREATE INDEX IF NOT EXISTS idx_model_calls_status ON llm_model_calls (status);
CREATE INDEX IF NOT EXISTS idx_model_calls_time ON llm_model_calls (request_time);
CREATE INDEX IF NOT EXISTS idx_model_calls_time_status ON llm_model_calls (request_time, status);


CREATE TABLE IF NOT EXISTS agent_knowledge (
	id VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL,
	knowledge_id VARCHAR(64) NOT NULL,
	mounted_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT uq_agent_knowledge_agent_knowledge UNIQUE (agent_id, knowledge_id),
	FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_agent_knowledge_agent_id ON agent_knowledge (agent_id);


CREATE TABLE IF NOT EXISTS agent_skills (
	id VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL,
	skill_id VARCHAR(64) NOT NULL,
	policy_guard_enabled BOOLEAN NOT NULL,
	mounted_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT uq_agent_skills_agent_skill UNIQUE (agent_id, skill_id),
	FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_agent_skills_agent_id ON agent_skills (agent_id);


CREATE TABLE IF NOT EXISTS agent_tools (
	id VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL,
	tool_id VARCHAR(64) NOT NULL,
	enabled BOOLEAN NOT NULL,
	runtime_overrides JSONB,
	credential_id VARCHAR(64),
	policy_guard_enabled BOOLEAN NOT NULL,
	approval_required BOOLEAN NOT NULL,
	mounted_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT uq_agent_tools_agent_tool UNIQUE (agent_id, tool_id),
	FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_agent_tools_agent_enabled ON agent_tools (agent_id, enabled);
CREATE INDEX IF NOT EXISTS idx_agent_tools_agent_id ON agent_tools (agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_tools_tool_enabled ON agent_tools (tool_id, enabled);


CREATE TABLE IF NOT EXISTS tool_credentials (
	id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	provider_id VARCHAR(64) NOT NULL,
	name VARCHAR(128) NOT NULL,
	credential_payload_encrypted TEXT NOT NULL,
	status VARCHAR(16) NOT NULL,
	rotated_at TIMESTAMP WITH TIME ZONE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT uq_tool_credentials_owner_provider_name UNIQUE (owner_id, provider_id, name),
	CONSTRAINT ck_tool_credentials_status CHECK (status IN ('active', 'revoked')),
	FOREIGN KEY(provider_id) REFERENCES tool_providers (id)
);
CREATE INDEX IF NOT EXISTS idx_tool_credentials_owner_provider_status ON tool_credentials (owner_id, provider_id, status);


CREATE TABLE IF NOT EXISTS tool_definitions (
	id VARCHAR(64) NOT NULL,
	provider_id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	tool_type VARCHAR(16) NOT NULL,
	name VARCHAR(128) NOT NULL,
	description TEXT,
	input_schema JSONB NOT NULL,
	output_schema JSONB NOT NULL,
	execution_config JSONB NOT NULL,
	risk_level VARCHAR(20) NOT NULL,
	approval_required BOOLEAN NOT NULL,
	timeout_seconds INTEGER NOT NULL,
	status VARCHAR(20) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	deleted_at TIMESTAMP WITH TIME ZONE,
	PRIMARY KEY (id),
	CONSTRAINT ck_tool_definitions_status CHECK (status IN ('draft', 'testing', 'active', 'deprecated', 'deleted')),
	CONSTRAINT ck_tool_definitions_type CHECK (tool_type IN ('plugin', 'custom')),
	CONSTRAINT ck_tool_definitions_risk_level CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
	CONSTRAINT ck_tool_definitions_timeout CHECK (timeout_seconds >= 1 AND timeout_seconds <= 300),
	FOREIGN KEY(provider_id) REFERENCES tool_providers (id)
);
CREATE INDEX IF NOT EXISTS idx_tool_definitions_created_at ON tool_definitions (created_at);
CREATE INDEX IF NOT EXISTS idx_tool_definitions_deleted_at ON tool_definitions (deleted_at);
CREATE INDEX IF NOT EXISTS idx_tool_definitions_provider_status ON tool_definitions (provider_id, status);
CREATE INDEX IF NOT EXISTS idx_tool_definitions_risk_status ON tool_definitions (risk_level, status);


CREATE TABLE IF NOT EXISTS skill_tools (
	id VARCHAR(64) NOT NULL,
	skill_id VARCHAR(64) NOT NULL,
	tool_id VARCHAR(64) NOT NULL,
	order_index INTEGER NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT uq_skill_tools_skill_tool UNIQUE (skill_id, tool_id),
	FOREIGN KEY(skill_id) REFERENCES skills (id) ON DELETE CASCADE,
	FOREIGN KEY(tool_id) REFERENCES tools (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_skill_tools_skill_id ON skill_tools (skill_id);
CREATE INDEX IF NOT EXISTS idx_skill_tools_tool_id ON skill_tools (tool_id);


CREATE TABLE IF NOT EXISTS bot_agent_bindings (
	id VARCHAR(64) NOT NULL,
	bot_id VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL,
	status VARCHAR(16) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_bot_agent_bindings_status CHECK (status IN ('active', 'inactive')),
	FOREIGN KEY(bot_id) REFERENCES bots (id) ON DELETE CASCADE,
	FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_bab_agent_status ON bot_agent_bindings (agent_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bab_bot_unique ON bot_agent_bindings (bot_id);


CREATE TABLE IF NOT EXISTS messages (
	id VARCHAR(64) NOT NULL,
	conversation_id VARCHAR(64) NOT NULL,
	user_id VARCHAR(64),
	message_id VARCHAR(128),
	role VARCHAR(16) NOT NULL,
	content TEXT NOT NULL,
	is_summarized BOOLEAN NOT NULL,
	self_token_count INTEGER NOT NULL,
	source VARCHAR(16) DEFAULT 'auto' NOT NULL,
	operator_id VARCHAR(64),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_messages_role CHECK (role IN ('user', 'assistant', 'system', 'tool')),
	CONSTRAINT ck_messages_source CHECK (source IN ('auto', 'human', 'webhook')),
	FOREIGN KEY(conversation_id) REFERENCES conversations (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_created_at ON messages (conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_source_created_at ON messages (source, created_at);


CREATE TABLE IF NOT EXISTS react_sessions (
	id VARCHAR(64) NOT NULL,
	conversation_id VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL,
	user_id VARCHAR(64) NOT NULL,
	status VARCHAR(20) NOT NULL,
	current_round INTEGER NOT NULL,
	max_rounds INTEGER NOT NULL,
	target_rounds INTEGER NOT NULL,
	confidence_score FLOAT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	completed_at TIMESTAMP WITH TIME ZONE,
	error TEXT,
	PRIMARY KEY (id),
	CONSTRAINT ck_react_sessions_status CHECK (status IN ('initialized', 'reasoning', 'acting', 'evaluating', 'completed', 'stopped', 'failed')),
	FOREIGN KEY(conversation_id) REFERENCES conversations (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_react_sessions_conv ON react_sessions (conversation_id);


CREATE TABLE IF NOT EXISTS tool_call_records (
	id VARCHAR(64) NOT NULL,
	trace_id VARCHAR(64) NOT NULL,
	conversation_id VARCHAR(64),
	agent_id VARCHAR(64) NOT NULL,
	owner_id VARCHAR(64) NOT NULL,
	tool_definition_id VARCHAR(64) NOT NULL,
	status VARCHAR(16) NOT NULL,
	input_summary TEXT,
	output_summary TEXT,
	error_code VARCHAR(64),
	error_message TEXT,
	duration_ms INTEGER NOT NULL,
	billed_credit NUMERIC(20, 6),
	idempotency_key VARCHAR(128),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_tool_call_records_status CHECK (status IN ('success', 'failed', 'timeout', 'blocked')),
	FOREIGN KEY(tool_definition_id) REFERENCES tool_definitions (id)
);
CREATE INDEX IF NOT EXISTS idx_tool_call_records_error ON tool_call_records (error_code);
CREATE INDEX IF NOT EXISTS idx_tool_call_records_trace ON tool_call_records (trace_id);


CREATE TABLE IF NOT EXISTS react_rounds (
	id VARCHAR(64) NOT NULL,
	session_id VARCHAR(64) NOT NULL,
	round_number INTEGER NOT NULL,
	thought TEXT,
	tool_calls JSONB,
	observation TEXT,
	observation_summary VARCHAR(500),
	confidence_score FLOAT,
	decision VARCHAR(16),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	PRIMARY KEY (id),
	CONSTRAINT ck_react_rounds_decision CHECK (decision IS NULL OR decision IN ('continue', 'stop')),
	FOREIGN KEY(session_id) REFERENCES react_sessions (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_react_rounds_session ON react_rounds (session_id);

-- 以下索引只存在于 Alembic 最终迁移中，SQLAlchemy model metadata 未完整声明，
-- 因此在最终态基线中显式补齐。
CREATE INDEX IF NOT EXISTS idx_do_owner_id ON deposit_orders (owner_id);
CREATE INDEX IF NOT EXISTS idx_do_status ON deposit_orders (status);
CREATE INDEX IF NOT EXISTS idx_do_created_at ON deposit_orders (created_at);
CREATE INDEX IF NOT EXISTS idx_ro_owner_id ON recharge_orders (owner_id);
CREATE INDEX IF NOT EXISTS idx_ro_status ON recharge_orders (status);
CREATE INDEX IF NOT EXISTS idx_ro_created_at ON recharge_orders (created_at);
CREATE INDEX IF NOT EXISTS idx_tool_providers_owner_status_updated ON tool_providers (owner_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_tool_definitions_owner_status ON tool_definitions (owner_id, status);
CREATE INDEX IF NOT EXISTS idx_tool_decision_agent_time ON tool_decision_records (agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_tool_decision_owner_time ON tool_decision_records (owner_id, created_at);
CREATE INDEX IF NOT EXISTS idx_tool_decision_action_time ON tool_decision_records (decision_action, created_at);
CREATE INDEX IF NOT EXISTS idx_tool_call_records_conv_time ON tool_call_records (conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_tool_call_records_owner_time ON tool_call_records (owner_id, created_at);
CREATE INDEX IF NOT EXISTS idx_tool_call_records_tool_time_status ON tool_call_records (tool_definition_id, created_at, status);

ALTER TABLE conversations ADD FOREIGN KEY(summary_id) REFERENCES conversation_summaries (id) ON DELETE SET NULL;

ALTER TABLE conversation_summaries ADD FOREIGN KEY(conversation_id) REFERENCES conversations (id) ON DELETE CASCADE;

-- 当前 JMate 旧 Twin API 的进程内兼容状态。源 agent-server 不包含这些表，
-- 但当前项目仍需稳定映射 unique_name、训练任务与版本到新 Agent 聚合。
CREATE TABLE IF NOT EXISTS twin_compat_mappings (
	id VARCHAR(64) PRIMARY KEY,
	owner_id VARCHAR(64) NOT NULL,
	unique_name VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	knowledge_id VARCHAR(64) NOT NULL REFERENCES knowledge(id) ON DELETE CASCADE,
	display_name VARCHAR(128) NOT NULL,
	avatar_url TEXT NOT NULL DEFAULT '',
	greeting TEXT NOT NULL DEFAULT '',
	status VARCHAR(20) NOT NULL DEFAULT 'untrained',
	active_version VARCHAR(32) NOT NULL DEFAULT '',
	training_mode VARCHAR(20) NOT NULL DEFAULT '',
	materials_count INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT uq_twin_compat_owner_name UNIQUE(owner_id, unique_name),
	CONSTRAINT uq_twin_compat_unique_name UNIQUE(unique_name)
);
CREATE INDEX IF NOT EXISTS idx_twin_compat_owner ON twin_compat_mappings(owner_id);

CREATE TABLE IF NOT EXISTS twin_compat_materials (
	id VARCHAR(64) PRIMARY KEY,
	mapping_id VARCHAR(64) NOT NULL REFERENCES twin_compat_mappings(id) ON DELETE CASCADE,
	material_type VARCHAR(20) NOT NULL,
	title VARCHAR(200) NOT NULL DEFAULT '',
	source TEXT NOT NULL DEFAULT '',
	size_bytes BIGINT NOT NULL DEFAULT 0,
	content TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_twin_compat_material_mapping ON twin_compat_materials(mapping_id, created_at);

CREATE TABLE IF NOT EXISTS twin_compat_jobs (
	id VARCHAR(64) PRIMARY KEY,
	mapping_id VARCHAR(64) NOT NULL REFERENCES twin_compat_mappings(id) ON DELETE CASCADE,
	job_type VARCHAR(20) NOT NULL,
	status VARCHAR(20) NOT NULL,
	progress INTEGER,
	result JSONB NOT NULL DEFAULT '{}'::jsonb,
	error JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	started_at TIMESTAMPTZ,
	finished_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_twin_compat_job_mapping ON twin_compat_jobs(mapping_id, created_at);

CREATE TABLE IF NOT EXISTS twin_compat_versions (
	id VARCHAR(64) PRIMARY KEY,
	mapping_id VARCHAR(64) NOT NULL REFERENCES twin_compat_mappings(id) ON DELETE CASCADE,
	version VARCHAR(32) NOT NULL,
	mode VARCHAR(20) NOT NULL,
	active BOOLEAN NOT NULL DEFAULT false,
	training_job_id VARCHAR(64) NOT NULL,
	materials_count INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT uq_twin_compat_mapping_version UNIQUE(mapping_id, version)
);
CREATE INDEX IF NOT EXISTS idx_twin_compat_version_mapping ON twin_compat_versions(mapping_id, created_at);

CREATE TABLE IF NOT EXISTS twin_compat_evaluations (
	id VARCHAR(64) PRIMARY KEY,
	mapping_id VARCHAR(64) NOT NULL REFERENCES twin_compat_mappings(id) ON DELETE CASCADE,
	version VARCHAR(32) NOT NULL,
	overall_score DOUBLE PRECISION NOT NULL,
	dimensions JSONB NOT NULL DEFAULT '[]'::jsonb,
	summary_md TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_twin_compat_evaluation_mapping ON twin_compat_evaluations(mapping_id, created_at);
