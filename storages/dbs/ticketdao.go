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
	SourceId    string    `gorm:"source_id"`
	CustomerId  string    `gorm:"customer_id"`
	ChannelId   string    `gorm:"channel_id"`
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
		SourceId:    d.SourceId,
		CustomerId:  d.CustomerId,
		ChannelId:   d.ChannelId,
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
		SourceId:   item.SourceId,
		CustomerId: item.CustomerId,
		ChannelId:  item.ChannelId,
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
			"source_id":    item.SourceId,
			"channel_id":   item.ChannelId,
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
			"source_id",
			"channel_id",
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

func (d *TicketDao) FindBySource(appkey, sourceId string) (*models.Ticket, error) {
	var item TicketDao
	err := dbcommons.GetDb().
		Where("app_key=? and source_id=?", appkey, sourceId).
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

func (d *TicketDao) QryAll(appkey string, status *models.TicketStatus, limit, offset int64) ([]*models.Ticket, error) {
	db := dbcommons.GetDb().Where("app_key=?", appkey)
	if status != nil {
		db = db.Where("status=?", int(*status))
	}
	return queryTicketsWithOffset(db, limit, offset)
}

func (d *TicketDao) QryVisible(appkey, assigneeId string, status *models.TicketStatus, limit, offset int64) ([]*models.Ticket, error) {
	db := dbcommons.GetDb().
		Where("app_key=?", appkey).
		Where("(status=? or assignee_id=?)", int(models.TicketStatusPending), assigneeId)
	if status != nil {
		db = db.Where("status=?", int(*status))
	}
	return queryTicketsWithOffset(db, limit, offset)
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

func (d *TicketDao) QryByChannel(appkey, channelId string, status int, startId, limit int64) ([]*models.Ticket, error) {
	db := dbcommons.GetDb().Where("app_key=? and channel_id=?", appkey, channelId)
	if status >= 0 {
		db = db.Where("status=?", status)
	}
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	return queryTickets(db, limit)
}

func (d *TicketDao) QryBySource(appkey, sourceId string, status int, startId, limit int64) ([]*models.Ticket, error) {
	db := dbcommons.GetDb().Where("app_key=? and source_id=?", appkey, sourceId)
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

func (d *TicketDao) ClaimIfPending(appkey, ticketId, assigneeId string) (*models.Ticket, error) {
	result := dbcommons.GetDb().Model(&TicketDao{}).
		Where("app_key=? and ticket_id=? and status=?", appkey, ticketId, int(models.TicketStatusPending)).
		Updates(map[string]interface{}{
			"assignee_id":  assigneeId,
			"status":       int(models.TicketStatusProcessing),
			"updated_time": time.Now(),
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return d.FindByTicketId(appkey, ticketId)
}

func (d *TicketDao) RevertClaimIfAssignee(appkey, ticketId, assigneeId string) error {
	return dbcommons.GetDb().Model(&TicketDao{}).
		Where("app_key=? and ticket_id=? and assignee_id=? and status=?", appkey, ticketId, assigneeId, int(models.TicketStatusProcessing)).
		Updates(map[string]interface{}{
			"assignee_id":  "",
			"status":       int(models.TicketStatusPending),
			"updated_time": time.Now(),
		}).Error
}

func queryTickets(db *gorm.DB, limit int64) ([]*models.Ticket, error) {
	return queryTicketsWithOffset(db, limit, 0)
}

func queryTicketsWithOffset(db *gorm.DB, limit, offset int64) ([]*models.Ticket, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var items []TicketDao
	err := db.Order("id desc").Limit(int(limit)).Offset(int(offset)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.Ticket, 0, len(items))
	for _, item := range items {
		ret = append(ret, item.toModel())
	}
	return ret, nil
}
