package services

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	telegramapi "github.com/juggleim/jugglemate-server/commons/telegram"
	consoleModels "github.com/juggleim/jugglemate-server/console/apis/models"
	appServices "github.com/juggleim/jugglemate-server/services"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func inboxTestCtx() context.Context {
	return context.WithValue(context.Background(), ctxs.CtxKey_AppKey, "app_1")
}

func withInboxFakes(t *testing.T, inbox *fakeInboxStorage, members *fakeInboxMemberStorage, users *fakeUserStorageForInbox) {
	t.Helper()
	oldInbox := newInboxStorageForConsole
	oldMembers := newInboxMemberStorageForConsole
	oldUsers := newUserStorageForConsoleInbox
	newInboxStorageForConsole = func() storageModels.IInboxStorage { return inbox }
	newInboxMemberStorageForConsole = func() storageModels.IInboxMemberStorage { return members }
	newUserStorageForConsoleInbox = func() storageModels.IUserStorage { return users }
	t.Cleanup(func() {
		newInboxStorageForConsole = oldInbox
		newInboxMemberStorageForConsole = oldMembers
		newUserStorageForConsoleInbox = oldUsers
	})
}

func withTelegramSetupFakes(t *testing.T) *fakeTelegramSetup {
	t.Helper()
	fake := &fakeTelegramSetup{botInfo: &telegramapi.BotInfo{Username: "support_bot"}}
	oldGetMe := telegramGetMeForConsole
	oldDelete := telegramDeleteWebhookForConsole
	oldSet := telegramSetWebhookForConsole
	oldConfig := configures.Config
	configures.Config.JmateBaseUrl = "https://jmate.example.com/"
	telegramGetMeForConsole = func(botToken string) (*telegramapi.BotInfo, error) {
		fake.getMeToken = botToken
		return fake.botInfo, fake.getMeErr
	}
	telegramDeleteWebhookForConsole = func(botToken string) error {
		fake.deleteToken = botToken
		return fake.deleteErr
	}
	telegramSetWebhookForConsole = func(botToken, callbackURL string) error {
		fake.setToken = botToken
		fake.callbackURL = callbackURL
		return fake.setErr
	}
	t.Cleanup(func() {
		telegramGetMeForConsole = oldGetMe
		telegramDeleteWebhookForConsole = oldDelete
		telegramSetWebhookForConsole = oldSet
		configures.Config = oldConfig
	})
	return fake
}

func TestCreateTelegramInboxStoresConfigAndRedactsResponse(t *testing.T) {
	inboxStorage := &fakeInboxStorage{}
	withInboxFakes(t, inboxStorage, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})
	telegramSetup := withTelegramSetupFakes(t)

	code, resp := CreateTelegramInbox(inboxTestCtx(), &consoleModels.CreateTelegramInboxReq{
		Name:     "Telegram Support",
		BotName:  "manual_bot",
		BotToken: "secret-token",
	})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if resp.ID == "" || resp.Name != "Telegram Support" || resp.ChannelType != "telegram" {
		t.Fatalf("resp = %+v", resp)
	}
	conf, ok := resp.ChannelConf.(consoleModels.TelegramConfigItem)
	if !ok {
		t.Fatalf("channel conf type = %T", resp.ChannelConf)
	}
	if conf.BotToken != "" {
		t.Fatalf("bot token should be redacted, got %q", conf.BotToken)
	}
	if inboxStorage.created.AppKey != "app_1" {
		t.Fatalf("created appkey = %q", inboxStorage.created.AppKey)
	}
	if inboxStorage.created.ChannelType != string(appServices.ChannelType_Telegram) {
		t.Fatalf("channel type = %q", inboxStorage.created.ChannelType)
	}
	var storedConf appServices.TelegramChannelConf
	if err := json.Unmarshal([]byte(inboxStorage.created.ChannelConf), &storedConf); err != nil {
		t.Fatalf("channel conf json: %v", err)
	}
	if storedConf.BotName != "support_bot" || storedConf.BotToken != "secret-token" {
		t.Fatalf("conf = %+v", storedConf)
	}
	if telegramSetup.getMeToken != "secret-token" || telegramSetup.deleteToken != "secret-token" || telegramSetup.setToken != "secret-token" {
		t.Fatalf("telegram tokens = getMe:%q delete:%q set:%q", telegramSetup.getMeToken, telegramSetup.deleteToken, telegramSetup.setToken)
	}
	wantCallback := "https://jmate.example.com/jmate/webhooks/telegram/" + resp.ID
	if telegramSetup.callbackURL != wantCallback {
		t.Fatalf("callbackURL = %q, want %q", telegramSetup.callbackURL, wantCallback)
	}
}

