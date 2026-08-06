package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultAPIBaseURL = "https://graph.facebook.com"
	DefaultAPIVersion = "v24.0"
)

type Client struct {
	httpClient  *http.Client
	baseURL     string
	version     string
	accessToken string
	phoneID     string
}

type PhoneInfo struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	VerifiedName       string `json:"verified_name"`
	ID                 string `json:"id"`
}

type SendMessageResponse struct {
	MessageID string
}

func NewClient(accessToken, phoneID, baseURL, version string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultAPIBaseURL
	}
	if strings.TrimSpace(version) == "" {
		version = DefaultAPIVersion
	}
	return &Client{
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		baseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		version:     strings.Trim(strings.TrimSpace(version), "/"),
		accessToken: strings.TrimSpace(accessToken),
		phoneID:     strings.TrimSpace(phoneID),
	}
}

func (c *Client) GetPhoneInfo(ctx context.Context) (*PhoneInfo, error) {
	if c.accessToken == "" || c.phoneID == "" {
		return nil, fmt.Errorf("whatsapp access token and phone number id are required")
	}
	var info PhoneInfo
	if err := c.doJSON(ctx, http.MethodGet, c.resourceURL(c.phoneID)+"?fields=display_phone_number,verified_name,id", nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *Client) SendText(ctx context.Context, recipient, text string) (*SendMessageResponse, error) {
	if c.accessToken == "" || c.phoneID == "" {
		return nil, fmt.Errorf("whatsapp access token and phone number id are required")
	}
	recipient = strings.TrimSpace(recipient)
	text = strings.TrimSpace(text)
	if recipient == "" || text == "" {
		return nil, fmt.Errorf("whatsapp recipient and text are required")
	}
	body := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                recipient,
		"type":              "text",
		"text":              map[string]string{"body": text},
	}
	var response struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.resourceURL(c.phoneID)+"/messages", body, &response); err != nil {
		return nil, err
	}
	if len(response.Messages) == 0 || strings.TrimSpace(response.Messages[0].ID) == "" {
		return nil, fmt.Errorf("whatsapp send response did not contain a message id")
	}
	return &SendMessageResponse{MessageID: response.Messages[0].ID}, nil
}

func (c *Client) doJSON(ctx context.Context, method, rawURL string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("whatsapp api %s: %s", resp.Status, whatsappAPIError(responseBody))
	}
	if out == nil || len(responseBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, out); err != nil {
		return fmt.Errorf("decode whatsapp api response: %w", err)
	}
	return nil
}

func (c *Client) resourceURL(resource string) string {
	return fmt.Sprintf("%s/%s/%s", c.baseURL, url.PathEscape(c.version), url.PathEscape(resource))
}

func whatsappAPIError(body []byte) string {
	var response struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err == nil && response.Error.Message != "" {
		if response.Error.Code != 0 {
			return fmt.Sprintf("%s (%d)", response.Error.Message, response.Error.Code)
		}
		return response.Error.Message
	}
	if len(body) == 0 {
		return "empty response"
	}
	return strings.TrimSpace(string(body))
}
