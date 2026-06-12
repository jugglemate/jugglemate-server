package storages

import (
	"github.com/juggleim/jugglechat-server-ai/storages/dbs"
	"github.com/juggleim/jugglechat-server-ai/storages/models"
)

func NewAiBotStorage() models.IAiBotStorage {
	return &dbs.AiBotDao{}
}

func NewAgentStorage() models.AgentStorage {
	return &dbs.AgentDao{}
}
