package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const apiBaseURL = "https://api.telegram.org"

type BotInfo struct {
	ID       int64
	Username string
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    apiBaseURL,
	}
}

func (c *Client) GetMe(botToken string) (*BotInfo, error) {
	var resp struct {
		OK     bool `json:"ok"`
		Result struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := c.do(botToken, "getMe", nil, &resp); err != nil {
		return nil, err
	}
	return &BotInfo{ID: resp.Result.ID, Username: resp.Result.Username}, nil
}

func (c *Client) DeleteWebhook(botToken string) error {
	var resp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	return c.do(botToken, "deleteWebhook", nil, &resp)
}

func (c *Client) SetWebhook(botToken, callbackURL string) error {
	var resp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	return c.do(botToken, "setWebhook", map[string]string{"url": callbackURL}, &resp)
}

func (c *Client) do(botToken, method string, body any, out any) error {
	if botToken == "" {
		return fmt.Errorf("telegram bot token is empty")
	}
	url := fmt.Sprintf("%s/bot%s/%s", c.baseURL, botToken, method)
	if body == nil {
		body = map[string]any{}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	ok, description := telegramAPIStatus(out)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !ok {
		if description == "" {
			description = resp.Status
		}
		return fmt.Errorf("telegram %s failed: %s", method, description)
	}
	return nil
}

func telegramAPIStatus(out any) (bool, string) {
	raw, err := json.Marshal(out)
	if err != nil {
		return false, ""
	}
	var status struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		return false, ""
	}
	return status.OK, status.Description
}
