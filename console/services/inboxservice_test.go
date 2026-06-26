package services

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
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

func TestCreateTelegramInboxStoresConfigAndRedactsResponse(t *testing.T) {
	inboxStorage := &fakeInboxStorage{}
	withInboxFakes(t, inboxStorage, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})

	code, resp := CreateTelegramInbox(inboxTestCtx(), &consoleModels.CreateTelegramInboxReq{
		Name:     "Telegram Support",
		BotName:  "support_bot",
		BotToken: "secret-token",
	})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if resp.ID == "" || resp.Name != "Telegram Support" || resp.ChannelType != "telegram" {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.ChannelConf.BotToken != "" {
		t.Fatalf("bot token should be redacted, got %q", resp.ChannelConf.BotToken)
	}
	if inboxStorage.created.AppKey != "app_1" {
		t.Fatalf("created appkey = %q", inboxStorage.created.AppKey)
	}
	if inboxStorage.created.ChannelType != string(appServices.ChannelType_Telegram) {
		t.Fatalf("channel type = %q", inboxStorage.created.ChannelType)
	}
	var conf appServices.TelegramChannelConf
	if err := json.Unmarshal([]byte(inboxStorage.created.ChannelConf), &conf); err != nil {
		t.Fatalf("channel conf json: %v", err)
	}
	if conf.BotName != "support_bot" || conf.BotToken != "secret-token" {
		t.Fatalf("conf = %+v", conf)
	}
}

func TestCreateTelegramInboxRequiresConfig(t *testing.T) {
	withInboxFakes(t, &fakeInboxStorage{}, &fakeInboxMemberStorage{}, &fakeUserStorageForInbox{})
	code, _ := CreateTelegramInbox(inboxTestCtx(), &consoleModels.CreateTelegramInboxReq{Name: "Telegram"})
	if code != errs.IMErrorCode_APP_REQ_BODY_ILLEGAL {
		t.Fatalf("code = %d", code)
	}
}

func TestQryInboxesUsesAppScopeAndRedactsToken(t *testing.T) {
	conf, _ := json.Marshal(appServices.TelegramChannelConf{BotName: "bot", BotToken: "secret"})
	inboxStorage := &fakeInboxStorage{
		list: []*storageModels.Inbox{{
			InboxId:     "inbox_1",
			Name:        "Telegram",
			ChannelType: "telegram",
			ChannelConf: string(conf),
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
	if inboxStorage.qryAppkey != "app_1" || inboxStorage.qryChannelType != "telegram" {
		t.Fatalf("query scope app=%q type=%q", inboxStorage.qryAppkey, inboxStorage.qryChannelType)
	}
	if len(resp.List) != 1 || resp.List[0].MemberCount != 2 {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.List[0].ChannelConf.BotName != "bot" || resp.List[0].ChannelConf.BotToken != "" {
		t.Fatalf("conf = %+v", resp.List[0].ChannelConf)
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
