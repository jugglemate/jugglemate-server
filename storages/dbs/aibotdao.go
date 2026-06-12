package dbs

import (
	"time"

	"github.com/juggleim/jugglechat-server-ai/storages/models"
	"github.com/juggleim/jugglechat-server-ai/commons/dbcommons"
)

type AiBotDao struct {
	ID          int64     `gorm:"primary_key"`
	BotId       string    `gorm:"bot_id"`
	BotName     string    `gorm:"bot_name"`
	BotPortrait string    `gorm:"bot_portrait"`
	Prompts     string    `gorm:"prompts"`
	OwnerId     string    `gorm:"owner_id"`
	UpdatedTime time.Time `gorm:"updated_time"`
	CreatedTime time.Time `gorm:"created_time"`
	AppKey      string    `gorm:"app_key"`
}

func (bot AiBotDao) TableName() string {
	return "aibots"
}

func (bot AiBotDao) Create(item models.AiBot) error {
	dao := &AiBotDao{
		BotId:       item.BotId,
		BotName:     item.BotName,
		BotPortrait: item.BotPortrait,
		Prompts:     item.Prompts,
		OwnerId:     item.OwnerId,
		AppKey:      item.AppKey,
	}

	db := dbcommons.GetDb()
	var omits []string
	if item.CreatedTime > 0 {
		dao.CreatedTime = time.UnixMilli(item.CreatedTime)
	} else {
		omits = append(omits, "created_time")
	}
	if item.UpdatedTime > 0 {
		dao.UpdatedTime = time.UnixMilli(item.UpdatedTime)
	} else {
		omits = append(omits, "updated_time")
	}
	if len(omits) > 0 {
		db = db.Omit(omits...)
	}
	return db.Create(dao).Error
}

func (bot AiBotDao) Update(item models.AiBot) error {
	updates := map[string]interface{}{
		"bot_name":     item.BotName,
		"bot_portrait": item.BotPortrait,
		"prompts":      item.Prompts,
		"owner_id":     item.OwnerId,
	}
	return dbcommons.GetDb().Model(&AiBotDao{}).
		Where("app_key=? and bot_id=?", item.AppKey, item.BotId).
		Updates(updates).Error
}

func (bot AiBotDao) Delete(appkey, botId string) error {
	return dbcommons.GetDb().Where("app_key=? and bot_id=?", appkey, botId).Delete(&AiBotDao{}).Error
}

func (bot AiBotDao) FindByBotId(appkey, botId string) (*models.AiBot, error) {
	var item AiBotDao
	err := dbcommons.GetDb().Where("app_key=? and bot_id=?", appkey, botId).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return &models.AiBot{
		ID:          item.ID,
		BotId:       item.BotId,
		BotName:     item.BotName,
		BotPortrait: item.BotPortrait,
		Prompts:     item.Prompts,
		OwnerId:     item.OwnerId,
		UpdatedTime: item.UpdatedTime.UnixMilli(),
		CreatedTime: item.CreatedTime.UnixMilli(),
		AppKey:      item.AppKey,
	}, nil
}

func (bot AiBotDao) QryByOwner(appkey, ownerId string, startId, limit int64) ([]*models.AiBot, error) {
	db := dbcommons.GetDb().Where("app_key=? and owner_id=?", appkey, ownerId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AiBotDao
	err := db.Order("id desc").Limit(int(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AiBot, 0, len(items))
	for _, item := range items {
		ret = append(ret, &models.AiBot{
			ID:          item.ID,
			BotId:       item.BotId,
			BotName:     item.BotName,
			BotPortrait: item.BotPortrait,
			Prompts:     item.Prompts,
			OwnerId:     item.OwnerId,
			UpdatedTime: item.UpdatedTime.UnixMilli(),
			CreatedTime: item.CreatedTime.UnixMilli(),
			AppKey:      item.AppKey,
		})
	}
	return ret, nil
}
