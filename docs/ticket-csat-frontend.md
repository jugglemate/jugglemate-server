# 前端接入手册：工单评价自定义消息

> 本文档面向前端工程师，描述 2026-07-29 ticket-auto-close change 上线的 `jgm:csatreply` 评分自定义消息如何在前端订阅、构造、发送。
>
> 与之配套的后端协议说明见 `docs/ticket-api.md` §"工单自动关闭 & 评价系统"。

## 一、消息流总览

```
[客户]
  ↓ IM 客户端订阅 jgm:csat 消息
[前端 widget / juggleim]
  ↓ 渲染评分卡（按钮 1-5）
  ↓ 用户点 5★ → 构造 jgm:csatreply 群消息
  ↓ imSdk.sendGroupMsg({
        msg_type: "jgm:csatreply",
        content: { rating, comment, ticket_id, app_key }
      })
[服务端 webhook /jmate/webhooks/message]
  ↓ 解析 msg_content
  ↓ 校验 app_key / ticket_id / sender / schema
  ↓ ticket_ratings 落库
[前端]
  ↓ GET /jmate/tickets/:ticket_id/rating → 看到已评，禁用卡片
```

## 二、前端需要做的两件事

### A. 订阅 `jgm:csat` 渲染邀请卡

监听 IM 消息流（Socket SDK / JuggleIM 客户端 SDK）：

```ts
imSdk.on('message', (msg) => {
  if (msg.msg_type === 'jgm:csat' && msg.content?.kind === 'csat') {
    renderCsatCard({
      ticketId: msg.content.ticket_id,
      options: msg.content.csat.options,        // [{value:1,label:"1 很差"}, ...]
      lowRatingThreshold: msg.content.csat.low_rating_threshold, // 3
      replyMsgType: msg.content.csat.reply_msg_type,             // "jgm:csatreply"
    });
  }
});
```

### B. 评分卡交互后构造 `jgm:csatreply` 发回群里

```ts
async function submitRating(ticketId: string, rating: number, comment?: string) {
  // 1. message_type 必须严格为 "jgm:csatreply"（低层 IM server 按 type 路由）
  // 2. content 必须 JSON 序列化
  const content = {
    kind: 'csatreply',         // 强校验：必须是 "csatreply"
    version: 1,
    app_key: CURRENT_APPKEY,  // 与登录态一致
    ticket_id: ticketId,
    rating,                    // 1..5
    comment: comment ?? '',    // ≤ 500 字符；前端可提前截断
    client_ts: Date.now(),     // 可选
  };

  await imSdk.sendGroupMsg({
    target_id: ticketId,                // ticket 群 ID
    msg_type: 'jgm:csatreply',          // 关键
    msg_content: JSON.stringify(content),
  });

  // 乐观更新：本地禁用评分卡
  setRated(true);
}
```

## 三、评分前端判断（避免重复评分）

客户点开工单详情时，先拉一次 GET API：

```ts
const data = await fetch(
  `/jmate/tickets/${ticketId}/rating?customer_id=${customerId}`,
  { headers: { appkey: APPKEY, Authorization: token } }
);
const json = await data.json();

if (json.data?.rating) {
  // 已评：隐藏评分卡，显示"已收到您的评价 N 星"
  hideCsatCard();
  showRatedHint(json.data.rating);
} else {
  // 未评：保持评分卡，可点击
  showCsatCard();
}
```

## 四、低分引导（rating ≤ 3）

服务端的 `jgm:csat` 消息已经携带：

```json
{
  "low_rating_threshold": 3,
  "low_rating_hint": "评分 ≤3 时，请附带文字说明问题，方便我们改进。",
  "reply_msg_type": "jgm:csatreply"
}
```

UI 渲染策略：

| 用户评分 | 前端行为 |
|---|---|
| 4 ★ 或 5 ★ | 直接调用 `submitRating(ticketId, rating)`，comment 留空 |
| 1 / 2 / 3 ★ | 弹出 textarea 提示 "想反馈点什么？"；必填后调 `submitRating(ticketId, rating, comment)` |

TIPS: 服务端对 `comment` 超 500 字符做**截断**而非拒收，所以前端可宽松允许输入；超长会被服务端悄无声息截断。

## 五、Telegram / WhatsApp 渠道的特殊情况

本 change 暂未对 Telegram / WhatsApp 适配：

| 渠道 | 客户端能否渲染 `jgm:csatreply`？ | 客户回什么？ |
|---|---|---|
| Web widget | ✅ JuggleIM 客户端原生支持 | 调 imSdk 发自定义消息 |
| JuggleIM（同源群） | ✅ | 同 widget |
| Telegram | ❌ Telegram Bot 不能解析自定义 type | 让客户**回群**说"5"，由 JuggleIM 直接透传 tg:text；服务端的 jgm:csatreply 入口**不会**匹配 — 评分走另开 change |
| WhatsApp | ❌ 未实现 | — |

如果需要支持 TG 评分，下一个 change 在 WebhookMsgs 增加 `tg:text` 评分文本解析（`^[1-5]$`），并手动 review 是否破坏现有逻辑。

## 六、错误与降级

后端 webhook 收到 `jgm:csatreply` 时如果校验失败，返回 `17005 ParamError`、UNIQUE 重复返回 SUCCESS（静默丢弃）。**前端不需要处理这些状态码**——所有失败都进同一个 IM 消息；前端只用本地乐观状态（评分按钮禁用）即可。

如果客户发的 jgm:csatreply 因为网络问题没送达 IM server（比如发了但 webhook 没收到），下次客户重新发一次即可（多次评分会被后端 UNIQUE 挡掉）。

## 七、联调检查清单

- [ ] 监听 `jgm:csat` 消息，渲染评分卡
- [ ] 点 1/2/3 星 → 弹反馈输入框（必填 comment）
- [ ] 点 4/5 星 → 直接提交
- [ ] 提交后**禁用按钮 + 显示已评状态**
- [ ] 打开工单时**先 GET** `/rating`，已评用户隐藏评分卡
- [ ] 不要调用 `POST /jmate/tickets/:id/rate` —— 该接口**不存在**，只能走 IM 消息
- [ ] `msg_type` 严格写 `jgm:csatreply`
- [ ] `content.app_key` 跟当前 app 一致（否则服务端拒收）
- [ ] `content.ticket_id` 跟当前 ticket_id 一致

## 八、API 接口速查

| 接口 | 用途 |
|---|---|
| `GET /jmate/tickets/:ticket_id/rating?customer_id=...` | 查"是否已评过"（只读） |
| IM `jgm:csat` 消息订阅 | 服务端发来的邀请卡（前端订阅） |
| IM `jgm:csatreply` 消息发送 | **唯一**评价入口（前端发往群里） |
