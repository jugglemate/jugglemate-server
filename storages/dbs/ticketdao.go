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
	ID               int64     `gorm:"primary_key"`
	TicketId         string    `gorm:"ticket_id"`
	SourceId         string    `gorm:"source_id"`
	CustomerId       string    `gorm:"customer_id"`
	InboxId          string    `gorm:"inbox_id"`
	ChannelType      string    `gorm:"channel_type"`
	AssigneeId       string    `gorm:"assignee_id"`
	Status           int       `gorm:"status"`
	IsHumanTakenOver bool      `gorm:"is_human_taken_over"`
	HumanTakenOverAt time.Time `gorm:"human_taken_over_at"`
	HumanTakenOverBy string    `gorm:"human_taken_over_by"`
	LastUserMsgAt    *time.Time `gorm:"last_user_msg_at"`
	ClosedAt         *time.Time `gorm:"closed_at"`
	CreatedTime      time.Time `gorm:"created_time"`
	UpdatedTime      time.Time `gorm:"updated_time"`
	AppKey           string    `gorm:"app_key"`
}

func (TicketDao) TableName() string {
	return "tickets"
}

func (d *TicketDao) toModel() *models.Ticket {
	if d == nil {
		return nil
	}
	item := &models.Ticket{
		ID:               d.ID,
		TicketId:         d.TicketId,
		SourceId:         d.SourceId,
		CustomerId:       d.CustomerId,
		InboxId:          d.InboxId,
		ChannelType:      d.ChannelType,
		AssigneeId:       d.AssigneeId,
		Status:           models.TicketStatus(d.Status),
		IsHumanTakenOver: d.IsHumanTakenOver,
		HumanTakenOverBy: d.HumanTakenOverBy,
		AppKey:           d.AppKey,
		CreatedTime:      d.CreatedTime.UnixMilli(),
		UpdatedTime:      d.UpdatedTime.UnixMilli(),
	}
	if !d.HumanTakenOverAt.IsZero() {
		item.HumanTakenOverAt = d.HumanTakenOverAt.UnixMilli()
	}
	if d.LastUserMsgAt != nil && !d.LastUserMsgAt.IsZero() {
		item.LastUserMsgAt = d.LastUserMsgAt.UnixMilli()
	}
	if d.ClosedAt != nil && !d.ClosedAt.IsZero() {
		item.ClosedAt = d.ClosedAt.UnixMilli()
	}
	return item
}

