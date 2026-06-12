package pbobjs

import (
	"encoding/base64"
	"time"

	"github.com/juggleim/jugglechat-server-ai/commons/tools"
)

type ApiKey struct {
	Appkey      string `json:"appkey"`
	CreatedTime int64  `json:"created_time"`
}

type AuthToken struct {
	Appkey     string `json:"appkey"`
	Version    int32  `json:"version"`
	TokenValue []byte `json:"tokenValue"`
}

type AuthTokenValue struct {
	UserId    string `json:"userId"`
	DeviceId  string `json:"deviceId"`
	TokenTime int64  `json:"tokenTime"`
}

// ToTokenString generates a token string from AuthTokenValue
func (t AuthTokenValue) ToTokenString(secureKey []byte) (string, error) {
	tokenBs, err := tools.JsonMarshal(t)
	if err != nil {
		return "", err
	}
	encrypted, err := tools.AesEncrypt(tokenBs, secureKey)
	if err != nil {
		return "", err
	}
	wrap := AuthToken{
		TokenValue: encrypted,
	}
	wrapBs, err := tools.JsonMarshal(wrap)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(wrapBs), nil
}

// FromTokenString parses a token string into AuthTokenValue
func (t *AuthTokenValue) FromTokenString(tokenStr string, secureKey []byte) error {
	tokenWrapBs, err := base64.URLEncoding.DecodeString(tokenStr)
	if err != nil {
		return err
	}
	var wrap AuthToken
	if err := tools.JsonUnMarshal(tokenWrapBs, &wrap); err != nil {
		return err
	}
	decrypted, err := tools.AesDecrypt(wrap.TokenValue, secureKey)
	if err != nil {
		return err
	}
	return tools.JsonUnMarshal(decrypted, t)
}

// NewApiKey creates a new ApiKey
func NewApiKey(appkey string, createdTime int64) *ApiKey {
	return &ApiKey{
		Appkey:      appkey,
		CreatedTime: createdTime,
	}
}

// ToBase64 encodes ApiKey to base64 string with AES encryption
func (k *ApiKey) ToBase64(secureKey string) (string, error) {
	bs, err := tools.JsonMarshal(k)
	if err != nil {
		return "", err
	}
	encrypted, err := tools.AesEncrypt(bs, []byte(secureKey))
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(encrypted), nil
}

// FromBase64 decodes base64 string to ApiKey
func (k *ApiKey) FromBase64(data string, secureKey string) error {
	bs, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		return err
	}
	decrypted, err := tools.AesDecrypt(bs, []byte(secureKey))
	if err != nil {
		return err
	}
	return tools.JsonUnMarshal(decrypted, k)
}

// NewToken creates a new AuthTokenValue
func NewToken(userId, deviceId string) *AuthTokenValue {
	return &AuthTokenValue{
		UserId:    userId,
		DeviceId:  deviceId,
		TokenTime: time.Now().UnixMilli(),
	}
}