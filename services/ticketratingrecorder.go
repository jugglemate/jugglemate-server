package services

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/juggleim/jugglemate-server/storages"
	"github.com/juggleim/jugglemate-server/storages/dbs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

// 注入点：单测可以替换。
var (
	newTicketRatingStorageForRating = storages.NewTicketRatingStorage
	newTicketMessageStorageForRating = storages.NewTicketMessageStorage
	newTicketStorageForRating        = storages.NewTicketStorage
)

// OverrideTestRatingStorage 是给外部 apis 层测试用的钩子（不需要把测试代码引入 services 包）。
// apis/ticketrating_test.go 通过赋值此变量来替换底层存储。
// 单测结束后请置 nil。
var OverrideTestRatingStorage func() storageModels.ITicketRatingStorage

// SetCSATCallerForTest 是占位符号，handler 测试用 OverrideTestRatingStorage 即可。
func SetCSATCallerForTest() {}

// 评价系统错误：纯 Go 错误，前端可通过 errors.Is(err, services.ErrCsatXxx) 判断。
var (
	ErrCsatTicketNotFound    = errors.New("csat: 工单不存在")
	ErrCsatRatingOutOfRange  = errors.New("csat: rating 必须 1..5")
	ErrCsatInvalidKind       = errors.New("csat: kind 必须是 csatreply")
	ErrCsatCommentTooLong    = errors.New("csat: comment 长度超过 500 字符")
	ErrCsatSenderNotCustomer = errors.New("csat: 触发者不是客户")
	ErrCsatAppKeyMismatch    = errors.New("csat: msg_content.app_key 与当前 app 不一致")
	ErrCsatTicketIdMismatch  = errors.New("csat: msg_content.ticket_id 与 receiver 不一致")
	ErrCsatAlreadyRated      = errors.New("csat: 该客户已评过当前工单")
)

