package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/gorm"
)

type InboxDao struct {
	ID          int64     `gorm:"primary_key"`
	InboxId     string    `gorm:"inbox_id"`
	ChannelType string    `gorm:"channel_type"`
	ChannelConf string    `gorm:"channel_conf"`
	Name        string    `gorm:"name"`
	CreatedTime time.Time `gorm:"created_time"`
	UpdatedTime time.Time `gorm:"updated_time"`
	AppKey      string    `gorm:"app_key"`
}

func (InboxDao) TableName() string {
	return "inboxes"
}

func (d *InboxDao) toModel() *models.Inbox {
	return &models.Inbox{
		ID:          d.ID,
		InboxId:     d.InboxId,
		ChannelType: d.ChannelType,
		ChannelConf: d.ChannelConf,
		Name:        d.Name,
		CreatedTime: d.CreatedTime.UnixMilli(),
		UpdatedTime: d.UpdatedTime.UnixMilli(),
		AppKey:      d.AppKey,
	}
}

func newInboxDao(item models.Inbox) *InboxDao {
	dao := &InboxDao{
		InboxId:     item.InboxId,
		ChannelType: item.ChannelType,
		ChannelConf: item.ChannelConf,
		Name:        item.Name,
		AppKey:      item.AppKey,
	}
	if item.CreatedTime > 0 {
		dao.CreatedTime = time.UnixMilli(item.CreatedTime)
	}
	if item.UpdatedTime > 0 {
		dao.UpdatedTime = time.UnixMilli(item.UpdatedTime)
	}
	return dao
}

