package services

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/juggleim/jugglemate-server/storages"
)

var (
	newTicketStorageForCsatNotify = storages.NewTicketStorage
)

// NotifyCsatInvitation 工单关闭后发送 jgm:csat 评价邀请卡片到 Ticket 群。
//
// 发送方式：走 IM REST API（imsdk.SendGroupMsg），IsStorage:true 由 IM Server
// 自动写入 ticket_messages。
//
// 发送者身份自动判定：
//   - 转人工工单（AssigneeId 非空）→ 以坐席身份发送
//   - 纯 Bot 工单（AssigneeId 为空）→ 以 Bot 身份发送，通过 GetInboxAgent 查 BotUserID
//
// 错误处理：工单不存在 / bot 查找失败（仅纯 Bot 工单）返回错误；
// IM 发送为 fire-and-forget，调用方应通过 MarkCsatNotifiedOnce 标记已发。
func NotifyCsatInvitation(ctx context.Context, appKey string, ticketId string, payload interface{}) error {
	_ = payload // 当前签名兼容；真实数据由 SendTicketCsatNtfMsg 用 BuildCsatInvitationPayload 生成。
	appKey = strings.TrimSpace(appKey)
	ticketId = strings.TrimSpace(ticketId)
	if appKey == "" || ticketId == "" {
		return errors.New("csat: appkey/ticketId 不能为空")
	}

	// 1. 查 ticket 获取 assignee 和 inboxId（用于兜底查 Bot 身份）
	ticket, err := newTicketStorageForCsatNotify().FindByTicketId(appKey, ticketId)
	if err != nil || ticket == nil {
		log.Printf("[CsatNotifier] ticket 不存在 appkey=%s ticket=%s err=%v",
			appkeyDebug(appKey), ticketId, err)
		return errors.New("csat: ticket not found")
	}

	// 2. 判定发送者身份：
	//    - 有 assignee → 坐席身份（转人工工单）
	//    - 无 assignee → Bot 身份（纯 Bot 对话工单，用 GetInboxAgent 查 BotUserID）
	var senderId string
	var senderLabel string

	if assigneeId := strings.TrimSpace(ticket.AssigneeId); assigneeId != "" {
		senderId = assigneeId
		senderLabel = "坐席"
	} else {
		// 纯 Bot 工单：通过 inbox_agent_bindings 查 Bot 的 IM 用户 ID
		botDetail, botErr := GetInboxAgent(ctx, appKey, ticket.InboxId)
		if botErr != nil || botDetail == nil {
			log.Printf("[CsatNotifier] 无法获取 Bot 身份 ticket=%s inbox_id=%s is_human_taken_over=%v err=%v",
				ticketId, ticket.InboxId, ticket.IsHumanTakenOver, botErr)
			return errors.New("csat: cannot resolve bot identity for auto-close csat")
		}
		senderId = botDetail.BotUserID
		senderLabel = "Bot"
	}

	if senderId == "" {
		log.Printf("[CsatNotifier] 发送者身份为空 ticket=%s is_human_taken_over=%v assignee_id=%q",
			ticketId, ticket.IsHumanTakenOver, ticket.AssigneeId)
		return errors.New("csat: empty sender id")
	}

	// 3. 通过 IM REST API 发送 jgm:csat（IsStorage:true，IM Server 自动写库）
	closedAt := time.Now().UnixMilli()
	csatPayload := BuildCsatInvitationPayload(appKey, ticketId, closedAt)
	sendTicketCsatNtfMsgFn(appKey, ticketId, senderId, csatPayload)
	log.Printf("[CsatNotifier] 发送完成 ticket=%s sender=%s(%s) is_human_taken_over=%v at=%s",
		ticketId, senderLabel, senderId, ticket.IsHumanTakenOver,
		time.UnixMilli(closedAt).Format(time.RFC3339))

	return nil
}

// appkeyDebug 在 log 里只展示前 6 字符避免大字符串噪音。
func appkeyDebug(s string) string {
	if len(s) > 6 {
		return s[:6] + "…"
	}
	return s
}
