package juggleim

import (
	"fmt"
	"strings"
)

type BotInfo struct {
	Username string
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) SetupBot(botName, botToken, callbackURL string) (*BotInfo, error) {
	botName = strings.TrimSpace(botName)
	if botName == "" {
		return nil, fmt.Errorf("juggleim bot name is empty")
	}
	if strings.TrimSpace(botToken) == "" {
		return nil, fmt.Errorf("juggleim bot token is empty")
	}
	if strings.TrimSpace(callbackURL) == "" {
		return nil, fmt.Errorf("juggleim callback URL is empty")
	}
	return &BotInfo{Username: botName}, nil
}
