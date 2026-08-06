package models

type TelegramConfigItem struct {
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token,omitempty"`
}

type JuggleIMConfigItem struct {
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token,omitempty"`
}

type WhatsAppConfigItem struct {
	PhoneNumberID      string `json:"phone_number_id"`
	DisplayPhoneNumber string `json:"display_phone_number,omitempty"`
	VerifiedName       string `json:"verified_name,omitempty"`
	APIVersion         string `json:"api_version,omitempty"`
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

type CreateWhatsAppInboxReq struct {
	Name               string `json:"name"`
	PhoneNumberID      string `json:"phone_number_id"`
	AccessToken        string `json:"access_token"`
	WebhookVerifyToken string `json:"webhook_verify_token"`
	AppSecret          string `json:"app_secret"`
	APIBaseURL         string `json:"api_base_url"`
	APIVersion         string `json:"api_version"`
}

type CreateWidgetInboxReq struct {
	Name           string `json:"name"`
	WelcomeMessage string `json:"welcome_message"`
}

// InboxMemberItem 收件箱成员。
//
// TIPS: id 字段与控制台其它用户接口（如 UserItem）保持一致，前端按 id 关联角色和去重。
type InboxMemberItem struct {
	UserId   string `json:"id"`
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

// BindInboxAgentReq 更新 Inbox 当前关联的 Agent。
type BindInboxAgentReq struct {
	AgentID string `json:"agent_id"`
}

// InboxAgentItem 表示 Inbox 当前真实生效的 Agent 与 Bot。
type InboxAgentItem struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	BotID     string `json:"bot_id"`
	BotUserID string `json:"bot_user_id"`
	BotName   string `json:"bot_name"`
	SyncError string `json:"sync_error,omitempty"`
}
