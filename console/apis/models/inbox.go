package models

type TelegramConfigItem struct {
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token,omitempty"`
}

type InboxItem struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	ChannelType string             `json:"channel_type"`
	ChannelConf TelegramConfigItem `json:"channel_conf"`
	MemberCount int64              `json:"member_count"`
	CreatedTime int64              `json:"created_time"`
	UpdatedTime int64              `json:"updated_time"`
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

type InboxMemberItem struct {
	ID       string `json:"id"`
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
