// Package security 提供 Agent 模块的敏感数据保护能力。
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const cipherVersion = "v1"

// EncryptionService 使用 AES-256-GCM 加密 LLM、工具等外部凭据。
type EncryptionService struct {
	aead cipher.AEAD
}

// NewEncryptionService 创建与 Python 源服务密文格式兼容的加密服务。
func NewEncryptionService(secretKey string, secretSalt string) (*EncryptionService, error) {
	secretKey = strings.TrimSpace(secretKey)
	secretSalt = strings.TrimSpace(secretSalt)
	if secretKey == "" || secretSalt == "" {
		return nil, errors.New("Agent 加密配置缺失：secretKey 或 secretSalt 为空")
	}
	key := sha256.Sum256([]byte(secretKey + ":" + secretSalt))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("创建 AES 密钥失败: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 AES-GCM 失败: %w", err)
	}
	return &EncryptionService{aead: aead}, nil
}

// Encrypt 将明文加密为 `v1:nonce:ciphertext` 格式。
func (service *EncryptionService) Encrypt(plainText string) (string, error) {
	if service == nil || service.aead == nil {
		return "", errors.New("加密服务未初始化")
	}
	nonce := make([]byte, service.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成加密随机数失败: %w", err)
	}
	cipherText := service.aead.Seal(nil, nonce, []byte(plainText), nil)
	return strings.Join([]string{
		cipherVersion,
		base64.URLEncoding.EncodeToString(nonce),
		base64.URLEncoding.EncodeToString(cipherText),
	}, ":"), nil
}

// Decrypt 解密 `v1:nonce:ciphertext` 格式的密文。
func (service *EncryptionService) Decrypt(value string) (string, error) {
	if service == nil || service.aead == nil {
		return "", errors.New("加密服务未初始化")
	}
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 || parts[0] != cipherVersion {
		return "", errors.New("密文格式或版本不受支持")
	}
	nonce, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("密文随机数格式非法")
	}
	cipherText, err := base64.URLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", errors.New("密文内容格式非法")
	}
	plain, err := service.aead.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", errors.New("密文解密失败")
	}
	return string(plain), nil
}

// Mask 按源服务规则返回 API Key 脱敏值。
func (service *EncryptionService) Mask(plainText string) string {
	// 简要描述：不足 12 位时不保留任何片段，避免短密钥被逆推出完整内容。
	if len(plainText) <= 11 {
		return "***"
	}
	return plainText[:8] + "***" + plainText[len(plainText)-3:]
}
