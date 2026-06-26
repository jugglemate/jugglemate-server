package models

type StartCustomReq struct {
	Identifier string `json:"identifier"`
	Nickname   string `json:"nickname"`

	InboxId string `json:"inbox_id"`
}

type StartCustomResp struct {
	ConversationId   string `json:"conversation_id"`
	ConversationType int    `json:"conversation_type"`
	UserId           string `json:"user_id"`
	Nickname         string `json:"nickname"`
	ImToken          string `json:"im_token"`
	WelcomeMessage   string `json:"welcome_message"`
}