func TestCreateTelegramInboxRequiresConfig(t *testing.T) {
	withInboxFakes(t, &fakeInboxStorage{}, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})
	withTelegramSetupFakes(t)
	code, _ := CreateTelegramInbox(inboxTestCtx(), &consoleModels.CreateTelegramInboxReq{Name: "Telegram"})
	if code != errs.IMErrorCode_APP_REQ_BODY_ILLEGAL {
		t.Fatalf("code = %d", code)
	}
}

func TestCreateTelegramInboxRejectsInvalidBotToken(t *testing.T) {
	inboxStorage := &fakeInboxStorage{}
	withInboxFakes(t, inboxStorage, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})
	telegramSetup := withTelegramSetupFakes(t)
	telegramSetup.getMeErr = errors.New("invalid token")

	code, _ := CreateTelegramInbox(inboxTestCtx(), &consoleModels.CreateTelegramInboxReq{
		Name:     "Telegram",
		BotName:  "bot",
		BotToken: "bad-token",
	})
	if code != errs.IMErrorCode_APP_REQ_BODY_ILLEGAL {
		t.Fatalf("code = %d", code)
	}
	if inboxStorage.created.InboxId != "" {
		t.Fatalf("inbox should not be created: %+v", inboxStorage.created)
	}
}

func TestCreateTelegramInboxRejectsMissingBaseURL(t *testing.T) {
	inboxStorage := &fakeInboxStorage{}
	withInboxFakes(t, inboxStorage, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})
	withTelegramSetupFakes(t)
	configures.Config.JmateBaseUrl = ""

	code, _ := CreateTelegramInbox(inboxTestCtx(), &consoleModels.CreateTelegramInboxReq{
		Name:     "Telegram",
		BotName:  "bot",
		BotToken: "secret",
	})
	if code != errs.IMErrorCode_APP_REQ_BODY_ILLEGAL {
		t.Fatalf("code = %d", code)
	}
	if inboxStorage.created.InboxId != "" {
		t.Fatalf("inbox should not be created: %+v", inboxStorage.created)
	}
}

func TestCreateTelegramInboxRejectsWebhookSetupFailure(t *testing.T) {
	inboxStorage := &fakeInboxStorage{}
	withInboxFakes(t, inboxStorage, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})
	telegramSetup := withTelegramSetupFakes(t)
	telegramSetup.setErr = errors.New("webhook failed")

	code, _ := CreateTelegramInbox(inboxTestCtx(), &consoleModels.CreateTelegramInboxReq{
		Name:     "Telegram",
		BotName:  "bot",
		BotToken: "secret",
	})
	if code != errs.IMErrorCode_APP_INTERNAL_TIMEOUT {
		t.Fatalf("code = %d", code)
	}
	if inboxStorage.created.InboxId != "" {
		t.Fatalf("inbox should not be created: %+v", inboxStorage.created)
	}
}

