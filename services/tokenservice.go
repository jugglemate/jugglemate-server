package services

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/juggleim/jugglemate-server/commons/appinfos"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"github.com/juggleim/jugglemate-server/services/pbobjs"
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

func GenerateToken(appkey, userId string) (string, error) {
	t := &ImToken{
		AppKey:    appkey,
		UserId:    userId,
		TokenTime: time.Now().UnixMilli(),
	}
	if appinfo, exist := appinfos.GetAppInfo(appkey); exist && appinfo != nil {
		if appinfo.AppSecret == "" {
			return "", fmt.Errorf("app secret is empty: appkey=%s", appkey)
		}
		token, err := t.ToTokenString([]byte(appinfo.AppSecret))
		if err != nil {
			log.Printf("[GenerateToken] failed: appkey=%s userId=%s err=%v", appkey, userId, err)
			return "", err
		}
		return token, nil
	}
	return "", fmt.Errorf("app not found: appkey=%s", appkey)
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
