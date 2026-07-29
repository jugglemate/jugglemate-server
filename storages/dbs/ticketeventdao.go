package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/storages/models"
)

// TicketEventDao 是 ticket_events 表的 GORM 实现。
//
// TIPS: Payload 字段以原始 JSON 字符串持久化 —— 上层 service 在调用 Create 之前自行
// encoding/json，DAO 仅做静态读写不做类型转换，避免在 MySQL 路径上引入额外依赖。
type TicketEventDao struct {
	ID           int64     `gorm:"primary_key"`
	AppKey       string    `gorm:"app_key"`
	TicketId     string    `gorm:"ticket_id"`
	EventType    string    `gorm:"event_type"`
	OperatorID   string    `gorm:"operator_id"`
	OperatorType string    `gorm:"operator_type"`
	Payload      string    `gorm:"payload"`
	CreatedTime  time.Time `gorm:"created_time"`
}

func (TicketEventDao) TableName() string {
	return "ticket_events"
}

func (d *TicketEventDao) toModel() *models.TicketEvent {
	if d == nil {
		return nil
	}
	return &models.TicketEvent{
		ID:           d.ID,
		AppKey:       d.AppKey,
		TicketId:     d.TicketId,
		EventType:    models.TicketEventType(d.EventType),
		OperatorID:   d.OperatorID,
		OperatorType: models.TicketEventOperator(d.OperatorType),
		Payload:      d.Payload,
		CreatedTime:  d.CreatedTime.UnixMilli(),
	}
}

func newTicketEventDao(event models.TicketEvent) *TicketEventDao {
	dao := &TicketEventDao{
		AppKey:       event.AppKey,
		TicketId:     event.TicketId,
		EventType:    string(event.EventType),
		OperatorID:   event.OperatorID,
		OperatorType: string(event.OperatorType),
		Payload:      event.Payload,
	}
	if event.CreatedTime > 0 {
		dao.CreatedTime = time.UnixMilli(event.CreatedTime)
	}
	return dao
}

func (d *TicketEventDao) Create(event models.TicketEvent) error {
	dao := newTicketEventDao(event)
	db := dbcommons.GetDb()
	if event.CreatedTime <= 0 {
		db = db.Omit("created_time")
	}
	return db.Create(dao).Error
}

func (d *TicketEventDao) QryByTicket(appkey, ticketId string, limit, offset int64) ([]*models.TicketEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var rows []TicketEventDao
	err := dbcommons.GetDb().Where("app_key=? AND ticket_id=?", appkey, ticketId).
		Order("id desc").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]*models.TicketEvent, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].toModel())
	}
	return items, nil
}

// 工具方法，供 service 测试或补偿场景使用。
func (d *TicketEventDao) deleteById(id int64) error {
	if id == 0 {
		return errors.New("id required")
	}
	return dbcommons.GetDb().Where("id=?", id).Delete(&TicketEventDao{}).Error
}

var _ models.ITicketEventStorage = (*TicketEventDao)(nil)
