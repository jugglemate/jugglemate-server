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
// 发送方式：走 IM REST API（imsdk.SendGroupMsg），以工单 assignee 坐席身份发送，
// IsStorage:true 由 IM Server 自动写入 ticket_messages。不再依赖 Bot 长连接。
//
// 错误处理：工单不存在 / 无 assignee 返回错误；IM 发送为 fire-and-forget，
// 调用方应通过 MarkCsatNotifiedOnce 标记已发，失败时下次 ticker 重试。
func NotifyCsatInvitation(ctx context.Context, appKey string, ticketId string, payload interface{}) error {
	_ = payload // 当前签名兼容；真实数据由 SendTicketCsatNtfMsg 用 BuildCsatInvitationPayload 生成。
	appKey = strings.TrimSpace(appKey)
	ticketId = strings.TrimSpace(ticketId)
	if appKey == "" || ticketId == "" {
		return errors.New("csat: appkey/ticketId 不能为空")
	}

	// 1. 查 ticket 获取 assignee 坐席 ID（作为消息发送者）
	ticket, err := newTicketStorageForCsatNotify().FindByTicketId(appKey, ticketId)
	if err != nil || ticket == nil {
		log.Printf("[CsatNotifier] ticket 不存在 appkey=%s ticket=%s err=%v",
			appkeyDebug(appKey), ticketId, err)
		return errors.New("csat: ticket not found")
	}

	senderId := strings.TrimSpace(ticket.AssigneeId)
	if senderId == "" {
		log.Printf("[CsatNotifier] 工单无 assignee 坐席 ticket=%s（无法以坐席身份发送 csat）", ticketId)
		return errors.New("csat: no assignee for ticket")
	}

	// 2. 通过 IM REST API 发送 jgm:csat（坐席身份，IsStorage:true，IM Server 自动写库）
	closedAt := time.Now().UnixMilli()
	csatPayload := BuildCsatInvitationPayload(appKey, ticketId, closedAt)
	sendTicketCsatNtfMsgFn(appKey, ticketId, senderId, csatPayload)
	log.Printf("[CsatNotifier] 发送完成 ticket=%s sender=%s at=%s",
		ticketId, senderId, time.UnixMilli(closedAt).Format(time.RFC3339))

	return nil
}

// appkeyDebug 在 log 里只展示前 6 字符避免大字符串噪音。
func appkeyDebug(s string) string {
	if len(s) > 6 {
		return s[:6] + "…"
	}
	return s
}