func TestQryInboxesUsesAppScopeAndRedactsToken(t *testing.T) {
	storedConf, _ := json.Marshal(appServices.TelegramChannelConf{BotName: "bot", BotToken: "secret"})
	inboxStorage := &fakeInboxStorage{
		list: []*storageModels.Inbox{{
			InboxId:     "inbox_1",
			Name:        "Telegram",
			ChannelType: "telegram",
			ChannelConf: string(storedConf),
			AppKey:      "app_1",
		}},
		total: 1,
	}
	memberStorage := &fakeInboxMemberStorage{counts: map[string]int64{"inbox_1": 2}}
	withInboxFakes(t, inboxStorage, memberStorage, &fakeUserStorageForInbox{})

	code, resp := QryInboxes(inboxTestCtx(), 20, 0)
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if inboxStorage.qryAppkey != "app_1" || inboxStorage.qryChannelType != "" {
		t.Fatalf("query scope app=%q type=%q", inboxStorage.qryAppkey, inboxStorage.qryChannelType)
	}
	if len(resp.List) != 1 || resp.List[0].MemberCount != 2 {
		t.Fatalf("resp = %+v", resp)
	}
	telegramConf, ok := resp.List[0].ChannelConf.(consoleModels.TelegramConfigItem)
	if !ok {
		t.Fatalf("channel conf type = %T", resp.List[0].ChannelConf)
	}
	if telegramConf.BotName != "bot" || telegramConf.BotToken != "" {
		t.Fatalf("conf = %+v", telegramConf)
	}
}

func TestCreateWidgetInboxStoresWelcomeMessage(t *testing.T) {
	inboxStorage := &fakeInboxStorage{}
	withInboxFakes(t, inboxStorage, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})

	code, resp := CreateWidgetInbox(inboxTestCtx(), &consoleModels.CreateWidgetInboxReq{
		Name:           "Website Support",
		WelcomeMessage: "Hello there",
	})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if resp.ChannelType != "widget" {
		t.Fatalf("channel type = %q", resp.ChannelType)
	}
	widgetConf, ok := resp.ChannelConf.(consoleModels.WidgetConfigItem)
	if !ok || widgetConf.WelcomeMessage != "Hello there" {
		t.Fatalf("widget conf = %+v", resp.ChannelConf)
	}
	var stored appServices.WebWidgetChannelConf
	if err := json.Unmarshal([]byte(inboxStorage.created.ChannelConf), &stored); err != nil {
		t.Fatalf("channel conf json: %v", err)
	}
	if stored.WelcomeMessage != "Hello there" {
		t.Fatalf("stored conf = %+v", stored)
	}
}

func TestCreateWidgetInboxAllowsEmptyWelcomeMessage(t *testing.T) {
	inboxStorage := &fakeInboxStorage{}
	withInboxFakes(t, inboxStorage, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})

	code, resp := CreateWidgetInbox(inboxTestCtx(), &consoleModels.CreateWidgetInboxReq{Name: "Website"})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	widgetConf, ok := resp.ChannelConf.(consoleModels.WidgetConfigItem)
	if !ok || widgetConf.WelcomeMessage != "" {
		t.Fatalf("widget conf = %+v", resp.ChannelConf)
	}
}

func TestCreateWidgetInboxRequiresName(t *testing.T) {
	withInboxFakes(t, &fakeInboxStorage{}, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})
	code, _ := CreateWidgetInbox(inboxTestCtx(), &consoleModels.CreateWidgetInboxReq{})
	if code != errs.IMErrorCode_APP_REQ_BODY_ILLEGAL {
		t.Fatalf("code = %d", code)
	}
}

func TestQryInboxesIncludesWidgetRows(t *testing.T) {
	widgetConf, _ := json.Marshal(appServices.WebWidgetChannelConf{WelcomeMessage: "Hi"})
	inboxStorage := &fakeInboxStorage{
		list: []*storageModels.Inbox{
			{
				InboxId:     "inbox_widget",
				Name:        "Website",
				ChannelType: "widget",
				ChannelConf: string(widgetConf),
				AppKey:      "app_1",
			},
		},
		total: 1,
	}
	withInboxFakes(t, inboxStorage, &fakeInboxMemberStorage{counts: map[string]int64{}}, &fakeUserStorageForInbox{})

	code, resp := QryInboxes(inboxTestCtx(), 20, 0)
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if len(resp.List) != 1 || resp.List[0].ChannelType != "widget" {
		t.Fatalf("resp = %+v", resp)
	}
	widgetItem, ok := resp.List[0].ChannelConf.(consoleModels.WidgetConfigItem)
	if !ok || widgetItem.WelcomeMessage != "Hi" {
		t.Fatalf("widget conf = %+v", resp.List[0].ChannelConf)
	}
}

