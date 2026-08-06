package services

import (
	"encoding/json"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/errs"
	whatsappapi "github.com/juggleim/jugglemate-server/commons/whatsapp"
	consoleModels "github.com/juggleim/jugglemate-server/console/apis/models"
	appServices "github.com/juggleim/jugglemate-server/services"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestCreateWhatsAppInboxValidatesAndRedactsCredentials(t *testing.T) {
	inboxStorage := &fakeInboxStorage{}
	withInboxFakes(t, inboxStorage, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})
	oldGetPhoneInfo := whatsappGetPhoneInfoForConsole
	oldConfig := configures.Config
	configures.Config.JmateBaseUrl = "https://jmate.example.com/"
	whatsappGetPhoneInfoForConsole = func(accessToken, phoneNumberID, apiBaseURL, apiVersion string) (*whatsappapi.PhoneInfo, error) {
		if accessToken != "secret-token" || phoneNumberID != "phone_1" || apiBaseURL != whatsappapi.DefaultAPIBaseURL || apiVersion != whatsappapi.DefaultAPIVersion {
			t.Fatalf("validation args token=%q phone=%q base=%q version=%q", accessToken, phoneNumberID, apiBaseURL, apiVersion)
		}
		return &whatsappapi.PhoneInfo{DisplayPhoneNumber: "+15551234567", VerifiedName: "Support"}, nil
	}
	t.Cleanup(func() {
		whatsappGetPhoneInfoForConsole = oldGetPhoneInfo
		configures.Config = oldConfig
	})

	code, resp := CreateWhatsAppInbox(inboxTestCtx(), &consoleModels.CreateWhatsAppInboxReq{
		Name:               "WhatsApp Support",
		PhoneNumberID:      "phone_1",
		AccessToken:        "secret-token",
		WebhookVerifyToken: "verify-token",
	})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	conf, ok := resp.ChannelConf.(consoleModels.WhatsAppConfigItem)
	if !ok {
		t.Fatalf("channel conf type = %T", resp.ChannelConf)
	}
	if conf.PhoneNumberID != "phone_1" || conf.DisplayPhoneNumber != "+15551234567" || conf.VerifiedName != "Support" {
		t.Fatalf("response conf = %+v", conf)
	}
	raw := appServices.ParseWhatsAppChannelConf(inboxStorage.created.ChannelConf)
	if raw.AccessToken != "secret-token" || raw.WebhookVerifyToken != "verify-token" {
		t.Fatalf("stored secret conf = %+v", raw)
	}
	if raw.PhoneNumberID != "phone_1" || raw.DisplayPhoneNumber != "+15551234567" {
		t.Fatalf("stored conf = %+v", raw)
	}
	if _, err := json.Marshal(resp); err != nil {
		t.Fatalf("response json: %v", err)
	}
}

var _ storageModels.IInboxStorage = (*fakeInboxStorage)(nil)
