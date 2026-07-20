package service

import (
	"context"
	"strings"
)

// handoffKeywords 是客户在 Ticket 群里触发转人工的关键词。
//
// TIPS: 只做整句精确匹配，不做包含匹配。"人工"这类词在正常提问里极常见
// （"这个是人工审核吗"、"人工费怎么算"），一旦用包含匹配就会大量误触发，
// 而误触发的代价是 Bot 被移出群且不会自动回来，必须人工恢复，比漏触发严重得多。
var handoffKeywords = map[string]struct{}{
	"转人工": {}, "人工": {}, "人工客服": {}, "转客服": {}, "转人工客服": {},
	"要人工": {}, "找人工": {}, "人工服务": {}, "转接人工": {},
}

// MatchHandoffKeyword 判断客户消息是否为转人工指令。
//
// @param text 客户发送的原始文本
// @return 命中关键词返回 true
func MatchHandoffKeyword(text string) bool {
	normalized := strings.TrimSpace(text)
	normalized = strings.Trim(normalized, "。！!？?，,、.~ ")
	normalized = strings.ReplaceAll(normalized, " ", "")
	if normalized == "" {
		return false
	}
	_, ok := handoffKeywords[normalized]
	return ok
}

// handoffToHuman 由客户在 Ticket 群触发转人工：拉入坐席、广播通知并把 Agent Bot 移出群。
//
// TIPS: 转人工是一次性的、不可逆的解绑动作 —— Bot 被移出群之后，IM 不再向它投递该群的
// 任何消息，本服务也就彻底看不到这个工单了。后续的人工接待完全由坐席在 IM 群内自行完成，
// 与 Agent 无关，因此这里不需要维护任何"人工态"标志：没有 Bot 在群里，也就没有需要静默的
// 对象。这正是人工介入的 Redis 会话机制被整体移除的原因。
func (service *Service) handoffToHuman(ctx context.Context, appKey, ticketID, botUserID string) error {
	if service.humanHandoff == nil {
		return messageError(500, "500_HUMAN_HANDOFF_NOT_CONFIGURED", "转人工能力未装配")
	}
	return service.humanHandoff(ctx, appKey, ticketID, botUserID)
}
