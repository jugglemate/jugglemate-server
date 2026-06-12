package services

import (
	"encoding/base64"
	"errors"
	"time"

	"github.com/juggleim/jugglechat-server-ai/commons/tools"
	"github.com/juggleim/jugglechat-server-ai/services/pbobjs"
)

type ImToken struct {
	AppKey    string
	UserId    string
	DeviceId  string
	TokenTime int64
}

func (t ImToken) ToTokenString(secureKey []byte) (string, error) {
	tokenValue := &pbobjs.AuthTokenValue{
		UserId:    t.UserId,
		DeviceId:  t.DeviceId,
		TokenTime: t.TokenTime,
	}
	tokenBs, err := tools.JsonMarshal(tokenValue)
	if err == nil {
		encryptToken, err := encrypt(tokenBs, secureKey)
		if err == nil {
			tokenWrap := &pbobjs.AuthToken{
				Appkey:     t.AppKey,
				TokenValue: encryptToken,
			}
			tokenWrapBs, err := tools.JsonMarshal(tokenWrap)
			if err == nil {
				return base64.URLEncoding.EncodeToString(tokenWrapBs), nil
			}
			return "", err
		}
		return "", err
	}
	return "", err
}

func GenerateToken(appkey, userId string, secureKey string) string {
	token := ""
	t := &ImToken{
		AppKey:    appkey,
		UserId:    userId,
		TokenTime: time.Now().UnixMilli(),
	}
	if secureKey != "" {
		token, _ = t.ToTokenString([]byte(secureKey))
	}
	return token
}

func encrypt(dataBs, secureKeyBs []byte) ([]byte, error) {
	return tools.AesEncrypt(dataBs, secureKeyBs)
}

func decrypt(cryptedData, secureKeyBs []byte) ([]byte, error) {
	return tools.AesDecrypt(cryptedData, secureKeyBs)
}

func ParseTokenString(tokenStr string) (*pbobjs.AuthToken, error) {
	tokenWrap := &pbobjs.AuthToken{}
	tokenWrapBs, err := base64.URLEncoding.DecodeString(tokenStr)
	if err != nil {
		tokenWrapBs, err = base64.StdEncoding.DecodeString(tokenStr)
	}
	if err == nil {
		err = tools.JsonUnMarshal(tokenWrapBs, tokenWrap)
	}
	return tokenWrap, err
}

func ParseToken(tokenWrap *pbobjs.AuthToken, secureKey []byte) (ImToken, error) {
	token := ImToken{
		AppKey: tokenWrap.Appkey,
	}
	cryptedToken := tokenWrap.TokenValue
	tokenBs, err := decrypt(cryptedToken, secureKey)
	if err == nil {
		tokenValue := &pbobjs.AuthTokenValue{}
		err = tools.JsonUnMarshal(tokenBs, tokenValue)
		if err != nil {
			return token, err
		}
		if tokenValue.UserId == "" {
			return token, errors.New("invalid token")
		}
		token.UserId = tokenValue.UserId
		token.DeviceId = tokenValue.DeviceId
		token.TokenTime = tokenValue.TokenTime
	}
	return token, err
}

func CheckApiKey(apiKey string, appkey, secureKey string) bool {
	bs, err := base64.URLEncoding.DecodeString(apiKey)
	if err != nil {
		return false
	}
	decodedBs, err := tools.AesDecrypt(bs, []byte(secureKey))
	if err != nil {
		return false
	}
	var apikey pbobjs.ApiKey
	err = tools.JsonUnMarshal(decodedBs, &apikey)
	if err != nil {
		return false
	}
	if apikey.Appkey != appkey {
		return false
	}
	return true
}

func GenerateApiKey(appkey, secureKey string) (string, error) {
	apikey := &pbobjs.ApiKey{
		Appkey:      appkey,
		CreatedTime: time.Now().UnixMilli(),
	}
	bs, _ := tools.JsonMarshal(apikey)
	encodedBs, err := tools.AesEncrypt(bs, []byte(secureKey))
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(encodedBs), nil
}