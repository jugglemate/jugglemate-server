# Agent 会话解绑与系统兜底移除设计文档

**日期:** 20260721
**复杂度:** Medium
**状态:** 已完成

## 背景与目标

修复 active Agent 列表额外注入系统 `Juggle_Agent` 导致的重复展示，并提供工单主动解除 Agent 绑定的接口。

## Tension Scan 结论

- T1（概念混用）：接口中的两个 `Juggle_Agent` 不是数据库重复行，而是应用 Agent 查询结果与系统兜底 Agent 注入结果叠加；在响应层按名称去重会掩盖租户隔离问题，因此删除兜底注入根因。
- T2（未检验假设）：删除“内置逻辑”不等于删除历史数据；本次只停止 Seed 和特殊读取/放行，不增加删除迁移，已有数据保持不动。

## 架构理解

- 核心模式：Gin Handler 负责契约和身份提取，Service 承担业务校验，GORM 访问 PostgreSQL。
- 相关模块：Agent API、Agent Profile Service、Agent 系统 Seed、工单-Agent 绑定测试与接口文档。
- 已识别设计模式：现有 Service 作为用例入口，`ticket_agent_bindings` 通过 `(app_key, ticket_id)` 唯一索引维护单一绑定。
- 假实现区域：未发现；本次直接读写真实 PostgreSQL 绑定表。
- 用户路径：active 列表只展示当前应用的 Agent；用户可在工单上绑定、换绑或清除 Agent。

## 方案决策

停止创建和前置系统兜底 Agent，同时移除系统 Agent 的跨租户权限与绑定特例。历史 `Juggle_Agent` 行不删除，后续只受普通 AppKey 规则约束。新增 `POST /agents/sessions/unbind`，按当前身份的 AppKey 与 `sessionId` 删除绑定；未绑定时幂等成功。

未采用响应层按 ID/名称去重，因为这会继续保留跨租户注入和错误的 `total`。未采用数据库迁移删除系统数据，因为需求明确要求保留已有数据。

### 引入/复用的设计模式

复用现有 Handler → Service 分层和唯一绑定模型，不引入新模式或中间层。

### 预期改动方向

- Agent 列表与绑定逻辑按 AppKey 严格隔离。
- Seed 保留系统钱包和系统工具，但不再创建系统 Agent。
- 新增会话解绑请求契约、路由、服务方法及回归测试。
- 更新前端对接文档。

### 副作用与风险

- 历史系统 Agent 仍保留在数据库中，但普通应用不再看到或绑定它。
- 依赖系统兜底 Agent 的旧调用方会失去该候选项，符合本次“无需内置系统级 Agent”的产品决策。
- 解绑只删除选择关系，不移出 IM 群成员，也不改变 Inbox 级自动回复路由。

## 改动范围

- `agent/migrations/seed.go`：停止 Seed 系统 `Juggle_Agent`，保留系统钱包和系统工具 Seed。
- `agent/modules/agent/service/service.go`、`profile.go`、`bot.go`、`capability.go`：移除内置 Agent 常量、权限特例、列表注入和跨 AppKey 绑定放行；新增原子解绑用例。
- `agent/modules/agent/dto/dto.go`、`apis/handler.go`：新增解绑请求 DTO 与 `POST /agents/sessions/unbind` 路由。
- Agent API、迁移、Bootstrap 与 PostgreSQL 集成测试：补充路由、历史系统 Agent 隔离、解绑幂等和租户隔离覆盖。
- `docs/agent-api.md`：更新 active/bind 契约并补充 unbind 对接说明。

## 实现声明

- PostgreSQL `ticket_agent_bindings`：✅ 真实接入，解绑使用 `DELETE ... RETURNING agent_id`。
- 控制台鉴权与 AppKey：✅ 复用真实身份提取和租户隔离。
- IM 群成员与 Inbox 路由：不适用；会话绑定按既有契约只记录选择关系。
- Mock / Fixture：生产实现无 Mock；测试数据仅存在于回滚事务内。

## Review 结论

- 正确性：历史系统 Agent 不再影响 active 列表 `items/total`；解绑、重复解绑和跨 AppKey 同名工单均符合预期。
- 架构契合度：复用现有 Handler → Service → GORM 分层，未增加空壳抽象。
- 安全：系统 Agent 不再绕过 AppKey 权限；解绑条件始终包含 AppKey。
- 性能：解绑为单条带唯一索引条件的 DELETE；active 列表未增加查询。
- 验证：`go test ./...` 全仓通过；需要 PostgreSQL/Redis 环境变量的基础设施测试在未配置时按项目约定跳过。
