-- Agent 软删除：新增 deleted 状态。
--
-- 为什么是软删而不是物理删除：conversations、consumption_records（计费）、llm_model_calls、
-- long_term_memories 等表都带 agent_id 但没有外键，物理删除 agents 行会把它们变成指向不存在
-- Agent 的孤儿数据，计费与对话历史也随之不可追溯。因此删除只置状态，由查询侧统一排除。
ALTER TABLE agents DROP CONSTRAINT IF EXISTS ck_agents_status;
ALTER TABLE agents ADD CONSTRAINT ck_agents_status
	CHECK (status IN ('draft', 'active', 'paused', 'error', 'archived', 'deleted'));
