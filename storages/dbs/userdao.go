package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"gorm.io/gorm"
)

type UserDao struct {
	ID           int64     `gorm:"primary_key"`
	UserId       string    `gorm:"user_id"`
	Nickname     string    `gorm:"nickname"`
	UserPortrait string    `gorm:"user_portrait"`
	LoginAccount string    `gorm:"login_account"`
	Email        string    `gorm:"email"`
	LoginPass    string    `gorm:"login_pass"`
	Status       int       `gorm:"status"`
	ImToken      string    `gorm:"im_token"`
	CreatedTime  time.Time `gorm:"created_time"`
	UpdatedTime  time.Time `gorm:"updated_time"`
	AppKey       string    `gorm:"app_key"`
}

func (UserDao) TableName() string {
	return "users"
}

func (d *UserDao) FindByAccount(appkey, account string) (*UserDao, error) {
	var item UserDao
	err := dbcommons.GetDb().Where("app_key=? and login_account=?", appkey, account).Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (d *UserDao) FindByUserId(appkey, userId string) (*UserDao, error) {
	var item UserDao
	err := dbcommons.GetDb().Where("app_key=? and user_id=?", appkey, userId).Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (d *UserDao) Create(item *UserDao) error {
	// Set times if not already set
	if item.CreatedTime.IsZero() {
		item.CreatedTime = time.Now()
	}
	if item.UpdatedTime.IsZero() {
		item.UpdatedTime = time.Now()
	}
	return dbcommons.GetDb().Create(item).Error
}

func (d *UserDao) UpdateImToken(appkey, userId, imToken string) error {
	return dbcommons.GetDb().Model(&UserDao{}).
		Where("app_key=? and user_id=?", appkey, userId).
		Updates(map[string]interface{}{
			"im_token":     imToken,
			"updated_time": time.Now(),
		}).Error
}

func (d *UserDao) FindByAccountWithAppkey(account, appkey string) (*UserDao, error) {
	var item UserDao
	err := dbcommons.GetDb().Where("login_account=? and app_key=?", account, appkey).Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func GenerateUserId() string {
	return "u_" + tools.GenerateUUIDShort11()
}
