package models

type AiBotInfos struct {
	Items  []*AiBotInfo `json:"items"`
	Offset string       `json:"offset"`
}

type AiBotInfo struct {
	BotId          string `json:"bot_id"`
	UniqueName     string `json:"unique_name"`
	Nickname       string `json:"nickname"`
	DisplayName    string `json:"display_name"`
	Avatar         string `json:"avatar"`
	AvatarURL      string `json:"avatar_url"`
	Prompts        string `json:"prompts"`
	Greeting       string `json:"greeting"`
	OwnerId        string `json:"owner_id"`
	Status         string `json:"status"`
	ActiveVersion  string `json:"active_version"`
	TrainingMode   string `json:"training_mode"`
	MaterialsCount int    `json:"materials_count"`
	CreatedTime    int64  `json:"created_time"`
	UpdatedTime    int64  `json:"updated_time"`
}

type UpdateAiBotReq struct {
	BotId       string  `json:"bot_id"`
	UniqueName  *string `json:"unique_name"`
	Nickname    *string `json:"nickname"`
	DisplayName *string `json:"display_name"`
	Avatar      *string `json:"avatar"`
	AvatarURL   *string `json:"avatar_url"`
	Prompts     *string `json:"prompts"`
	Greeting    *string `json:"greeting"`
}

type AiMaterialInfo struct {
	Id        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	Content   string `json:"content,omitempty"`
	Url       string `json:"url,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	SizeBytes int64  `json:"size_bytes"`
}

type AiMaterialInfos struct {
	Items  []*AiMaterialInfo `json:"items"`
	Offset string            `json:"offset"`
}
