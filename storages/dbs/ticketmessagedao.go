package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TicketMessageDao struct {
	ID          int64     `gorm:"primary_key"`
	AppKey      string    `gorm:"app_key"`
	TicketId    string    `gorm:"ticket_id"`
	SenderId    string    `gorm:"sender_id"`
	SenderRole  string    `gorm:"sender_role"`
	MsgId       string    `gorm:"msg_id"`
	MsgType     string    `gorm:"msg_type"`
	CreatedTime time.Time `gorm:"created_time"`
}

func (TicketMessageDao) TableName() string {
	return "ticket_messages"
}

func (d *TicketMessageDao) toModel() *models.TicketMessage {
	if d == nil {
		return nil
	}
	return &models.TicketMessage{
		ID:          d.ID,
		AppKey:      d.AppKey,
		TicketId:    d.TicketId,
		SenderId:    d.SenderId,
		SenderRole:  models.TicketEventOperator(d.SenderRole),
		MsgId:       d.MsgId,
		MsgType:     d.MsgType,
		CreatedTime: d.CreatedTime.UnixMilli(),
	}
}

func newTicketMessageDao(item models.TicketMessage) *TicketMessageDao {
	dao := &TicketMessageDao{
		AppKey:     item.AppKey,
		TicketId:   item.TicketId,
		SenderId:   item.SenderId,
		SenderRole: string(item.SenderRole),
		MsgId:      item.MsgId,
		MsgType:    item.MsgType,
	}
	if item.CreatedTime > 0 {
		dao.CreatedTime = time.UnixMilli(item.CreatedTime)
	}
	return dao
}

// UpsertByMsgId 按 (app_key, msg_id) 去重写入。
//
// TIPS: sender_role / msg_type / created_time 取最新值，因为 IM 重投时可能变更；
// app_key / ticket_id / sender_id 取首次值（同 msg_id 不应对应不同会话上下文）。
func (d *TicketMessageDao) UpsertByMsgId(item models.TicketMessage) error {
	dao := newTicketMessageDao(item)
	if dao.CreatedTime.IsZero() {
		dao.CreatedTime = time.Now()
	}
	return dbcommons.GetDb().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "app_key"}, {Name: "msg_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"sender_role", "msg_type", "created_time",
		}),
	}).Create(dao).Error
}

// QryLastByTicket 取该 ticket 最近一条消息（id desc）。
func (d *TicketMessageDao) QryLastByTicket(appkey, ticketId string) (*models.TicketMessage, error) {
	var row TicketMessageDao
	err := dbcommons.GetDb().
		Where("app_key=? AND ticket_id=?", appkey, ticketId).
		Order("id desc").
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return row.toModel(), nil
}

var _ models.ITicketMessageStorage = (*TicketMessageDao)(nil)