func (d *InboxDao) Create(item models.Inbox) error {
	dao := newInboxDao(item)
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

func (d *InboxDao) Update(item models.Inbox) error {
	return dbcommons.GetDb().Model(&InboxDao{}).
		Where("app_key=? and inbox_id=?", item.AppKey, item.InboxId).
		Updates(map[string]interface{}{
			"channel_type": item.ChannelType,
			"channel_conf": item.ChannelConf,
			"name":         item.Name,
			"updated_time": time.Now(),
		}).Error
}

func (d *InboxDao) Upsert(item models.Inbox) error {
	existing, err := d.FindByInboxId(item.AppKey, item.InboxId)
	if err != nil {
		return err
	}
	if existing == nil {
		return d.Create(item)
	}
	return d.Update(item)
}

func (d *InboxDao) Delete(appkey, inboxId string) error {
	return dbcommons.GetDb().
		Where("app_key=? and inbox_id=?", appkey, inboxId).
		Delete(&InboxDao{}).Error
}

func (d *InboxDao) FindByInboxId(appkey, inboxId string) (*models.Inbox, error) {
	var item InboxDao
	err := dbcommons.GetDb().
		Where("app_key=? and inbox_id=?", appkey, inboxId).
		Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

func (d *InboxDao) FindByInboxIdAny(inboxId string) ([]*models.Inbox, error) {
	var items []InboxDao
	err := dbcommons.GetDb().
		Where("inbox_id=?", inboxId).
		Limit(2).
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.Inbox, 0, len(items))
	for i := range items {
		ret = append(ret, items[i].toModel())
	}
	return ret, nil
}

func (d *InboxDao) QryByApp(appkey, channelType string, limit, offset int64) (*models.InboxListResult, error) {
	db := dbcommons.GetDb().Model(&InboxDao{}).Where("app_key=?", appkey)
	if channelType != "" {
		db = db.Where("channel_type=?", channelType)
	} else {
		db = db.Where("channel_type IN ?", []string{"widget", "telegram", "juggleim"})
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var items []InboxDao
	err := db.Order("id desc").Limit(int(limit)).Offset(int(offset)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.Inbox, 0, len(items))
	for i := range items {
		ret = append(ret, items[i].toModel())
	}
	return &models.InboxListResult{List: ret, Total: total}, nil
}

func (d *InboxDao) QryByChannelType(appkey, channelType string, startId, limit int64) ([]*models.Inbox, error) {
	db := dbcommons.GetDb().Where("app_key=? and channel_type=?", appkey, channelType)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	return queryInboxes(db, limit)
}

func queryInboxes(db *gorm.DB, limit int64) ([]*models.Inbox, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []InboxDao
	err := db.Order("id desc").Limit(int(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.Inbox, 0, len(items))
	for _, item := range items {
		ret = append(ret, item.toModel())
	}
	return ret, nil
}

type InboxMemberDao struct {
	ID          int64     `gorm:"primary_key"`
	InboxId     string    `gorm:"inbox_id"`
	MemberId    string    `gorm:"member_id"`
	CreatedTime time.Time `gorm:"created_time"`
	AppKey      string    `gorm:"app_key"`
}

func (InboxMemberDao) TableName() string {
	return "inboxmembers"
}

func (d *InboxMemberDao) toModel() *models.InboxMember {
	return &models.InboxMember{
		ID:          d.ID,
		InboxId:     d.InboxId,
		MemberId:    d.MemberId,
		CreatedTime: d.CreatedTime.UnixMilli(),
		AppKey:      d.AppKey,
	}
}

func newInboxMemberDao(item models.InboxMember) *InboxMemberDao {
	dao := &InboxMemberDao{
		InboxId:  item.InboxId,
		MemberId: item.MemberId,
		AppKey:   item.AppKey,
	}
	if item.CreatedTime > 0 {
		dao.CreatedTime = time.UnixMilli(item.CreatedTime)
	}
	return dao
}

func (d *InboxMemberDao) Create(item models.InboxMember) error {
	dao := newInboxMemberDao(item)
	db := dbcommons.GetDb()
	if item.CreatedTime <= 0 {
		db = db.Omit("created_time")
	}
	return db.Create(dao).Error
}

func (d *InboxMemberDao) Upsert(item models.InboxMember) error {
	existing, err := d.Find(item.AppKey, item.InboxId, item.MemberId)
	if err != nil {
		return err
	}
	if existing == nil {
		return d.Create(item)
	}
	return nil
}

func (d *InboxMemberDao) Delete(appkey, inboxId, memberId string) error {
	return dbcommons.GetDb().
		Where("app_key=? and inbox_id=? and member_id=?", appkey, inboxId, memberId).
		Delete(&InboxMemberDao{}).Error
}

func (d *InboxMemberDao) Find(appkey, inboxId, memberId string) (*models.InboxMember, error) {
	var item InboxMemberDao
	err := dbcommons.GetDb().
		Where("app_key=? and inbox_id=? and member_id=?", appkey, inboxId, memberId).
		Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

func (d *InboxMemberDao) QryByInbox(appkey, inboxId string, startId, limit int64) ([]*models.InboxMember, error) {
	db := dbcommons.GetDb().Where("app_key=? and inbox_id=?", appkey, inboxId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	return queryInboxMembers(db, limit)
}

func (d *InboxMemberDao) QryByMember(appkey, memberId string, startId, limit int64) ([]*models.InboxMember, error) {
	db := dbcommons.GetDb().Where("app_key=? and member_id=?", appkey, memberId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	return queryInboxMembers(db, limit)
}

func (d *InboxMemberDao) CountByInboxes(appkey string, inboxIds []string) (map[string]int64, error) {
	ret := make(map[string]int64, len(inboxIds))
	if len(inboxIds) == 0 {
		return ret, nil
	}
	type row struct {
		InboxId string
		Count   int64
	}
	var rows []row
	err := dbcommons.GetDb().Model(&InboxMemberDao{}).
		Select("inbox_id, count(*) as count").
		Where("app_key=? and inbox_id in ?", appkey, inboxIds).
		Group("inbox_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		ret[r.InboxId] = r.Count
	}
	return ret, nil
}

func (d *InboxMemberDao) ReplaceByInbox(appkey, inboxId string, memberIds []string) error {
	return dbcommons.GetDb().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_key=? and inbox_id=?", appkey, inboxId).Delete(&InboxMemberDao{}).Error; err != nil {
			return err
		}
		if len(memberIds) == 0 {
			return nil
		}
		items := make([]InboxMemberDao, 0, len(memberIds))
		for _, memberId := range memberIds {
			items = append(items, InboxMemberDao{
				InboxId:  inboxId,
				MemberId: memberId,
				AppKey:   appkey,
			})
		}
		return tx.Omit("created_time").Create(&items).Error
	})
}

func queryInboxMembers(db *gorm.DB, limit int64) ([]*models.InboxMember, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []InboxMemberDao
	err := db.Order("id desc").Limit(int(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.InboxMember, 0, len(items))
	for _, item := range items {
		ret = append(ret, item.toModel())
	}
	return ret, nil
}
