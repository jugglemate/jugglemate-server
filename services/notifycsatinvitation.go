package services

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

var (
	newTicketStorageForCsatNotify = storages.NewTicketStorage
)

// csatIMSender 是 jgm:csat 邀请卡到 IM SDK 的桥接函数。
//
// 由 main.go 在装配 Agent 模块时注入；services 包不持有 IM SDK 引用。
// 签名：
//   ctx: 上下文
//   appKey: 目标 app
//   botUserID: 当前 Inbox 的 Agent Bot IM 身份
//   ticketId: ticket 群 ID（target）
//   msgType: 固定 "jgm:csat"（前端常量）
//   payload: 服务端 CsatInvitationPayload（自动 JSON）
var csatIMSender func(ctx context.Context, appKey string, botUserID string, ticketId string, msgType string, payload interface{}) error

// SetCsatIMSender 注入 IM SDK 发送函数；Agent 模块 bootstrap 完成时调用。
func SetCsatIMSender(fn func(ctx context.Context, appKey string, botUserID string, ticketId string, msgType string, payload interface{}) error) {
	csatIMSender = fn
}

// NotifyCsatInvitation 工单关闭后调此发邀请卡片。
//
// 通过 ticket 拿 inbox，再查 inbox 的 Agent 绑定拿到 bot_user_id，最后通过
// csatIMSender 把 msg_type=jgm:csat 的 msg_content 发到 ticket 群。
//
// 错误处理：工单已不存在 / inbox 未绑 bot / IM 失败都返回错误（业务侧记录
// 但不重试；工单已关闭）。
//
// @param payload 必须是 CsatInvitationPayload 类型（或兼容的 struct/map），字段在
// BuildCsatInvitationPayload 内部填好（appkey、ticketId、时间）。
// 当前实现用 payload 作 carrier；签名与 bootstrap.CsatNotifySenderFunc 一致。
func NotifyCsatInvitation(ctx context.Context, appKey string, ticketId string, payload interface{}) error {
	_ = payload // 当前签名兼容；真实数据由 IM Sender 用 BuildCsatInvitationPayload 重新生成。
	if csatIMSender == nil {
		log.Printf("[CsatNotifier] csatIMSender 未注入，跳过 ticket=%s", ticketId)
		return nil
	}
	appKey = strings.TrimSpace(appKey)
	ticketId = strings.TrimSpace(ticketId)
	if appKey == "" || ticketId == "" {
		return errors.New("csat: appkey/ticketId 不能为空")
	}

	// 1. 找 ticket 拿 inbox_id
	ticket, err := newTicketStorageForCsatNotify().FindByTicketId(appKey, ticketId)
	if err != nil || ticket == nil || strings.TrimSpace(ticket.InboxId) == "" {
		log.Printf("[CsatNotifier] ticket/inbox 不存在 appkey=%s ticket=%s err=%v",
			appkeyDebug(appKey), ticketId, err)
		return errors.New("csat: ticket not found or has no inbox")
	}

	// 2. 查 inbox 的 active Agent 绑定，拿到 bot_user_id
	detail, err := GetInboxAgent(ctx, appKey, ticket.InboxId)
	if err != nil {
		log.Printf("[CsatNotifier] GetInboxAgent 失败 ticket=%s err=%v", ticketId, err)
		return err
	}
	botUserID := ""
	if detail != nil {
		botUserID = detail.BotUserID
	}
	if botUserID == "" {
		log.Printf("[CsatNotifier] 找不到 inbox bot 绑定 ticket=%s inbox=%s",
			ticketId, ticket.InboxId)
		return errors.New("csat: no bot binding for ticket inbox")
	}

	// 3. 通过注入的 IM Sender 发 jgm:csat 消息
	closedAt := time.Now().UnixMilli()
	csatPayload := BuildCsatInvitationPayload(appKey, ticketId, closedAt)
	if err := csatIMSender(ctx, appKey, botUserID, ticketId, "jgm:csat", csatPayload); err != nil {
		log.Printf("[CsatNotifier] 发送 jgm:csat 失败 ticket=%s bot=%s err=%v", ticketId, botUserID, err)
		return err
	}
	log.Printf("[CsatNotifier] OK ticket=%s bot=%s at=%s",
		ticketId, botUserID, time.UnixMilli(closedAt).Format(time.RFC3339))
	return nil
}

// appkeyDebug 在 log 里只展示前 6 字符避免大字符串噪音。
func appkeyDebug(s string) string {
	if len(s) > 6 {
		return s[:6] + "…"
	}
	return s
}

var _ storageModels.Ticket = storageModels.Ticket{}
