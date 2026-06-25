package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CustomerDao struct {
	ID          int64     `gorm:"primary_key"`
	CustomerId  string    `gorm:"customer_id"`
	Nickname    string    `gorm:"nickname"`
	Avator      string    `gorm:"avator"`
	Phone       string    `gorm:"phone"`
	Email       string    `gorm:"email"`
	Identifier  string    `gorm:"identifier"`
	CreatedTime time.Time `gorm:"created_time"`
	UpdatedTime time.Time `gorm:"updated_time"`
	AppKey      string    `gorm:"app_key"`
}

func (CustomerDao) TableName() string {
	return "customers"
}

func (d *CustomerDao) toModel() *models.Customer {
	return &models.Customer{
		ID:          d.ID,
		CustomerId:  d.CustomerId,
		Nickname:    d.Nickname,
		Avator:      d.Avator,
		Phone:       d.Phone,
		Email:       d.Email,
		Identifier:  d.Identifier,
		CreatedTime: d.CreatedTime.UnixMilli(),
		UpdatedTime: d.UpdatedTime.UnixMilli(),
		AppKey:      d.AppKey,
	}
}

func (d *CustomerDao) Create(item models.Customer) error {
	dao := &CustomerDao{
		CustomerId:  item.CustomerId,
		Nickname:    item.Nickname,
		Avator:      item.Avator,
		Phone:       item.Phone,
		Email:       item.Email,
		Identifier:  item.Identifier,
		UpdatedTime: time.Now(),
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

func (d *CustomerDao) Update(item models.Customer) error {
	return dbcommons.GetDb().Model(&CustomerDao{}).
		Where("app_key=? and customer_id=?", item.AppKey, item.CustomerId).
		Updates(map[string]interface{}{
			"nickname":     item.Nickname,
			"avator":       item.Avator,
			"phone":        item.Phone,
			"email":        item.Email,
			"identifier":   item.Identifier,
			"updated_time": time.Now(),
		}).Error
}

func (d *CustomerDao) Upsert(item models.Customer) error {
	dao := &CustomerDao{
		CustomerId: item.CustomerId,
		Nickname:   item.Nickname,
		Avator:     item.Avator,
		Phone:      item.Phone,
		Email:      item.Email,
		Identifier: item.Identifier,
		AppKey:     item.AppKey,
	}
	return dbcommons.GetDb().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "app_key"}, {Name: "customer_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"nickname", "avator", "phone", "email", "identifier", "updated_time"}),
	}).Create(dao).Error
}

func (d *CustomerDao) Delete(appkey, customerId string) error {
	return dbcommons.GetDb().
		Where("app_key=? and customer_id=?", appkey, customerId).
		Delete(&CustomerDao{}).Error
}

func (d *CustomerDao) FindByCustomerId(appkey, customerId string) (*models.Customer, error) {
	var item CustomerDao
	err := dbcommons.GetDb().
		Where("app_key=? and customer_id=?", appkey, customerId).
		Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

func (d *CustomerDao) FindByIdentifier(appkey, identifier string) (*models.Customer, error) {
	var item CustomerDao
	err := dbcommons.GetDb().
		Where("app_key=? and identifier=?", appkey, identifier).
		Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}
