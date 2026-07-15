package models

type TelegramConfigItem struct {
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token,omitempty"`
}

type JuggleIMConfigItem struct {
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token,omitempty"`
}

type WidgetConfigItem struct {
	WelcomeMessage string `json:"welcome_message"`
}

type InboxItem struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	ChannelType string      `json:"channel_type"`
	ChannelConf interface{} `json:"channel_conf"`
	MemberCount int64       `json:"member_count"`
	CreatedTime int64       `json:"created_time"`
	UpdatedTime int64       `json:"updated_time"`
}

type InboxListResp struct {
	List  []InboxItem `json:"list"`
	Total int64       `json:"total"`
}

type CreateTelegramInboxReq struct {
	Name     string `json:"name"`
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token"`
}

type CreateJuggleIMInboxReq struct {
	Name     string `json:"name"`
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token"`
}

type CreateWidgetInboxReq struct {
	Name           string `json:"name"`
	WelcomeMessage string `json:"welcome_message"`
}

type InboxMemberItem struct {
	UserId   string `json:"user_id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Email    string `json:"email"`
}

type InboxMembersResp struct {
	List []InboxMemberItem `json:"list"`
}

type ReplaceInboxMembersReq struct {
	UserIds []string `json:"user_ids"`
}

// UpdateInboxReq 更新收件箱请求体。
// Name 为必填；WelcomeMessage 仅对网站挂件（widget）渠道生效。
type UpdateInboxReq struct {
	Name           string `json:"name"`
	WelcomeMessage string `json:"welcome_message"`
}