// RecordFromCustomMessage 处理 webhook 收到的 jgm:csatreply 消息：
// 解析 msg_content，校验，落 ticket_ratings 表，同时写 ticket_messages 留痕。
//
// @param appKey ctx 里的 appkey（强信任，msg_content.app_key 与之必须一致）
// @param receiverTicketId webhook 入参 receiver（即 ticket_id）
// @param payloadMsgContent 必须是 jgm:csatreply 的 msg_content 序列化结果（JSON 字符串）
// @param customerId 当前 web hook sender（必须等于 ticket.SourceId）
// @param msgId webhook 入参 msg_id（用于 ticket_messages 幂等）
// @param msgType 原始 msg_type（jgm:csatreply）
// @param msgTimeMs IM server 给的毫秒时间戳
// @return 已写入的 TicketRating；重复评分时返回 ErrCsatAlreadyRated（调用方静默忽略）
func RecordFromCustomMessage(
	ctx context.Context,
	appKey string,
	receiverTicketId string,
	payloadMsgContent string,
	senderId string,
	msgId string,
	msgType string,
	msgTimeMs int64,
) (*storageModels.TicketRating, error) {
	appKey = strings.TrimSpace(appKey)
	if appKey == "" {
		return nil, ErrCsatTicketNotFound
	}
	receiverTicketId = strings.TrimSpace(receiverTicketId)

	// 1. 反序列化 msg_content
	var reply CsatReplyPayload
	if err := json.Unmarshal([]byte(payloadMsgContent), &reply); err != nil {
		log.Printf("[CsatReply] 解析 msg_content 失败 msg_id=%s err=%v", msgId, err)
		return nil, err
	}

	// 2. 强校验 app_key / ticket_id 防伪造
	if reply.AppKey != appKey {
		return nil, ErrCsatAppKeyMismatch
	}
	if reply.TicketID != receiverTicketId {
		return nil, ErrCsatTicketIdMismatch
	}

	// 3. schema 校验：rating 范围必须严格；comment 长度超过 500 服务端截断而非拒收
	// （前端可能允许 emoji 等多字节字符计算误差；宽容处理避免客户评分丢失）
	if reply.Kind != "csatreply" {
		return nil, ErrCsatInvalidKind
	}
	if reply.Rating < 1 || reply.Rating > 5 {
		return nil, ErrCsatRatingOutOfRange
	}
	rating := reply.Rating
	comment := reply.Comment
	if len(comment) > 500 {
		comment = comment[:500]
	}

	// 4. 找 ticket 拿 assignee_id + 验证 sender 是客户
	ticket, err := newTicketStorageForRating().FindByTicketId(appKey, receiverTicketId)
	if err != nil || ticket == nil {
		return nil, ErrCsatTicketNotFound
	}
	if strings.TrimSpace(ticket.SourceId) != strings.TrimSpace(senderId) {
		log.Printf("[CsatReply] 非客户发送拒绝 sender=%s source_id=%s ticket=%s",
			senderId, ticket.SourceId, ticket.TicketId)
		return nil, ErrCsatSenderNotCustomer
	}

	// 5. source 字段：comment 非空 → card_with_comment；否则 card_button
	source := storageModels.TicketRatingSourceCardButton
	if comment != "" {
		source = storageModels.TicketRatingSourceCardWithComment
	}

	// 6. 写 ticket_ratings（UNIQUE 防重）
	nowMs := time.Now().UnixMilli()
	if reply.ClientTs > 0 && reply.ClientTs < nowMs {
		nowMs = reply.ClientTs
	}
	item := storageModels.TicketRating{
		AppKey:      appKey,
		TicketId:    receiverTicketId,
		CustomerId:  ticket.SourceId,
		AssigneeId:  ticket.AssigneeId,
		Rating:      rating,
		Comment:     comment,
		Source:      source,
		CreatedTime: nowMs,
	}

	// 7. 先写 ticket_messages 留痕（UpsertByMsgId 幂等；先写保证 retry 时也能补 messages）；
	//    即使后续 rating 写失败，audit 数据已落库便于人工回查。
	if err := newTicketMessageStorageForRating().UpsertByMsgId(storageModels.TicketMessage{
		AppKey:      appKey,
		TicketId:    receiverTicketId,
		SenderId:    senderId,
		SenderRole:  storageModels.TicketEventOperatorCustomer,
		MsgId:       msgId,
		MsgType:     msgType,
		CreatedTime: msgTimeMs,
	}); err != nil {
		log.Printf("[CsatReply] ticket_messages 写库失败 msg_id=%s err=%v（继续）", msgId, err)
	}

	// 8. 写 ticket_ratings（UNIQUE 防重）。
	err = newTicketRatingStorageForRating().Create(item)
	if err != nil {
		if errors.Is(err, dbs.ErrTicketRatingAlreadyExists) {
			log.Printf("[CsatReply] 该工单已评过 msg_id=%s ticket=%s customer=%s",
				msgId, receiverTicketId, ticket.SourceId)
			return nil, ErrCsatAlreadyRated
		}
		log.Printf("[CsatReply] 写库失败 msg_id=%s err=%v", msgId, err)
		return nil, err
	}

	log.Printf("[CsatReply] OK ticket=%s rating=%d has_comment=%t customer=%s",
		receiverTicketId, rating, comment != "", ticket.SourceId)
	return &item, nil
}

// GetTicketRating 给 GET API 与 webhook 用来查重。
func GetTicketRating(appKey, ticketId, customerId string) (*storageModels.TicketRating, error) {
	appKey = strings.TrimSpace(appKey)
	ticketId = strings.TrimSpace(ticketId)
	customerId = strings.TrimSpace(customerId)
	if appKey == "" || ticketId == "" || customerId == "" {
		return nil, nil
	}
	if OverrideTestRatingStorage != nil {
		return OverrideTestRatingStorage().FindByTicketAndCustomer(appKey, ticketId, customerId)
	}
	return newTicketRatingStorageForRating().FindByTicketAndCustomer(appKey, ticketId, customerId)
}
