package services

import "encoding/json"

type ChannelType string

const (
	ChannelType_Widget   ChannelType = "widget"
	ChannelType_Telegram ChannelType = "telegram"
)

type TelegramChannelConf struct {
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token"`
}

type WebWidgetChannelConf struct {
	WelcomeMessage string `json:"welcome_message"`
}

func ParseWebWidgetChannelConf(channelConf string) WebWidgetChannelConf {
	if channelConf == "" {
		return WebWidgetChannelConf{}
	}
	var conf WebWidgetChannelConf
	if err := json.Unmarshal([]byte(channelConf), &conf); err != nil {
		return WebWidgetChannelConf{}
	}
	return conf
}

func ParseTelegramChannelConf(channelConf string) TelegramChannelConf {
	if channelConf == "" {
		return TelegramChannelConf{}
	}
	var conf TelegramChannelConf
	if err := json.Unmarshal([]byte(channelConf), &conf); err != nil {
		return TelegramChannelConf{}
	}
	return conf
}
