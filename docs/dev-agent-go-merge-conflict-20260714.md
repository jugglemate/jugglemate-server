# dev 合并 Go Agent 冲突修复文档

**日期：** 2026-07-14
**严重程度：** 🔴 阻塞
**状态：** 已修复

## 问题描述

`feature/agent_go` 合并到 `dev` 时，`go.mod` 与 `go.sum` 发生冲突。直接保留 `dev` 的依赖配置会引用不存在的本机目录，直接保留 feature 配置则会导致工单全局会话标签代码无法编译。

## 根因分析

- L2 直接原因：`dev` 使用 `replace github.com/juggleim/imserver-sdk-go => ../../gitee/imserver-sdk-go`，并调用本地 SDK 才有的 `GlobalConverTagsReq` 和 `SetGlobalConverTags`。
- L3 根因：全局会话标签功能依赖了未发布、未纳入仓库的本地 SDK 修改，依赖声明无法在其他开发机和 CI 环境复现。
- L4 系统性：
  - 代码层面：属于外部依赖版本与业务代码契约不一致的问题。
  - 设计层面：业务服务不应通过 `replace` 隐式依赖开发者机器上的源码目录。
  - 演化层面：全局会话标签 Server API 已进入 IM Server，但对应 Go SDK 封装尚未发布。

## 推理过程（关键节点）

→ 初始冲突只出现在 `go.mod` 和 `go.sum`。

→ 合并依赖集合并执行 `go mod tidy` 后，发现本机 `replace` 目录不存在。

→ 移除 `replace` 后，全量测试暴露公开 SDK 缺少全局标签符号。

→ 核对官方 SDK `v1.0.15` 与本机 IM Server 提交 `e55e896`，确认真实接口为 `POST /apigateway/convers/globaltags/set`，SDK 已公开的 `HttpCall` 可以复用统一签名和错误解析。

## 修复方案

**选择：** 正确修复

**理由：** 移除不可移植的本地模块替换，在工单标签服务内定义最小请求契约，并通过公开 SDK 的 `HttpCall` 调用真实 Server API。该方案保留 dev 功能，同时保证依赖可在 CI 和其他开发机复现。

**改动范围：**

- 合并并整理 `go.mod`、`go.sum` 的依赖集合。
- 移除 `../../gitee/imserver-sdk-go` 本机路径替换。
- 调整 `services/ticketglobaltagservice.go` 的全局标签调用适配。
- 增加真实 HTTP 契约测试。

## 实现说明

全局标签请求继续使用 `conver_id`、`channel_type`、`sub_channel`、`global_conver_tags` 字段。SDK 负责 AppKey、nonce、timestamp、signature 等鉴权头，业务层只维护当前尚未发布的接口路径和请求结构。

## 实现声明

| 依赖点 | 实现状态 | 说明 |
| --- | --- | --- |
| IM Server API | ✅ 真实接入 | 通过官方 SDK `HttpCall` 调用 `/apigateway/convers/globaltags/set` |
| 请求鉴权 | ✅ 真实验证 | 复用 SDK 统一签名头生成逻辑 |
| 数据库读写 | ✅ 真实接入 | 继续从工单存储读取最新状态 |

## 回归风险与测试建议

- 回归风险：后续 SDK 正式发布 `SetGlobalConverTags` 后，当前兼容层会产生重复封装，但接口契约不冲突。
- 已补充测试：校验 HTTP 方法、请求路径、JSON 字段和成功响应解析。
- 已执行：`go test ./...`、`go vet ./...`、`git diff --check`。

## Review 结论

- 未保留冲突标记。
- dev 的工单/收件箱功能与 feature 的 Go Agent 功能均保留。
- 未继续依赖本机目录或测试替身，合并结果可复现构建。
