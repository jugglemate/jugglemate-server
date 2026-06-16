package qiniu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultUploadHost = "https://upload.qiniup.com"
)

// TokenResponse maps to the JuggleIM upload token HTTP API response.
type TokenResponse struct {
	Code int `json:"code"`
	Data struct {
		OssType   int `json:"oss_type"`
		QiniuResp struct {
			Domain string `json:"domain"`
			Token  string `json:"token"`
		} `json:"qiniu_resp"`
	} `json:"data"`
}

// QiniuResp is the raw qiniu upload response.
type qiniuUploadResp struct {
	Key string `json:"key"`
}

// Uploader is a unified qiniu upload client that first fetches an upload
// token from the JuggleIM backend, then uploads the file to Qiniu CDN.
type Uploader struct {
	tokenAPI   string
	uploadHost string
	appkey     string
	appSecret  string
	httpClient *http.Client
}

// NewUploader creates a new Uploader.
// tokenAPI is the full URL of the JuggleIM file credential endpoint
// (e.g. "https://api.juggle.im/jim/file_cred").
// appkey and appSecret are used for authentication with the JuggleIM API gateway.
func NewUploader(tokenAPI, appkey, appSecret string) *Uploader {
	return &Uploader{
		tokenAPI:   tokenAPI,
		uploadHost: defaultUploadHost,
		appkey:     appkey,
		appSecret:  appSecret,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Upload reads the file from reader, obtains a qiniu upload token from the
// JuggleIM backend, uploads the file to Qiniu, and returns the final CDN URL.
//
// Parameters:
//   - ctx: context for cancellation
//   - reader: the file content source (will be consumed)
//   - size: file size in bytes (-1 if unknown)
//   - prefix: key prefix like "avatars/" or "materials/"
//   - filename: the file name used as the object key suffix
func (u *Uploader) Upload(ctx context.Context, reader io.Reader, size int64, prefix, filename string) (string, error) {
	// Step 1: Get upload token from JuggleIM backend
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	fileType := extToFileType(ext)
	token, domain, err := u.getUploadToken(ctx, ext, fileType)
	if err != nil {
		return "", fmt.Errorf("get upload token: %w", err)
	}

	// Step 2: Upload to Qiniu
	key := prefix + filename
	cdnURL, err := u.uploadToQiniu(ctx, reader, size, key, token, domain)
	if err != nil {
		return "", fmt.Errorf("upload to qiniu: %w", err)
	}

	return cdnURL, nil
}

// getUploadToken calls the JuggleIM jim/file_cred HTTP API.
func (u *Uploader) getUploadToken(ctx context.Context, ext, fileType string) (token, domain string, err error) {
	reqBody := map[string]interface{}{
		"file_type": fileType,
		"ext":       ext,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	log.Printf("[qiniu] requesting token from %s (appkey=%s)", u.tokenAPI, u.appkey)

	req, err := http.NewRequestWithContext(ctx, "POST", u.tokenAPI, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-appkey", u.appkey)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read token response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[qiniu] token API HTTP %d, body: %s", resp.StatusCode, string(respBody))
		return "", "", fmt.Errorf("token API returned HTTP %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		log.Printf("[qiniu] token API response parse failed, body: %s", string(respBody))
		return "", "", fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.Code != 0 {
		return "", "", fmt.Errorf("token API returned code %d", tokenResp.Code)
	}

	return tokenResp.Data.QiniuResp.Token, tokenResp.Data.QiniuResp.Domain, nil
}

// uploadToQiniu performs a multipart form upload to the Qiniu upload host.
func (u *Uploader) uploadToQiniu(ctx context.Context, reader io.Reader, size int64, key, token, domain string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("token", token); err != nil {
		return "", err
	}
	if err := w.WriteField("key", key); err != nil {
		return "", err
	}

	part, err := w.CreateFormFile("file", filepath.Base(key))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, reader); err != nil {
		return "", err
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", u.uploadHost, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("qiniu upload returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var uploadResp qiniuUploadResp
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return "", fmt.Errorf("parse qiniu upload response: %w", err)
	}

	// Construct CDN URL: https://{domain}/{key}
	cdnURL := fmt.Sprintf("https://%s/%s", domain, uploadResp.Key)
	return cdnURL, nil
}

// extToFileType maps file extension to the file_type expected by JuggleIM jim/file_cred API.
func extToFileType(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "svg", "ico":
		return "image"
	case "mp4", "avi", "mov", "mkv", "webm", "flv", "wmv":
		return "video"
	case "mp3", "wav", "aac", "ogg", "flac", "wma":
		return "audio"
	default:
		return "file"
	}
}
