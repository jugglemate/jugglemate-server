package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/gorm"
)

// ErrTicketRatingAlreadyExists 是 ticket_ratings UNIQUE 冲突的语义化错误。
//
// DAO 层捕获 GORM/数据库的 duplicate-key / 1062 等失败并翻译为该错误；service
// 层用 errors.Is(err, dbs.ErrTicketRatingAlreadyExists) 判定是否"已评过"，避免依赖
// 不同方言的 error 字符串子串匹配。
var ErrTicketRatingAlreadyExists = errors.New("ticket_ratings: already exists")

type TicketRatingDao struct {
	ID          int64     `gorm:"primary_key"`
	AppKey      string    `gorm:"app_key"`
	TicketId    string    `gorm:"ticket_id"`
	CustomerId  string    `gorm:"customer_id"`
	AssigneeId  string    `gorm:"assignee_id"`
	Rating      int       `gorm:"rating"`
	Comment     string    `gorm:"comment"`
	Source      string    `gorm:"source"`
	CreatedTime time.Time `gorm:"created_time"`
}

func (TicketRatingDao) TableName() string {
	return "ticket_ratings"
}

func (d *TicketRatingDao) toModel() *models.TicketRating {
	if d == nil {
		return nil
	}
	return &models.TicketRating{
		ID:          d.ID,
		AppKey:      d.AppKey,
		TicketId:    d.TicketId,
		CustomerId:  d.CustomerId,
		AssigneeId:  d.AssigneeId,
		Rating:      d.Rating,
		Comment:     d.Comment,
		Source:      models.TicketRatingSource(d.Source),
		CreatedTime: d.CreatedTime.UnixMilli(),
	}
}

func newTicketRatingDao(item models.TicketRating) *TicketRatingDao {
	dao := &TicketRatingDao{
		AppKey:     item.AppKey,
		TicketId:   item.TicketId,
		CustomerId: item.CustomerId,
		AssigneeId: item.AssigneeId,
		Rating:     item.Rating,
		Comment:    item.Comment,
		Source:     string(item.Source),
	}
	if item.CreatedTime > 0 {
		dao.CreatedTime = time.UnixMilli(item.CreatedTime)
	}
	return dao
}

// Create 写评价；UNIQUE 冲突返回 ErrTicketRatingAlreadyExists。
//
// 跨方言策略：Postgres 抛 *pgconn.PgError (SQLSTATE 23505)，
// MySQL 抛 *mysql.MySQLError Number 1062。两者都译为 ErrTicketRatingAlreadyExists。
func (d *TicketRatingDao) Create(item models.TicketRating) error {
	dao := newTicketRatingDao(item)
	if dao.CreatedTime.IsZero() {
		dao.CreatedTime = time.Now()
	}
	err := dbcommons.GetDb().Create(dao).Error
	if err == nil {
		return nil
	}
	if isDuplicateKeyError(err) {
		return ErrTicketRatingAlreadyExists
	}
	return err
}

func (d *TicketRatingDao) FindByTicketAndCustomer(appkey, ticketId, customerId string) (*models.TicketRating, error) {
	var row TicketRatingDao
	err := dbcommons.GetDb().
		Where("app_key=? AND ticket_id=? AND customer_id=?", appkey, ticketId, customerId).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return row.toModel(), nil
}

// QryCsatByAssigneeAndTime 按坐席聚合 csat_avg + rating_count。
//
// TIPS: 用 Postgres 的 AVG / COUNT 直接在 SQL 端算，service 层只做 Go 端 map
// 拼接；MySQL 路径在同一 raw SQL 上语义一致。
func (d *TicketRatingDao) QryCsatByAssigneeAndTime(appkey string, startMs, endMs int64) (map[string]models.CsatAggregate, error) {
	type row struct {
		AssigneeID  string
		CsatAvg     *float64
		RatingCount int64
	}
	var rows []row
	start := time.UnixMilli(startMs)
	end := time.UnixMilli(endMs)
	err := dbcommons.GetDb().Raw(`
		SELECT
		  assignee_id,
		  AVG(rating)::float                AS csat_avg,
		  COUNT(*)                          AS rating_count
		FROM ticket_ratings
		WHERE app_key = ?
		  AND assignee_id <> ''
		  AND created_time >= ? AND created_time < ?
		GROUP BY assignee_id`, appkey, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]models.CsatAggregate, len(rows))
	for _, r := range rows {
		out[r.AssigneeID] = models.CsatAggregate{
			AssigneeID:  r.AssigneeID,
			CsatAvg:     r.CsatAvg,
			RatingCount: r.RatingCount,
		}
	}
	return out, nil
}

var _ models.ITicketRatingStorage = (*TicketRatingDao)(nil)
