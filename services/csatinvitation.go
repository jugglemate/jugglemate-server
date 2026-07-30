package services

import (
	"fmt"
	"strings"
)

// CsatInvitationPayload 是 jgm:csat 邀请卡消息体。
//
// 该消息经 SDK SendCustomMessage 发送：IM server 把 msg_type 与 msg_content 透传给
// 各客户端（widget / juggleim / telegram）。前端 UI 按 kind == "csat" 路由到评分卡片。
// 客户端评分后将数据封装为 jgm:csatreply 自定义消息发回群里，后端 webhook 解析入库。
type CsatInvitationPayload struct {
	Kind    string                   `json:"kind"`
	Version int                      `json:"version"`
	AppKey  string                   `json:"app_key"`
	TicketID string                  `json:"ticket_id"`
	Text    string                   `json:"text"`
	Csat    CsatInvitationCsatBlock  `json:"csat"`
}

// CsatInvitationCsatBlock 描述评分卡片的具体内容。
type CsatInvitationCsatBlock struct {
	LowRatingThreshold int               `json:"low_rating_threshold"`
	LowRatingHint      string            `json:"low_rating_hint"`
	Options            []CsatOptionItem   `json:"options"`
	ReplyMsgType       string            `json:"reply_msg_type"`
	ReplySchema        map[string]string `json:"reply_schema"`
}

// CsatOptionItem 是评分项。
type CsatOptionItem struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

// BuildCsatInvitationPayload 拼装服务端发送给客户的"邀请评价"卡片。
//
// @param appKey 应用 AppKey
// @param ticketId 工单 ID
// @param closedAtMs ticker 命中关闭的时间戳（ms），前端展示"已关闭"可参考
// @return 已 JSON-serializable 的 msg_content
func BuildCsatInvitationPayload(appKey, ticketId string, closedAtMs int64) CsatInvitationPayload {
	return CsatInvitationPayload{
		Kind:     "csat",
		Version:  1,
		AppKey:   strings.TrimSpace(appKey),
		TicketID: strings.TrimSpace(ticketId),
		Text: "您的会话已结束。请对本次服务做个评价：\n" +
			"1 很差 / 2 不满意 / 3 一般 / 4 满意 / 5 很满意。\n" +
			"评分 ≤3 时建议同时写点意见，方便我们改进。",
		Csat: CsatInvitationCsatBlock{
			LowRatingThreshold: 3,
			LowRatingHint:      "评分 ≤3 时，请附带文字说明问题，方便我们改进。",
			Options: []CsatOptionItem{
				{Value: 1, Label: "1 很差"},
				{Value: 2, Label: "2 不满意"},
				{Value: 3, Label: "3 一般"},
				{Value: 4, Label: "4 满意"},
				{Value: 5, Label: "5 很满意"},
			},
			ReplyMsgType: CsatReplyMsgType,
			ReplySchema: map[string]string{
				"rating":  "int 1..5",
				"comment": "string optional (≤ 500 chars); recommended when rating ≤ 3",
			},
		},
	}
}

// CsatReplyMsgType 是客户回复评分时使用的 IM 自定义消息类型名。
const CsatReplyMsgType = "jgm:csatreply"

// CsatReplyPayload 是 jgm:csatreply 评分消息的 msg_content。
//
// 客户端 imSdk.sendGroupMsg(msg_type="jgm:csatreply", content=此结构) 发到群里，
// webhook 入站后由 services/ticketoutboundservice.go 解析为 ticket_ratings 写入。
type CsatReplyPayload struct {
	Kind     string `json:"kind"`
	Version  int    `json:"version"`
	AppKey   string `json:"app_key"`
	TicketID string `json:"ticket_id"`
	Rating   int    `json:"rating"`
	Comment  string `json:"comment"`
	ClientTs int64  `json:"client_ts,omitempty"`
}

// ValidateCsatReplyPayload 验证 msg_content 是否合法。
//
// @return error 为 nil 即合法
func ValidateCsatReplyPayload(p CsatReplyPayload) error {
	if p.Kind != "csatreply" {
		return fmt.Errorf("csat reply kind 必须是 csatreply，实际 %q", p.Kind)
	}
	if p.Rating < 1 || p.Rating > 5 {
		return fmt.Errorf("csat reply rating 必须 1..5，实际 %d", p.Rating)
	}
	if len(p.Comment) > 500 {
		return fmt.Errorf("csat reply comment 长度 %d 超过 500 字符上限", len(p.Comment))
	}
	return nil
}