func TestReplaceInboxMembersWorksForWidgetInbox(t *testing.T) {
	inboxStorage := &fakeInboxStorage{byId: map[string]*storageModels.Inbox{
		"inbox_widget": {InboxId: "inbox_widget", ChannelType: "widget", AppKey: "app_1"},
	}}
	memberStorage := &fakeInboxMemberStorage{}
	userStorage := &fakeUserStorageForInbox{users: map[string]*storageModels.User{
		"u_1": {UserId: "u_1", AppKey: "app_1"},
	}}
	withInboxFakes(t, inboxStorage, memberStorage, userStorage)

	code := ReplaceInboxMembers(inboxTestCtx(), "inbox_widget", &consoleModels.ReplaceInboxMembersReq{UserIds: []string{"u_1"}})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if !reflect.DeepEqual(memberStorage.replaced, []string{"u_1"}) {
		t.Fatalf("replaced = %v", memberStorage.replaced)
	}
}

func TestReplaceInboxMembersValidatesUsersAndReplaces(t *testing.T) {
	inboxStorage := &fakeInboxStorage{byId: map[string]*storageModels.Inbox{
		"inbox_1": {InboxId: "inbox_1", ChannelType: "telegram", AppKey: "app_1"},
	}}
	memberStorage := &fakeInboxMemberStorage{}
	userStorage := &fakeUserStorageForInbox{users: map[string]*storageModels.User{
		"u_1": {UserId: "u_1", AppKey: "app_1"},
		"u_2": {UserId: "u_2", AppKey: "app_1"},
	}}
	withInboxFakes(t, inboxStorage, memberStorage, userStorage)

	code := ReplaceInboxMembers(inboxTestCtx(), "inbox_1", &consoleModels.ReplaceInboxMembersReq{
		UserIds: []string{"u_1", "u_2", "u_1"},
	})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if memberStorage.replaceAppkey != "app_1" || memberStorage.replaceInboxId != "inbox_1" {
		t.Fatalf("replace scope app=%q inbox=%q", memberStorage.replaceAppkey, memberStorage.replaceInboxId)
	}
	if !reflect.DeepEqual(memberStorage.replaced, []string{"u_1", "u_2"}) {
		t.Fatalf("replaced = %v", memberStorage.replaced)
	}
}

func TestReplaceInboxMembersRejectsUnknownUser(t *testing.T) {
	inboxStorage := &fakeInboxStorage{byId: map[string]*storageModels.Inbox{
		"inbox_1": {InboxId: "inbox_1", ChannelType: "telegram", AppKey: "app_1"},
	}}
	withInboxFakes(t, inboxStorage, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{users: map[string]*storageModels.User{}})

	code := ReplaceInboxMembers(inboxTestCtx(), "inbox_1", &consoleModels.ReplaceInboxMembersReq{UserIds: []string{"missing"}})
	if code != errs.IMErrorCode_APP_ParamError {
		t.Fatalf("code = %d", code)
	}
}

type fakeInboxStorage struct {
	created        storageModels.Inbox
	list           []*storageModels.Inbox
	total          int64
	byId           map[string]*storageModels.Inbox
	qryAppkey      string
	qryChannelType string
}

