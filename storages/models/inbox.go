package models

type Inbox struct {
	ID          int64
	InboxId     string
	ChannelType string
	ChannelConf string
	Name        string
	CreatedTime int64
	UpdatedTime int64
	AppKey      string
}

type InboxListResult struct {
	List  []*Inbox
	Total int64
}

type IInboxStorage interface {
	Create(item Inbox) error
	Update(item Inbox) error
	Upsert(item Inbox) error
	Delete(appkey, inboxId string) error
	FindByInboxId(appkey, inboxId string) (*Inbox, error)
	QryByApp(appkey, channelType string, limit, offset int64) (*InboxListResult, error)
	QryByChannelType(appkey, channelType string, startId, limit int64) ([]*Inbox, error)
}

type InboxMember struct {
	ID          int64
	InboxId     string
	MemberId    string
	CreatedTime int64
	AppKey      string
}

type IInboxMemberStorage interface {
	Create(item InboxMember) error
	Upsert(item InboxMember) error
	Delete(appkey, inboxId, memberId string) error
	Find(appkey, inboxId, memberId string) (*InboxMember, error)
	QryByInbox(appkey, inboxId string, startId, limit int64) ([]*InboxMember, error)
	QryByMember(appkey, memberId string, startId, limit int64) ([]*InboxMember, error)
	CountByInboxes(appkey string, inboxIds []string) (map[string]int64, error)
	ReplaceByInbox(appkey, inboxId string, memberIds []string) error
}
