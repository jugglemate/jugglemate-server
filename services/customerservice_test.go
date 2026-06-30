package services

import (
	"strings"
	"testing"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestValidateWidgetInbox(t *testing.T) {
	tests := []struct {
		name  string
		inbox *storageModels.Inbox
		want  errs.IMErrorCode
	}{
		{
			name:  "missing inbox",
			inbox: nil,
			want:  errs.IMErrorCode_APP_CHANNEL_NOT_EXIST,
		},
		{
			name: "telegram inbox rejected",
			inbox: &storageModels.Inbox{
				InboxId:     "inbox_1",
				ChannelType: string(ChannelType_Telegram),
			},
			want: errs.IMErrorCode_APP_CHANNEL_NOT_EXIST,
		},
		{
			name: "widget inbox accepted",
			inbox: &storageModels.Inbox{
				InboxId:     "inbox_widget_1",
				ChannelType: string(ChannelType_Widget),
			},
			want: errs.IMErrorCode_SUCCESS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateWidgetInbox(tt.inbox); got != tt.want {
				t.Fatalf("validateWidgetInbox() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildTicketGroupMemberIds(t *testing.T) {
	got := buildTicketGroupMemberIds("customer_1", []string{"u_1", "u_2", "u_1", "", "customer_1"})
	want := []string{"customer_1", "u_1", "u_2"}
	if len(got) != len(want) {
		t.Fatalf("member ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("member ids = %v, want %v", got, want)
		}
	}
}

func TestGenerateTicketIdUsesTicketPrefix(t *testing.T) {
	ticketId := generateTicketId()
	if !strings.HasPrefix(ticketId, TicketIDPrefix) {
		t.Fatalf("ticket id = %q, want prefix %q", ticketId, TicketIDPrefix)
	}
	if len(ticketId) <= len(TicketIDPrefix) {
		t.Fatalf("ticket id = %q, want random suffix", ticketId)
	}
}

func TestPrepareTicketGroupMemberIdsRegistersInboxMembers(t *testing.T) {
	memberStorage := &customerInboxMemberStorage{
		members: []*storageModels.InboxMember{
			{MemberId: "u_1"},
			{MemberId: "u_2"},
		},
	}
	userStorage := &customerUserStorage{
		users: map[string]*storageModels.User{
			"u_1": {UserId: "u_1", Nickname: "Agent 1"},
			"u_2": {UserId: "u_2", Nickname: "Agent 2"},
		},
	}
	registered := map[string]struct{}{}
	oldRegister := registerIMUserForCustomer
	registerIMUserForCustomer = func(sdk *juggleimsdk.JuggleIMSdk, userId, nickname, portrait string) errs.IMErrorCode {
		registered[userId] = struct{}{}
		return errs.IMErrorCode_SUCCESS
	}
	t.Cleanup(func() {
		registerIMUserForCustomer = oldRegister
	})

	code, memberIds := prepareTicketGroupMemberIds(
		"app_1",
		"inbox_1",
		"customer_1",
		&juggleimsdk.JuggleIMSdk{},
		memberStorage,
		userStorage,
	)
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if len(memberIds) != 3 || memberIds[0] != "customer_1" {
		t.Fatalf("member ids = %v", memberIds)
	}
	if len(registered) != 2 {
		t.Fatalf("registered users = %v", registered)
	}
}

type customerInboxMemberStorage struct {
	members []*storageModels.InboxMember
}

func (s *customerInboxMemberStorage) Create(item storageModels.InboxMember) error { return nil }
func (s *customerInboxMemberStorage) Upsert(item storageModels.InboxMember) error { return nil }
func (s *customerInboxMemberStorage) Delete(appkey, inboxId, memberId string) error {
	return nil
}
func (s *customerInboxMemberStorage) Find(appkey, inboxId, memberId string) (*storageModels.InboxMember, error) {
	return nil, nil
}
func (s *customerInboxMemberStorage) QryByInbox(appkey, inboxId string, startId, limit int64) ([]*storageModels.InboxMember, error) {
	return s.members, nil
}
func (s *customerInboxMemberStorage) QryByMember(appkey, memberId string, startId, limit int64) ([]*storageModels.InboxMember, error) {
	return nil, nil
}
func (s *customerInboxMemberStorage) CountByInboxes(appkey string, inboxIds []string) (map[string]int64, error) {
	return nil, nil
}
func (s *customerInboxMemberStorage) ReplaceByInbox(appkey, inboxId string, memberIds []string) error {
	return nil
}

type customerUserStorage struct {
	users map[string]*storageModels.User
}

func (s *customerUserStorage) Create(item storageModels.User) error { return nil }
func (s *customerUserStorage) FindByAccount(appkey, account string) (*storageModels.User, error) {
	return nil, nil
}
func (s *customerUserStorage) FindByUserId(appkey, userId string) (*storageModels.User, error) {
	if s.users == nil {
		return nil, nil
	}
	return s.users[userId], nil
}
func (s *customerUserStorage) FindByAccountWithAppkey(account, appkey string) (*storageModels.User, error) {
	return nil, nil
}
func (s *customerUserStorage) UpdateImToken(appkey, userId, imToken string) error { return nil }
func (s *customerUserStorage) QryByApp(appkey string, filter storageModels.UserListFilter, limit, offset int64) (*storageModels.UserListResult, error) {
	return nil, nil
}
func (s *customerUserStorage) UpdateUser(appkey, userId string, updates storageModels.UserUpdate) error {
	return nil
}
func (s *customerUserStorage) Delete(appkey, userId string) error { return nil }
