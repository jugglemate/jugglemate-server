package services

type ChannelType string

const (
	ChannelType_Widget   ChannelType = "widget"
	ChannelType_Telegram ChannelType = "telegram"
)

type TelegramChannelConf struct {
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token"`
}
