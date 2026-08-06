package services

import "encoding/json"

type ChannelType string

const (
	ChannelType_Widget   ChannelType = "widget"
	ChannelType_Telegram ChannelType = "telegram"
	ChannelType_JuggleIM ChannelType = "juggleim"
	ChannelType_WhatsApp ChannelType = "whatsapp"
)

type TelegramChannelConf struct {
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token"`
}

type JuggleIMChannelConf struct {
	BotName  string `json:"bot_name"`
	BotToken string `json:"bot_token"`
}

type WebWidgetChannelConf struct {
	WelcomeMessage string `json:"welcome_message"`
}

type WhatsAppChannelConf struct {
	AccessToken        string `json:"access_token"`
	PhoneNumberID      string `json:"phone_number_id"`
	WebhookVerifyToken string `json:"webhook_verify_token"`
	AppSecret          string `json:"app_secret,omitempty"`
	DisplayPhoneNumber string `json:"display_phone_number,omitempty"`
	VerifiedName       string `json:"verified_name,omitempty"`
	APIBaseURL         string `json:"api_base_url,omitempty"`
	APIVersion         string `json:"api_version,omitempty"`
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

func ParseJuggleIMChannelConf(channelConf string) JuggleIMChannelConf {
	if channelConf == "" {
		return JuggleIMChannelConf{}
	}
	var conf JuggleIMChannelConf
	if err := json.Unmarshal([]byte(channelConf), &conf); err != nil {
		return JuggleIMChannelConf{}
	}
	return conf
}

func ParseWhatsAppChannelConf(channelConf string) WhatsAppChannelConf {
	if channelConf == "" {
		return WhatsAppChannelConf{}
	}
	var conf WhatsAppChannelConf
	if err := json.Unmarshal([]byte(channelConf), &conf); err != nil {
		return WhatsAppChannelConf{}
	}
	return conf
}