func (s *fakeInboxStorage) Create(item storageModels.Inbox) error {
	s.created = item
	return nil
}
func (s *fakeInboxStorage) Update(item storageModels.Inbox) error { return nil }
func (s *fakeInboxStorage) Upsert(item storageModels.Inbox) error { return nil }
func (s *fakeInboxStorage) Delete(appkey, inboxId string) error   { return nil }
func (s *fakeInboxStorage) FindByInboxId(appkey, inboxId string) (*storageModels.Inbox, error) {
	if s.byId == nil {
		return nil, nil
	}
	return s.byId[inboxId], nil
}
func (s *fakeInboxStorage) FindByInboxIdAny(inboxId string) ([]*storageModels.Inbox, error) {
	if s.byId == nil {
		return nil, nil
	}
	if inbox, ok := s.byId[inboxId]; ok {
		return []*storageModels.Inbox{inbox}, nil
	}
	return nil, nil
}
func (s *fakeInboxStorage) QryByApp(appkey, channelType string, limit, offset int64) (*storageModels.InboxListResult, error) {
	s.qryAppkey = appkey
	s.qryChannelType = channelType
	return &storageModels.InboxListResult{List: s.list, Total: s.total}, nil
}
func (s *fakeInboxStorage) QryByChannelType(appkey, channelType string, startId, limit int64) ([]*storageModels.Inbox, error) {
	return s.list, nil
}

type fakeInboxMemberStorage struct {
	counts         map[string]int64
	replaceAppkey  string
	replaceInboxId string
	replaced       []string
}

func (s *fakeInboxMemberStorage) Create(item storageModels.InboxMember) error { return nil }
func (s *fakeInboxMemberStorage) Upsert(item storageModels.InboxMember) error { return nil }
func (s *fakeInboxMemberStorage) Delete(appkey, inboxId, memberId string) error {
	return nil
}
func (s *fakeInboxMemberStorage) Find(appkey, inboxId, memberId string) (*storageModels.InboxMember, error) {
	return nil, nil
}
func (s *fakeInboxMemberStorage) QryByInbox(appkey, inboxId string, startId, limit int64) ([]*storageModels.InboxMember, error) {
	return nil, nil
}
func (s *fakeInboxMemberStorage) QryByMember(appkey, memberId string, startId, limit int64) ([]*storageModels.InboxMember, error) {
	return nil, nil
}
func (s *fakeInboxMemberStorage) CountByInboxes(appkey string, inboxIds []string) (map[string]int64, error) {
	return s.counts, nil
}
func (s *fakeInboxMemberStorage) ReplaceByInbox(appkey, inboxId string, memberIds []string) error {
	s.replaceAppkey = appkey
	s.replaceInboxId = inboxId
	s.replaced = append([]string{}, memberIds...)
	return nil
}

type fakeUserStorageForInbox struct {
	users map[string]*storageModels.User
}

type fakeTelegramSetup struct {
	botInfo *telegramapi.BotInfo

	getMeToken  string
	deleteToken string
	setToken    string
	callbackURL string

	getMeErr  error
	deleteErr error
	setErr    error
}

func (s *fakeUserStorageForInbox) Create(item storageModels.User) error { return nil }
func (s *fakeUserStorageForInbox) FindByAccount(appkey, account string) (*storageModels.User, error) {
	return nil, nil
}
func (s *fakeUserStorageForInbox) FindByUserId(appkey, userId string) (*storageModels.User, error) {
	if s.users == nil {
		return nil, nil
	}
	return s.users[userId], nil
}
func (s *fakeUserStorageForInbox) FindByAccountWithAppkey(account, appkey string) (*storageModels.User, error) {
	return nil, nil
}
func (s *fakeUserStorageForInbox) UpdateImToken(appkey, userId, imToken string) error {
	return nil
}
func (s *fakeUserStorageForInbox) QryByApp(appkey string, filter storageModels.UserListFilter, limit, offset int64) (*storageModels.UserListResult, error) {
	return nil, nil
}
func (s *fakeUserStorageForInbox) UpdateUser(appkey, userId string, updates storageModels.UserUpdate) error {
	return nil
}
func (s *fakeUserStorageForInbox) Delete(appkey, userId string) error { return nil }
