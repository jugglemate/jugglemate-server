package models

type AiBot struct {
	ID          int64
	BotId       string
	BotName     string
	BotPortrait string
	Prompts     string
	OwnerId     string
	UpdatedTime int64
	CreatedTime int64
	AppKey      string
}

type IAiBotStorage interface {
	Create(item AiBot) error
	Update(item AiBot) error
	Delete(appkey, botId string) error
	FindByBotId(appkey, botId string) (*AiBot, error)
	QryByOwner(appkey, ownerId string, startId, limit int64) ([]*AiBot, error)
}
