package imsdk

import (
	"fmt"
	"sync"

	"github.com/juggleim/imserver-sdk-go"
)

var imsdkMap *sync.Map
var imLock *sync.RWMutex

// AppInfoProvider is a function type to get app info (appSecret and imApiDomain)
var AppInfoProvider func(appkey string) (appSecret string, imApiDomain string, ok bool)

func init() {
	imsdkMap = &sync.Map{}
	imLock = &sync.RWMutex{}
}

func GetImSdk(appkey string) *juggleimsdk.JuggleIMSdk {
	if val, exist := imsdkMap.Load(appkey); exist {
		return val.(*juggleimsdk.JuggleIMSdk)
	}

	imLock.Lock()
	defer imLock.Unlock()

	if val, exist := imsdkMap.Load(appkey); exist {
		return val.(*juggleimsdk.JuggleIMSdk)
	}

	// Use provider to get app info if available
	if AppInfoProvider != nil {
		if appSecret, imApiDomain, ok := AppInfoProvider(appkey); ok {
			sdk := juggleimsdk.NewJuggleIMSdk(appkey, appSecret, imApiDomain)
			imsdkMap.Store(appkey, sdk)
			return sdk
		}
	}

	return nil
}

// RegisterAppInfoProvider registers a function to provide app info (appSecret, imApiDomain, ok)
func RegisterAppInfoProvider(provider func(appkey string) (appSecret string, imApiDomain string, ok bool)) {
	AppInfoProvider = provider
}

// GetServerInfo returns debug info about the SDK configuration
func GetServerInfo(appkey string) string {
	if provider := AppInfoProvider; provider != nil {
		if secret, domain, ok := provider(appkey); ok {
			return fmt.Sprintf("domain=%s, secret=%s***", domain, secret[:8])
		}
	}
	return "no provider or not found"
}