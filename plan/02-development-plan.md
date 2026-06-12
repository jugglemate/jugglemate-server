# 开发计划：JuggleChat App Server Agent 对接

生成日期：2026-06-01  
基于：`api.md` + 当前 App Server 代码  
交付范围：MVP

## 一、模块全景图

```text
Layer 0 基础设施
  [MOD-001] 本地 Agent 域数据模型与迁移
  [MOD-002] Agent Server HTTP 适配层

Layer 1 核心框架
  [MOD-003] Owner 分身与素材管理服务
  [MOD-004] Owner 训练 / 版本 / 任务 / 评估 / 会话服务

Layer 2 业务模块
  [MOD-005] Customer 消息回调与 JuggleIM 回复链路

Layer 3 集成验证
  [MOD-006] 配置、路由、测试与联调收口
```

## 二、模块详情

### [MOD-001] 本地 Agent 域数据模型与迁移

| 属性 | 内容 |
|---|---|
| 所属层 | Layer 0 |
| 描述 | 建立 App Server 本地的 Agent 域数据底座，承载 twin、material、version、job、evaluation、message、sync 状态和 tombstone |
| 依赖模块 | 无 |
| 解锁模块 | MOD-003, MOD-004, MOD-005, MOD-006 |
| 接口契约 | 本地表结构、DAO、Domain Model、同步状态字段 |
| Done Definition | 迁移可在空库与已有库上执行；核心实体可落库、查询、分页；unique_name tombstone 可阻止复用 |

主要工作内容：

- 设计并新增本地表结构
- 把 Agent Server 的资源模型映射到 App Server domain model
- 增加同步状态、同步错误、最后同步时间等字段
- 给现有 `aibots` 兼容逻辑预留迁移入口

风险/注意事项：

- 当前 `aibots` 表只够存 bot 基础信息，不足以承载完整 Agent 域
- `unique_name` 的不可复用规则必须在库层保证

### [MOD-002] Agent Server HTTP 适配层

| 属性 | 内容 |
|---|---|
| 所属层 | Layer 0 |
| 描述 | 把 `api.md` 所有 Agent Server 接口封装成 App Server 可调用的 client 方法 |
| 依赖模块 | 无 |
| 解锁模块 | MOD-003, MOD-004, MOD-005 |
| 接口契约 | Owner API、Customer chat API、错误映射、请求头透传 |
| Done Definition | 所有目标接口有统一 client 封装；错误可解析；Owner/Customer 请求头可按规则注入；本地测试可覆盖主要成功和失败路径 |

主要工作内容：

- 封装 `/v1/twins/*`、`/v1/jobs/*`、`/v1/twins/{unique_name}/chat`
- 统一 owner/customer 身份头与超时配置
- 统一错误码、HTTP 状态码和内部错误映射
- 为 chat JSON 响应预留 SSE 解析能力

风险/注意事项：

- Agent Server 当前是外部契约，返回字段需要按文档严格对齐
- customer 侧业务异常不能直接穿透给终端用户

### [MOD-003] Owner 分身与素材管理服务

| 属性 | 内容 |
|---|---|
| 所属层 | Layer 1 |
| 描述 | 实现分身和素材的本地写入、Agent Server 同步和对外管理接口 |
| 依赖模块 | MOD-001, MOD-002 |
| 解锁模块 | MOD-006 |
| 接口契约 | 创建 / 查询 / 更新 / 删除分身，素材增删查，local-first 写入规则 |
| Done Definition | 分身与素材 CRUD 可跑通；本地写成功后能同步到 Agent Server；失败时本地状态可追踪且可重试 |

主要工作内容：

- 复用或替换当前 `aibot` 管理接口
- 增加 `unique_name`、`display_name`、`avatar_url`、`greeting` 的标准化处理
- 实现素材文本、URL、文件上传的本地存储与同步
- 维护本地和远端的一致性投影

风险/注意事项：

- 现有 `bot_id` 语义和 `unique_name` 不完全等价，需要在迁移时统一

### [MOD-004] Owner 训练 / 版本 / 任务 / 评估服务

| 属性 | 内容 |
|---|---|
| 所属层 | Layer 1 |
| 描述 | 实现训练、评估、任务轮询、版本切换的完整 Owner 流程 |
| 依赖模块 | MOD-001, MOD-002 |
| 解锁模块 | MOD-006 |
| 接口契约 | `/training`、`/evaluations`、`/jobs/{job_id}`、`/versions/*` |
| Done Definition | 训练 / 评估任务可发起、可轮询、可落本地状态；版本激活和当前版本查询能从本地读出稳定结果 |