func newTicketDao(item models.Ticket) *TicketDao {
	dao := &TicketDao{
		TicketId:         item.TicketId,
		SourceId:         item.SourceId,
		CustomerId:       item.CustomerId,
		InboxId:          item.InboxId,
		ChannelType:      item.ChannelType,
		AssigneeId:       item.AssigneeId,
		Status:           int(item.Status),
		IsHumanTakenOver: item.IsHumanTakenOver,
		HumanTakenOverBy: item.HumanTakenOverBy,
		AppKey:           item.AppKey,
	}
	if item.HumanTakenOverAt > 0 {
		dao.HumanTakenOverAt = time.UnixMilli(item.HumanTakenOverAt)
	}
	if item.LastUserMsgAt > 0 {
		t := time.UnixMilli(item.LastUserMsgAt)
		dao.LastUserMsgAt = &t
	}
	if item.ClosedAt > 0 {
		t := time.UnixMilli(item.ClosedAt)
		dao.ClosedAt = &t
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
	if item.HumanTakenOverAt <= 0 {
		omits = append(omits, "human_taken_over_at")
	}
	if len(omits) > 0 {
		db = db.Omit(omits...)
	}
	return db.Create(dao).Error
}

func (d *TicketDao) Update(item models.Ticket) error {
	updates := map[string]interface{}{
		"customer_id":  item.CustomerId,
		"source_id":    item.SourceId,
		"inbox_id":     item.InboxId,
		"channel_type": item.ChannelType,
		"assignee_id":  item.AssigneeId,
		"status":       int(item.Status),
		"updated_time": time.Now(),
	}
	if item.HumanTakenOverAt > 0 {
		updates["human_taken_over_at"] = time.UnixMilli(item.HumanTakenOverAt)
	}
	if item.HumanTakenOverBy != "" {
		updates["human_taken_over_by"] = item.HumanTakenOverBy
	}
	return dbcommons.GetDb().Model(&TicketDao{}).
		Where("app_key=? and ticket_id=?", item.AppKey, item.TicketId).
		Updates(updates).Error
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
	if dao.HumanTakenOverAt.IsZero() {
		db = db.Omit("human_taken_over_at")
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "app_key"}, {Name: "ticket_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"customer_id",
			"source_id",
			"inbox_id",
			"channel_type",
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

func (d *TicketDao) QryAll(appkey string, status *models.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*models.Ticket, error) {
	db := dbcommons.GetDb().Where("app_key=?", appkey)
	if status != nil {
		db = db.Where("status=?", int(*status))
	}
	if isHumanTakenOver != nil {
		db = db.Where("is_human_taken_over=?", *isHumanTakenOver)
	}
	return queryTicketsWithOffset(db, limit, offset)
}

func (d *TicketDao) QryVisible(appkey, assigneeId string, status *models.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*models.Ticket, error) {
	db := dbcommons.GetDb().
		Where("app_key=?", appkey).
		Where("(status=? or assignee_id=?)", int(models.TicketStatusPending), assigneeId)
	if status != nil {
		db = db.Where("status=?", int(*status))
	}
	if isHumanTakenOver != nil {
		db = db.Where("is_human_taken_over=?", *isHumanTakenOver)
	}
	return queryTicketsWithOffset(db, limit, offset)
}

func (d *TicketDao) QryByCustomer(appkey, customerId string, isHumanTakenOver *bool, startId, limit int64) ([]*models.Ticket, error) {
	db := dbcommons.GetDb().Where("app_key=? and customer_id=?", appkey, customerId)
	if isHumanTakenOver != nil {
		db = db.Where("is_human_taken_over=?", *isHumanTakenOver)
	}
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

func (d *TicketDao) QryByInbox(appkey, inboxId string, status int, startId, limit int64) ([]*models.Ticket, error) {
	db := dbcommons.GetDb().Where("app_key=? and inbox_id=?", appkey, inboxId)
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

func (d *TicketDao) TransferIfAssignee(appkey, ticketId, oldAssigneeId, newAssigneeId string) (*models.Ticket, error) {
	result := dbcommons.GetDb().Model(&TicketDao{}).
		Where("app_key=? and ticket_id=? and assignee_id=? and status=?", appkey, ticketId, oldAssigneeId, int(models.TicketStatusProcessing)).
		Updates(map[string]interface{}{
			"assignee_id":  newAssigneeId,
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

// MarkHumanTakenOverIfZero 仅在工单尚未转人工时一次性写入首次转人工事实。
//
// TIPS: 重复触发只补 ticket_events，不更新 human_taken_over_at —— 事务内一句 UPDATE
// 配合 ticket_events INSERT 即可保证单调递增。
func (d *TicketDao) MarkHumanTakenOverIfZero(appkey, ticketId, by string, atMs int64) error {
	at := time.UnixMilli(atMs)
	return dbcommons.GetDb().Model(&TicketDao{}).
		Where("app_key=? and ticket_id=? and is_human_taken_over=?", appkey, ticketId, false).
		Updates(map[string]interface{}{
			"is_human_taken_over": true,
			"human_taken_over_at": at,
			"human_taken_over_by": by,
			"updated_time":        time.Now(),
		}).Error
}

// UpdateLastUserMsgAt 在 atMs 大于当前 last_user_msg_at 时才覆盖。
//
// TIPS: 历史消息回灌不会"倒退" 时间戳；PostgreSQL 用 GREATEST 函数，MySQL 用
// CASE WHEN 实现同样的语义（IF/GREATEST 在 MySQL 8 才有）。
func (d *TicketDao) UpdateLastUserMsgAt(appkey, ticketId string, atMs int64) error {
	at := time.UnixMilli(atMs)
	db := dbcommons.GetDb()
	dialect := db.Dialector.Name()
	var query string
	var args []interface{}
	switch dialect {
	case "postgres":
		query = `UPDATE tickets
			SET last_user_msg_at = GREATEST(COALESCE(last_user_msg_at, $1::timestamptz), $2::timestamptz),
			    updated_time = now()
			WHERE app_key=$3 AND ticket_id=$4`
		args = []interface{}{at, at, appkey, ticketId}
	default:
		query = `UPDATE tickets
			SET last_user_msg_at = CASE
			    WHEN last_user_msg_at IS NULL THEN ?
			    WHEN last_user_msg_at < ? THEN ?
			    ELSE last_user_msg_at
			END,
			updated_time = NOW()
			WHERE app_key=? AND ticket_id=?`
		args = []interface{}{at, at, at, appkey, ticketId}
	}
	return db.Exec(query, args...).Error
}

// CloseByIdle 抢占式关闭：仅在 status=1 且 last_user_msg_at < cutoff 时执行；
// 其他实例已处理则 RowsAffected=0，返回 (false, nil)。
func (d *TicketDao) CloseByIdle(appkey, ticketId string, idleMs int64, atMs int64) (bool, error) {
	now := time.UnixMilli(atMs)
	cutoffMs := atMs - idleMs
	cutoff := time.UnixMilli(cutoffMs)
	res := dbcommons.GetDb().Model(&TicketDao{}).
		Where("app_key=? AND ticket_id=? AND status=? AND last_user_msg_at IS NOT NULL AND last_user_msg_at < ?",
			appkey, ticketId, int(models.TicketStatusProcessing), cutoff).
		Updates(map[string]interface{}{
			"status":       int(models.TicketStatusClosed),
			"closed_at":    now,
			"updated_time": now,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
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
	for i := range items {
		ret = append(ret, items[i].toModel())
	}
	return ret, nil
}

var _ models.ITicketStorage = (*TicketDao)(nil)
