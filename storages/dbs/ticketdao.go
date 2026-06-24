package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TicketDao struct {
	ID          int64     `gorm:"primary_key"`
	TicketId    string    `gorm:"ticket_id"`
	CustomerId  string    `gorm:"customer_id"`
	AssigneeId  string    `gorm:"assignee_id"`
	Status      int       `gorm:"status"`
	CreatedTime time.Time `gorm:"created_time"`
	UpdatedTime time.Time `gorm:"updated_time"`
	AppKey      string    `gorm:"app_key"`
}

func (TicketDao) TableName() string {
	return "tickets"
}

func (d *TicketDao) toModel() *models.Ticket {
	return &models.Ticket{
		ID:          d.ID,
		TicketId:    d.TicketId,
		CustomerId:  d.CustomerId,
		AssigneeId:  d.AssigneeId,
		Status:      models.TicketStatus(d.Status),
		CreatedTime: d.CreatedTime.UnixMilli(),
		UpdatedTime: d.UpdatedTime.UnixMilli(),
		AppKey:      d.AppKey,
	}
}

func newTicketDao(item models.Ticket) *TicketDao {
	dao := &TicketDao{
		TicketId:   item.TicketId,
		CustomerId: item.CustomerId,
		AssigneeId: item.AssigneeId,
		Status:     int(item.Status),
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

func (d *TicketDao) Create(item models.Ticket) error {
	dao := newTicketDao(item)
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

func (d *TicketDao) Update(item models.Ticket) error {
	return dbcommons.GetDb().Model(&TicketDao{}).
		Where("app_key=? and ticket_id=?", item.AppKey, item.TicketId).
		Updates(map[string]interface{}{
			"customer_id":  item.CustomerId,
			"assignee_id":  item.AssigneeId,
			"status":       int(item.Status),
			"updated_time": time.Now(),
		}).Error
}

func (d *TicketDao) Upsert(item models.Ticket) error {
	dao := newTicketDao(item)
	if dao.UpdatedTime.IsZero() {
		dao.UpdatedTime = time.Now()
	}
	db := dbcommons.GetDb()
	if dao.CreatedTime.IsZero() {
		db = db.Omit("created_time")
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "app_key"}, {Name: "ticket_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"customer_id",
			"assignee_id",
			"status",
			"updated_time",
		}),
	}).Create(dao).Error
}

func (d *TicketDao) Delete(appkey, ticketId string) error {
	return dbcommons.GetDb().
		Where("app_key=? and ticket_id=?", appkey, ticketId).
		Delete(&TicketDao{}).Error
}

func (d *TicketDao) FindByTicketId(appkey, ticketId string) (*models.Ticket, error) {
	var item TicketDao
	err := dbcommons.GetDb().
		Where("app_key=? and ticket_id=?", appkey, ticketId).
		Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

func (d *TicketDao) QryByCustomer(appkey, customerId string, startId, limit int64) ([]*models.Ticket, error) {
	db := dbcommons.GetDb().Where("app_key=? and customer_id=?", appkey, customerId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	return queryTickets(db, limit)
}

func (d *TicketDao) QryByAssignee(appkey, assigneeId string, status int, startId, limit int64) ([]*models.Ticket, error) {
	db := dbcommons.GetDb().Where("app_key=? and assignee_id=?", appkey, assigneeId)
	if status >= 0 {
		db = db.Where("status=?", status)
	}
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	return queryTickets(db, limit)
}

func (d *TicketDao) UpdateStatus(appkey, ticketId string, status models.TicketStatus) error {
	return dbcommons.GetDb().Model(&TicketDao{}).
		Where("app_key=? and ticket_id=?", appkey, ticketId).
		Updates(map[string]interface{}{
			"status":       int(status),
			"updated_time": time.Now(),
		}).Error
}

func queryTickets(db *gorm.DB, limit int64) ([]*models.Ticket, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []TicketDao
	err := db.Order("id desc").Limit(int(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.Ticket, 0, len(items))
	for _, item := range items {
		ret = append(ret, item.toModel())
	}
	return ret, nil
}
