package security

import (
	"strings"
	"testing"
)

func TestEncryptionServiceRoundTripAndCompatibilityFormat(t *testing.T) {
	service, err := NewEncryptionService("secret", "salt")
	if err != nil {
		t.Fatal(err)
	}
	cipherText, err := service.Encrypt("sk-1234567890abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cipherText, "v1:") {
		t.Fatalf("密文格式不兼容: %s", cipherText)
	}
	plainText, err := service.Decrypt(cipherText)
	if err != nil || plainText != "sk-1234567890abcdef" {
		t.Fatalf("解密结果错误: plain=%q err=%v", plainText, err)
	}
	if got := service.Mask(plainText); got != "sk-12345***def" {
		t.Fatalf("脱敏结果错误: %s", got)
	}
}
