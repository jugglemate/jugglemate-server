package services

import (
	"context"
	"strings"
	"time"

	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

// 注入点：service 单测或上层 Agent 平台可通过这两个变量替换底层存储。
var (
	newTicketStorageForHumanHandoff  = storages.NewTicketStorage
	newTicketEventStorageForHandoff = storages.NewTicketEventStorage
)

// MarkHumanTakeover 持久化「工单转人工」事实，并追加一条 ticket_events 记录。
//
// 调用顺序固定为「UPDATE 事实 → INSERT 事件」，两步都失败时返回错误，由调用方
// 决定是否继续 IM 副作用（IM 入站推荐：DB 失败直接中断；控制台手工触发可继续）。
//
// 关键不变量：
//   - is_human_taken_over 只能 0→1；重复触发只补事件、不更新 human_taken_over_at；
//   - operatorId / operatorType 必须都非空，且 operatorType 必须是合法 enum。
//
// @param ctx 请求上下文
// @param appKey 应用 AppKey
// @param ticketId 目标工单 ID
// @param operatorId 触发者标识（客户时填 customer_id，坐席时填 user_id）
// @param operatorType 触发者类型：customer / user / system
func MarkHumanTakeover(ctx context.Context, appKey, ticketId, operatorId, operatorType string) error {
	appKey, ticketId = strings.TrimSpace(appKey), strings.TrimSpace(ticketId)
	if appKey == "" || ticketId == "" {
		return errTicketHumanHandoffParam("appKey/ticketId 不能为空")
	}
	operatorId = strings.TrimSpace(operatorId)
	if operatorId == "" {
		return errTicketHumanHandoffParam("operatorId 不能为空")
	}
	switch storageModels.TicketEventOperator(operatorType) {
	case storageModels.TicketEventOperatorCustomer,
		storageModels.TicketEventOperatorUser,
		storageModels.TicketEventOperatorSystem:
	default:
		return errTicketHumanHandoffParam("operatorType 非法")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	atMs := time.Now().UnixMilli()
	ticketStorage := newTicketStorageForHumanHandoff()
	if err := ticketStorage.MarkHumanTakenOverIfZero(appKey, ticketId, operatorId, atMs); err != nil {
		return errTicketHumanHandoffInternal("持久化工单转人工事实失败", err)
	}
	eventStorage := newTicketEventStorageForHandoff()
	if err := eventStorage.Create(storageModels.TicketEvent{
		AppKey:       appKey,
		TicketId:     ticketId,
		EventType:    storageModels.TicketEventTypeHumanTakeover,
		OperatorID:   operatorId,
		OperatorType: storageModels.TicketEventOperator(operatorType),
		Payload:      `{"trigger":"keyword"}`,
		CreatedTime:  atMs,
	}); err != nil {
		return errTicketHumanHandoffInternal("写入工单事件失败", err)
	}
	return nil
}

// QryTicketEvents 拉取指定工单的事件列表（倒序）。
func QryTicketEvents(ctx context.Context, appKey, ticketId string, limit, offset int64) ([]*storageModels.TicketEvent, error) {
	appKey, ticketId = strings.TrimSpace(appKey), strings.TrimSpace(ticketId)
	if appKey == "" || ticketId == "" {
		return nil, errTicketHumanHandoffParam("appKey/ticketId 不能为空")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return newTicketEventStorageForHandoff().QryByTicket(appKey, ticketId, limit, offset)
}

// 错误辅助：抽离出来让 service 层错误体系列保持一致。
type ticketHumanHandoffError struct {
	stage string
	err   error
}

func (e *ticketHumanHandoffError) Error() string {
	if e == nil {
		return ""
	}
	if e.err == nil {
		return e.stage
	}
	return e.stage + ": " + e.err.Error()
}

func (e *ticketHumanHandoffError) Unwrap() error { return e.err }

func errTicketHumanHandoffParam(msg string) error {
	return &ticketHumanHandoffError{stage: msg}
}

func errTicketHumanHandoffInternal(stage string, err error) error {
	return &ticketHumanHandoffError{stage: stage, err: err}
}
