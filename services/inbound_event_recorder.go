package services

import (
	"context"
	"log"
	"strings"

	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

// UnreliableOp 表达"非关键路径，操作结果只用于 slog / 监控；调用方应继续主流程"。
//
// 设计意图：recordOutboundTicketMessage 这条 webhook 钩子是 best-effort ——
// 留痕失败不能让 webhook 主流程报错（IM server 会重投导致工单状态更乱）。
// 但写代码时人脑要能直接看出"这个调用是吞错的"；用 UnreliableOp 包起来强迫
// 调用方主动写 Discard()，把语义刻在类型里。
type UnreliableOp struct {
	err     error
	source  string
	msgId   string
	desc    string
}

func (u UnreliableOp) Discard() {
	if u.err == nil {
		return
	}
	log.Printf("[WebhookMsgRecorder] %s 失败 msg_id=%s err=%v（继续）", u.desc, u.msgId, u.err)
}

func (u UnreliableOp) Err() error { return u.err }

// ClassifySenderRole 推断 webhook 入站消息作者角色。
//
// 判定规则（按优先级）：
//   - sender == ticket.SourceId  → customer（客户本人）
//   - sender == ticket.AssigneeId → user（坐席）
//   - 其他                          → bot（兜底：含 Agent Bot / 未知身份）
//
// TIPS: "bot" 兜底避免污染"坐席活跃"指标 —— bot 自身 / 系统消息不该推进
// last_user_msg_at。
func ClassifySenderRole(ticket *storageModels.Ticket, sender string) storageModels.TicketEventOperator {
	if ticket == nil {
		return storageModels.TicketEventOperatorSystem
	}
	sender = strings.TrimSpace(sender)
	switch {
	case sender != "" && ticket.SourceId == sender:
		return storageModels.TicketEventOperatorCustomer
	case sender != "" && ticket.AssigneeId == sender:
		return storageModels.TicketEventOperatorUser
	default:
		return storageModels.TicketEventOperatorBot
	}
}

// InboundEventRecorder 把 webhook 收到的群消息做两件事：
//
//   1. ticket_messages 留痕（UpsertByMsgId 幂等）：
//      每条 webhook 入站消息按 (app_key, msg_id) 唯一写入；同一条 IM 重投写同一行，
//      sender_role / msg_type / created_time 三列 ON CONFLICT 更新。
//   2. 若 sender 是该工单的 assignee（坐席），推进 last_user_msg_at（GREATEST 防回退）。
//
// 设计要点：
//   - 调用方通过 NewInboundEventRecorder 注入 tickets / messages storage，
//     以及 updateLast 函数（开发期替换为 fake、测试时 mock）。
//   - RecordOnce 返回 UnreliableOp：调用方写 Discard() 一次性 slog 完事。
//   - ticket==nil / msg_id 空 / time==0 都是合法 early-return，不报错。
//   - 不写"幽灵消息"：找不到 ticket 不写 ticket_messages，与 webhook 主流程
//     "工单不存在 → 静默丢" 保持一致。
type InboundEventRecorder struct {
	tickets   storageModels.ITicketStorage
	messages  storageModels.ITicketMessageStorage
	advanceOp func(appkey, ticketId string, atMs int64) error
}

// NewInboundEventRecorder 构造器。
//
// updateLast 必传 nil-安全：可传 func() 永远返回 nil 的空实现。
// 默认实现 = storages.NewTicketStorage().UpdateLastUserMsgAt。
func NewInboundEventRecorder(
	tickets storageModels.ITicketStorage,
	messages storageModels.ITicketMessageStorage,
	updateLast func(appkey, ticketId string, atMs int64) error,
) *InboundEventRecorder {
	return &InboundEventRecorder{
		tickets:   tickets,
		messages:  messages,
		advanceOp: updateLast,
	}
}

// DefaultAdvanceLastUserMsgAt 是 updateLast 的默认实现（直接调 storage）。
func DefaultAdvanceLastUserMsgAt(appkey, ticketId string, atMs int64) error {
	return storages.NewTicketStorage().UpdateLastUserMsgAt(appkey, ticketId, atMs)
}

// RecordOnce 留痕 +（必要时）推进 last_user_msg_at；返回 UnreliableOp。
//
// 调用方约定：拿到 UnreliableOp 后**必须**调 Discard() 一次（slog 失败）
// 或显式忽略；类型层把这条契约显式化。
//
// TIPS: 该方法**不**做 last_user_msg_at 的写 DB，仅调注入的 advanceOp 钩子；
// 业务层做语义判断（"是否为坐席"），storage 层做并发安全（GREATEST）。
func (r *InboundEventRecorder) RecordOnce(_ context.Context, appkey string, payload WebhookMessagePayload) UnreliableOp {
	if payload.MsgID == "" || payload.MsgTime == 0 {
		log.Printf("[WebhookRecord] 跳过：msg_id 或 msg_time 缺失 appkey=%s ticket=%s sender=%s msg_id=%s msg_time=%d",
			appkey, payload.Receiver, payload.Sender, payload.MsgID, payload.MsgTime)
		return UnreliableOp{desc: "msg_id_or_time_missing"}
	}
	ticket, err := r.tickets.FindByTicketId(appkey, payload.Receiver)
	if err != nil {
		log.Printf("[WebhookRecord] ticket 查询失败 appkey=%s ticket=%s msg_id=%s err=%v",
			appkey, payload.Receiver, payload.MsgID, err)
		return UnreliableOp{err: err, msgId: payload.MsgID, desc: "ticket_find"}
	}
	if ticket == nil {
		// 工单不存在；不写 ticket_messages，避免"幽灵消息"。
		log.Printf("[WebhookRecord] 跳过：工单不存在 appkey=%s ticket=%s msg_id=%s sender=%s",
			appkey, payload.Receiver, payload.MsgID, payload.Sender)
		return UnreliableOp{desc: "ticket_not_found"}
	}

	log.Printf("[WebhookRecord] ticket 信息 appkey=%s ticket=%s status=%d assignee=%s source=%s msg_type=%s",
		appkey, ticket.TicketId, ticket.Status, ticket.AssigneeId, ticket.SourceId, payload.MsgType)

	role := ClassifySenderRole(ticket, payload.Sender)
	log.Printf("[WebhookRecord] 角色判定 ticket=%s sender=%s role=%s assignee_id=%s source_id=%s",
		ticket.TicketId, payload.Sender, role, ticket.AssigneeId, ticket.SourceId)

	if err := r.messages.UpsertByMsgId(storageModels.TicketMessage{
		AppKey:      appkey,
		TicketId:    payload.Receiver,
		SenderId:    payload.Sender,
		SenderRole:  role,
		MsgId:       payload.MsgID,
		MsgType:     payload.MsgType,
		CreatedTime: payload.MsgTime,
	}); err != nil {
		log.Printf("[WebhookRecord] ticket_messages 写入失败 ticket=%s msg_id=%s err=%v",
			ticket.TicketId, payload.MsgID, err)
		return UnreliableOp{err: err, msgId: payload.MsgID, desc: "ticket_messages_upsert"}
	}

	// 按角色推进对应的时间戳：
	//   - 坐席 → last_user_msg_at（5 分钟空闲判断基准）
	//   - 客户 → last_customer_msg_at（判断"最后一句是谁说的"）
	switch role {
	case storageModels.TicketEventOperatorUser:
		log.Printf("[WebhookRecord] 推进 last_user_msg_at ticket=%s assignee=%s msg_time=%d ms",
			ticket.TicketId, ticket.AssigneeId, payload.MsgTime)
		if err := r.advanceOp(appkey, payload.Receiver, payload.MsgTime); err != nil {
			log.Printf("[WebhookRecord] 推进 last_user_msg_at 失败 ticket=%s msg_id=%s msg_time=%d err=%v",
				ticket.TicketId, payload.MsgID, payload.MsgTime, err)
			return UnreliableOp{err: err, msgId: payload.MsgID, desc: "update_last_user_msg_at"}
		}
	case storageModels.TicketEventOperatorCustomer:
		log.Printf("[WebhookRecord] 推进 last_customer_msg_at ticket=%s source=%s msg_time=%d ms",
			ticket.TicketId, ticket.SourceId, payload.MsgTime)
		if err := r.tickets.UpdateLastCustomerMsgAt(appkey, payload.Receiver, payload.MsgTime); err != nil {
			log.Printf("[WebhookRecord] 推进 last_customer_msg_at 失败 ticket=%s msg_id=%s msg_time=%d err=%v",
				ticket.TicketId, payload.MsgID, payload.MsgTime, err)
			// 客户时间戳写入失败不阻断主流程——降级为"可能误关"而非"webhook 报错"
		}
	default:
		log.Printf("[WebhookRecord] 不推进时间戳 ticket=%s sender=%s role=%s（bot/系统消息）",
			ticket.TicketId, payload.Sender, role)
	}
	return UnreliableOp{desc: "ok"}
}

// inboundEventRecorderHolder 全局 holder（懒加载）。单测可覆盖。
var inboundEventRecorderHolder = struct {
	value *InboundEventRecorder
}{
	value: nil,
}

// GetInboundEventRecorder 取当前 holder 中的 recorder；首次访问时按默认依赖构造。
func GetInboundEventRecorder() *InboundEventRecorder {
	if inboundEventRecorderHolder.value != nil {
		return inboundEventRecorderHolder.value
	}
	rec := NewInboundEventRecorder(
		storages.NewTicketStorage(),
		storages.NewTicketMessageStorage(),
		DefaultAdvanceLastUserMsgAt,
	)
	inboundEventRecorderHolder.value = rec
	return rec
}

// SetInboundEventRecorderForTest 替换 holder 中的 recorder（仅供单测）。
// 测试结束后无需显式复位 —— 下一个测试在 init 阶段会重置。
func SetInboundEventRecorderForTest(r *InboundEventRecorder) {
	inboundEventRecorderHolder.value = r
}
