package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AppInfoDao struct {
	ID          int64     `gorm:"primary_key"`
	AppKey      string    `gorm:"app_key"`
	AppSecret   string    `gorm:"app_secret"`
	AppStatus   int       `gorm:"app_status"`
	CreatedTime time.Time `gorm:"created_time"`
	UpdatedTime time.Time `gorm:"updated_time"`
	AppName     string    `gorm:"app_name"`
}

func (AppInfoDao) TableName() string {
	return "apps"
}

func (d *AppInfoDao) toModel() *models.AppInfo {
	return &models.AppInfo{
		ID:          d.ID,
		AppKey:      d.AppKey,
		AppSecret:   d.AppSecret,
		AppStatus:   d.AppStatus,
		CreatedTime: d.CreatedTime.UnixMilli(),
		UpdatedTime: d.UpdatedTime.UnixMilli(),
		AppName:     d.AppName,
	}
}

func (d *AppInfoDao) Create(item models.AppInfo) error {
	dao := &AppInfoDao{
		AppKey:    item.AppKey,
		AppSecret: item.AppSecret,
		AppStatus: item.AppStatus,
		AppName:   item.AppName,
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

func (d *AppInfoDao) Update(item models.AppInfo) error {
	return dbcommons.GetDb().Model(&AppInfoDao{}).
		Where("app_key=?", item.AppKey).
		Updates(map[string]interface{}{
			"app_secret":   item.AppSecret,
			"app_status":   item.AppStatus,
			"app_name":     item.AppName,
			"updated_time": time.Now(),
		}).Error
}

func (d *AppInfoDao) Delete(appkey string) error {
	return dbcommons.GetDb().Where("app_key=?", appkey).Delete(&AppInfoDao{}).Error
}

func (d *AppInfoDao) FindByAppkey(appkey string) (*models.AppInfo, error) {
	var item AppInfoDao
	err := dbcommons.GetDb().Where("app_key=?", appkey).Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

type AppExtDao struct {
	ID           int64     `gorm:"primary_key"`
	AppKey       string    `gorm:"app_key"`
	AppItemKey   string    `gorm:"app_item_key"`
	AppItemValue string    `gorm:"app_item_value"`
	UpdatedTime  time.Time `gorm:"updated_time"`
}

func (AppExtDao) TableName() string {
	return "appexts"
}

func (d *AppExtDao) toModel() *models.AppExt {
	return &models.AppExt{
		ID:           d.ID,
		AppKey:       d.AppKey,
		AppItemKey:   d.AppItemKey,
		AppItemValue: d.AppItemValue,
		UpdatedTime:  d.UpdatedTime.UnixMilli(),
	}
}

func (d *AppExtDao) Upsert(item models.AppExt) error {
	dao := &AppExtDao{
		AppKey:       item.AppKey,
		AppItemKey:   item.AppItemKey,
		AppItemValue: item.AppItemValue,
	}
	if item.UpdatedTime > 0 {
		dao.UpdatedTime = time.UnixMilli(item.UpdatedTime)
	} else {
		dao.UpdatedTime = time.Now()
	}
	return dbcommons.GetDb().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "app_key"}, {Name: "app_item_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"app_item_value", "updated_time"}),
	}).Create(dao).Error
}

func (d *AppExtDao) Delete(appkey, itemKey string) error {
	return dbcommons.GetDb().
		Where("app_key=? and app_item_key=?", appkey, itemKey).
		Delete(&AppExtDao{}).Error
}

func (d *AppExtDao) FindByItemKey(appkey, itemKey string) (*models.AppExt, error) {
	items, err := d.FindByItemKeys(appkey, []string{itemKey})
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return items[0], nil
}

func (d *AppExtDao) FindByItemKeys(appkey string, itemKeys []string) ([]*models.AppExt, error) {
	if len(itemKeys) == 0 {
		return []*models.AppExt{}, nil
	}
	var items []AppExtDao
	err := dbcommons.GetDb().
		Where("app_key=? and app_item_key in ?", appkey, itemKeys).
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AppExt, 0, len(items))
	for _, item := range items {
		ret = append(ret, item.toModel())
	}
	return ret, nil
}
