package storages

import (
	"github.com/juggleim/jugglemate-server/storages/dbs"
	"github.com/juggleim/jugglemate-server/storages/models"
)

func NewAiBotStorage() models.IAiBotStorage {
	return &dbs.AiBotDao{}
}

func NewAgentStorage() models.AgentStorage {
	return &dbs.AgentDao{}
}
