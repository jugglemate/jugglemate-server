package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/gorm"
)

type CustomerInboxRelDao struct {
	ID          int64     `gorm:"primary_key"`
	CustomerId  string    `gorm:"customer_id"`
	InboxId     string    `gorm:"inbox_id"`
	SourceId    string    `gorm:"source_id"`
	CreatedTime time.Time `gorm:"created_time"`
	UpdatedTime time.Time `gorm:"updated_time"`
	AppKey      string    `gorm:"app_key"`
}

func (CustomerInboxRelDao) TableName() string {
	return "customerinboxrels"
}

func (d *CustomerInboxRelDao) toModel() *models.CustomerInboxRel {
	return &models.CustomerInboxRel{
		ID:          d.ID,
		CustomerId:  d.CustomerId,
		InboxId:     d.InboxId,
		SourceId:    d.SourceId,
		CreatedTime: d.CreatedTime.UnixMilli(),
		UpdatedTime: d.UpdatedTime.UnixMilli(),
		AppKey:      d.AppKey,
	}
}

func newCustomerInboxRelDao(item models.CustomerInboxRel) *CustomerInboxRelDao {
	dao := &CustomerInboxRelDao{
		CustomerId: item.CustomerId,
		InboxId:    item.InboxId,
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

func (d *CustomerInboxRelDao) Create(item models.CustomerInboxRel) error {
	dao := newCustomerInboxRelDao(item)
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

func (d *CustomerInboxRelDao) Update(item models.CustomerInboxRel) error {
	return dbcommons.GetDb().Model(&CustomerInboxRelDao{}).
		Where("app_key=? and customer_id=? and inbox_id=?", item.AppKey, item.CustomerId, item.InboxId).
		Updates(map[string]interface{}{
			"source_id":    item.SourceId,
			"updated_time": time.Now(),
		}).Error
}

func (d *CustomerInboxRelDao) Upsert(item models.CustomerInboxRel) error {
	existing, err := d.FindByCustomerInbox(item.AppKey, item.CustomerId, item.InboxId)
	if err != nil {
		return err
	}
	if existing == nil {
		return d.Create(item)
	}
	return d.Update(item)
}

func (d *CustomerInboxRelDao) Delete(appkey, customerId, inboxId, sourceId string) error {
	return dbcommons.GetDb().
		Where("app_key=? and customer_id=? and inbox_id=? and source_id=?", appkey, customerId, inboxId, sourceId).
		Delete(&CustomerInboxRelDao{}).Error
}

func (d *CustomerInboxRelDao) Find(appkey, customerId, inboxId, sourceId string) (*models.CustomerInboxRel, error) {
	var item CustomerInboxRelDao
	err := dbcommons.GetDb().
		Where("app_key=? and customer_id=? and inbox_id=? and source_id=?", appkey, customerId, inboxId, sourceId).
		Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

func (d *CustomerInboxRelDao) FindByCustomerInbox(appkey, customerId, inboxId string) (*models.CustomerInboxRel, error) {
	var item CustomerInboxRelDao
	err := dbcommons.GetDb().
		Where("app_key=? and customer_id=? and inbox_id=?", appkey, customerId, inboxId).
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

func (d *CustomerInboxRelDao) QryByCustomer(appkey, customerId string, startId, limit int64) ([]*models.CustomerInboxRel, error) {
	db := dbcommons.GetDb().Where("app_key=? and customer_id=?", appkey, customerId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	return queryCustomerInboxRels(db, limit)
}

func (d *CustomerInboxRelDao) QryByInbox(appkey, inboxId string, startId, limit int64) ([]*models.CustomerInboxRel, error) {
	db := dbcommons.GetDb().Where("app_key=? and inbox_id=?", appkey, inboxId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	return queryCustomerInboxRels(db, limit)
}

func queryCustomerInboxRels(db *gorm.DB, limit int64) ([]*models.CustomerInboxRel, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []CustomerInboxRelDao
	err := db.Order("id desc").Limit(int(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.CustomerInboxRel, 0, len(items))
	for _, item := range items {
		ret = append(ret, item.toModel())
	}
	return ret, nil
}