主要工作内容：

- 封装异步任务创建、轮询、取消
- 落版本快照和激活态
- 提供任务和版本状态查询

风险/注意事项：

- 训练和评估是异步任务，必须处理 queued / running / succeeded / failed / canceled

### [MOD-005] Customer 消息回调与 JuggleIM 回复链路

| 属性 | 内容 |
|---|---|
| 所属层 | Layer 2 |
| 描述 | 在 `/msgcallback` 中接住客户消息，调用 Agent Server chat，再通过 JuggleIM SDK 把回复发回去 |
| 依赖模块 | MOD-001, MOD-002 |
| 解锁模块 | MOD-006 |
| 接口契约 | callback payload 解析、chat 调用、fallback 回复、消息发送 |
| Done Definition | 客户消息能从 IM 回调进入 App Server，完成一次完整的对话回路；Agent 不可用时能给出安全兜底回复 |

主要工作内容：

- 解析 IM 回调消息，兼容 `event_type/timestamp/payload[]` callback body
- 识别 `unique_name`、`customer_id`、消息文本
- 先落本地会话和消息，再调用 `/v1/twins/{unique_name}/chat`
- 把 Agent Server 返回结果通过 JuggleIM SDK 发给发送者
- 保存回复消息、会话状态和来源渠道

风险/注意事项：

- 这是热路径，不能依赖同步慢操作阻塞消息发送

### [MOD-006] 配置、路由、测试与联调收口

| 属性 | 内容 |
|---|---|
| 所属层 | Layer 3 |
| 描述 | 把前面模块串起来，完成路由、配置、迁移、联调和最小集成验证 |
| 依赖模块 | MOD-001, MOD-002, MOD-003, MOD-004, MOD-005 |
| 解锁模块 | 无 |
| 接口契约 | 配置项、路由注册、测试覆盖、联调脚本 |
| Done Definition | 新配置生效；路由注册完整；核心 Owner / Customer 流程至少有一条集成测试或冒烟验证通过 |

主要工作内容：

- 增加 Agent Server base URL、timeout、retry、debug 配置
- 把 owner API 和 msgcallback 接口接到正确路由
- 增加冒烟测试和关键路径集成测试
- 输出联调说明

风险/注意事项：

- 如果配置项没收口，后续本地同步和 callback 会很难排查

## 三、依赖关系总览

```text
MOD-001 ──→ MOD-003 ──→ MOD-006
       └──→ MOD-004 ──┘
MOD-002 ──→ MOD-003
       └──→ MOD-004
       └──→ MOD-005 ──→ MOD-006
```

关键路径：

`MOD-001 -> MOD-002 -> MOD-005 -> MOD-006`

## 四、里程碑

| 里程碑 | 完成标志 | 解锁内容 |
|---|---|---|
| M0：契约确认 | `api.md` 的 Owner / Customer 调用边界和本地落盘策略确认 | 开始基础设施开发 |
| M1：底座就绪 | MOD-001、MOD-002 完成 | Owner / Customer 业务模块可并行开发 |
| M2：主流程完成 | MOD-003、MOD-004、MOD-005 完成 | 开始集成验证 |
| M3：联调通过 | MOD-006 完成 | 可以进入对外确认 |

## 五、风险与阻塞项

| # | 风险描述 | 影响模块 | 处理建议 |
|---|---|---|---|
| 1 | `unique_name` 的不可复用规则没有在数据库层落地 | MOD-001, MOD-003 | 必须增加 tombstone / 唯一索引设计 |
| 2 | 现有 `aibot` 语义与 Agent Server `twin` 不完全一致 | MOD-003, MOD-004 | 先做兼容映射，再逐步收敛字段 |
| 3 | `/msgcallback` 一次回调可能携带多条 `payload` 消息，且可能包含非文本消息 | MOD-005 | 逐条幂等处理，MVP 只响应 `event_type=message` 且 `msg_type=text` 的消息 |
| 4 | Agent Server 的同步失败没有重试策略 | MOD-003, MOD-004, MOD-005 | 用本地同步状态表兜底，补重试任务 |
