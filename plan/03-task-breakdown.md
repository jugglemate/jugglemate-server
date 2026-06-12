# 任务分解

以下任务按依赖顺序排列。它们不是按文件拆，而是按可交付功能切片拆。

## TASK-001 本地 Agent 域数据模型与迁移

| 属性 | 内容 |
|---|---|
| 类型 | [SCHEMA] |
| 所属模块 | MOD-001 |
| 依赖任务 | 无 |
| 解锁任务 | TASK-003, TASK-004, TASK-005 |

**目标**

建立 App Server 本地的 Agent 域数据底座，保证分身、素材、版本、任务、评估、消息和同步状态都能落盘。

**包含内容**

- 新增本地表结构和迁移 SQL
- 新增对应的 `storages/models` 领域模型
- 新增 `storages/dbs` DAO / 查询 / 分页
- 增加 `unique_name` tombstone 设计

**Done Definition**

- 迁移脚本可在空库执行
- 老库升级不破坏现有 `aibots` 数据
- 核心实体可创建、查询、分页、删除
- `unique_name` 不能被删除后重复复用

---

## TASK-002 Agent Server HTTP 适配层

| 属性 | 内容 |
|---|---|
| 类型 | [INFRA] |
| 所属模块 | MOD-002 |
| 依赖任务 | 无 |
| 解锁任务 | TASK-003, TASK-004, TASK-005 |

**目标**

把 `api.md` 的所有 Agent Server 接口封装成 App Server 可直接调用的 client 方法。

**包含内容**

- Owner 侧 client：twin、materials、training、evaluations、jobs、versions
- Customer 侧 client：chat JSON 请求
- 请求头注入：`X-Owner-Id`、`X-Customer-Id`、`X-Customer-Source`
- 错误映射和超时配置

**Done Definition**

- 所有目标接口都有显式方法
- 成功 / 失败 / 超时三类场景都能被上层识别
- chat 能返回 JSON reply，SSE 能力至少保留协议入口

---

## TASK-003 Owner 分身与素材管理

| 属性 | 内容 |
|---|---|
| 类型 | [FEATURE] |
| 所属模块 | MOD-003 |
| 依赖任务 | TASK-001, TASK-002 |
| 解锁任务 | TASK-006 |

**目标**

把当前 `aibot` 管理能力升级成 Agent Server 的 owner 侧分身 / 素材管理能力。

**包含内容**

- 创建 / 查询 / 更新 / 删除分身
- 文本、URL、上传素材管理
- 本地写入优先，再同步 Agent Server
- 兼容现有 `routers/router.go` 的 owner 路由

**Done Definition**

- 分身 CRUD 可走通
- 素材新增 / 列表 / 删除可走通
- 任一写操作先落本地，再发 Agent 同步
- 同步失败能在本地查到状态

---

## TASK-004 Owner 训练 / 版本 / 任务 / 评估管理

| 属性 | 内容 |
|---|---|
| 类型 | [FEATURE] |
| 所属模块 | MOD-004 |
| 依赖任务 | TASK-001, TASK-002 |
| 解锁任务 | TASK-006 |

**目标**

补齐训练、评估、任务轮询、版本切换和消息投影查询。

**包含内容**

- `training` / `evaluations` / `jobs` / `versions`
- `messages`
- 版本激活与当前版本读取
- 任务状态投影和评估报告落盘

**Done Definition**

- 异步任务状态可查询
- 版本激活后本地与远端视图一致
- 消息可以按 owner 维度查询

---

## TASK-005 Customer 消息回调与 JuggleIM 回复链路

| 属性 | 内容 |
|---|---|
| 类型 | [FEATURE] |
| 所属模块 | MOD-005 |
| 依赖任务 | TASK-001, TASK-002 |
| 解锁任务 | TASK-006 |

**目标**

把 `/msgcallback` 从“只打印请求体”改成完整的客户对话入口。

**包含内容**

- 解析 IM 回调消息
- 兼容 `event_type/timestamp/payload[]` callback body，至少能读出 `payload[].sender`、`payload[].receiver`、`payload[].msg_type`、`payload[].msg_content`、`payload[].msg_id`
- 识别 `unique_name`、`customer_id`、消息内容
- 调用 `/v1/twins/{unique_name}/chat`
- 通过 JuggleIM SDK 把 Agent 回复发回发送者
- 保存消息和回复状态

**Done Definition**

- 客户发一句话，App Server 能完成一次完整 round-trip
- 一次 callback 包含多条 `payload` 时能逐条处理，重复 `msg_id` 不重复回复
- Agent Server 不可用时，终端用户仍能收到安全兜底回复
- 本地消息能查到

---

## TASK-006 配置、路由、测试与联调收口

| 属性 | 内容 |
|---|---|
| 类型 | [CONFIG] / [TEST] |
| 所属模块 | MOD-006 |
| 依赖任务 | TASK-003, TASK-004, TASK-005 |
| 解锁任务 | 无 |

**目标**

把前面所有模块收口成可跑、可测、可联调的 App Server。

**包含内容**

- 新增 Agent Server base URL / timeout / retry 配置
- 调整路由注册
- 增加关键链路集成测试
- 补充联调说明和故障排查信息

**Done Definition**

- 启动后路由可用
- Owner / Customer 两条主链路至少各有一条冒烟验证通过
- 配置改动不需要改代码即可切换环境
