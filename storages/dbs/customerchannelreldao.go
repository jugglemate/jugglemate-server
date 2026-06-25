package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/gorm"
)

type CustomerChannelRelDao struct {
	ID          int64     `gorm:"primary_key"`
	CustomerId  string    `gorm:"customer_id"`
	ChannelId   string    `gorm:"channel_id"`
	SourceId    string    `gorm:"source_id"`
	CreatedTime time.Time `gorm:"created_time"`
	UpdatedTime time.Time `gorm:"updated_time"`
	AppKey      string    `gorm:"app_key"`
}

func (CustomerChannelRelDao) TableName() string {
	return "customerchannelrels"
}

func (d *CustomerChannelRelDao) toModel() *models.CustomerChannelRel {
	return &models.CustomerChannelRel{
		ID:          d.ID,
		CustomerId:  d.CustomerId,
		ChannelId:   d.ChannelId,
		SourceId:    d.SourceId,
		CreatedTime: d.CreatedTime.UnixMilli(),
		UpdatedTime: d.UpdatedTime.UnixMilli(),
		AppKey:      d.AppKey,
	}
}

func newCustomerChannelRelDao(item models.CustomerChannelRel) *CustomerChannelRelDao {
	dao := &CustomerChannelRelDao{
		CustomerId: item.CustomerId,
		ChannelId:  item.ChannelId,
		SourceId:   item.SourceId,
		AppKey:     item.AppKey,
	}
	if item.CreatedTime > 0 {
		dao.CreatedTime = time.UnixMilli(item.CreatedTime)
	}
	if item.UpdatedTime > 0 {
		dao.UpdatedTime = time.UnixMilli(item.UpdatedTime)
	}
	return dao
}

func (d *CustomerChannelRelDao) Create(item models.CustomerChannelRel) error {
	dao := newCustomerChannelRelDao(item)
	db := dbcommons.GetDb()
	var omits []string
	if item.CreatedTime <= 0 {
		omits = append(omits, "created_time")
	}
	if item.UpdatedTime <= 0 {
		omits = append(omits, "updated_time")
	}
	if len(omits) > 0 {
		db = db.Omit(omits...)
	}
	return db.Create(dao).Error
}

func (d *CustomerChannelRelDao) Update(item models.CustomerChannelRel) error {
	return dbcommons.GetDb().Model(&CustomerChannelRelDao{}).
		Where("app_key=? and customer_id=? and channel_id=? and source_id=?", item.AppKey, item.CustomerId, item.ChannelId, item.SourceId).
		Update("updated_time", time.Now()).Error
}

func (d *CustomerChannelRelDao) Upsert(item models.CustomerChannelRel) error {
	existing, err := d.Find(item.AppKey, item.CustomerId, item.ChannelId, item.SourceId)
	if err != nil {
		return err
	}
	if existing == nil {
		return d.Create(item)
	}
	return d.Update(item)
}

func (d *CustomerChannelRelDao) Delete(appkey, customerId, channelId, sourceId string) error {
	return dbcommons.GetDb().
		Where("app_key=? and customer_id=? and channel_id=? and source_id=?", appkey, customerId, channelId, sourceId).
		Delete(&CustomerChannelRelDao{}).Error
}

func (d *CustomerChannelRelDao) Find(appkey, customerId, channelId, sourceId string) (*models.CustomerChannelRel, error) {
	var item CustomerChannelRelDao
	err := dbcommons.GetDb().
		Where("app_key=? and customer_id=? and channel_id=? and source_id=?", appkey, customerId, channelId, sourceId).
		Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

func (d *CustomerChannelRelDao) FindByCustomerChannel(appkey, customerId, channelId string) (*models.CustomerChannelRel, error) {
	var item CustomerChannelRelDao
	err := dbcommons.GetDb().
		Where("app_key=? and customer_id=? and channel_id=?", appkey, customerId, channelId).
		Order("id desc").
		Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

func (d *CustomerChannelRelDao) QryByCustomer(appkey, customerId string, startId, limit int64) ([]*models.CustomerChannelRel, error) {
	db := dbcommons.GetDb().Where("app_key=? and customer_id=?", appkey, customerId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	return queryCustomerChannelRels(db, limit)
}

func (d *CustomerChannelRelDao) QryByChannel(appkey, channelId string, startId, limit int64) ([]*models.CustomerChannelRel, error) {
	db := dbcommons.GetDb().Where("app_key=? and channel_id=?", appkey, channelId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	return queryCustomerChannelRels(db, limit)
}

func queryCustomerChannelRels(db *gorm.DB, limit int64) ([]*models.CustomerChannelRel, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []CustomerChannelRelDao
	err := db.Order("id desc").Limit(int(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.CustomerChannelRel, 0, len(items))
	for _, item := range items {
		ret = append(ret, item.toModel())
	}
	return ret, nil
}
