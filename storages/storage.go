package storages

import (
	"github.com/juggleim/jugglemate-server/storages/dbs"
	"github.com/juggleim/jugglemate-server/storages/models"
)

func NewAiBotStorage() models.IAiBotStorage {
	return &dbs.AiBotDao{}
}

func NewUserStorage() models.IUserStorage {
	return &dbs.UserDao{}
}

func NewAppInfoStorage() models.IAppInfoStorage {
	return &dbs.AppInfoDao{}
}

func NewAppExtStorage() models.IAppExtStorage {
	return &dbs.AppExtDao{}
}

func NewCustomerStorage() models.ICustomerStorage {
	return &dbs.CustomerDao{}
}

func NewCustomerInboxRelStorage() models.ICustomerInboxRelStorage {
	return &dbs.CustomerInboxRelDao{}
}

func NewTicketStorage() models.ITicketStorage {
	return &dbs.TicketDao{}
}

func NewInboxStorage() models.IInboxStorage {
	return &dbs.InboxDao{}
}

func NewInboxMemberStorage() models.IInboxMemberStorage {
	return &dbs.InboxMemberDao{}
}

func NewAgentStorage() models.AgentStorage {
	return &dbs.AgentDao{}
}
