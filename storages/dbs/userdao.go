package dbs

import (
	"errors"
	"time"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"github.com/juggleim/jugglemate-server/storages/models"
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
	Role         int       `gorm:"role"`
	Status       int       `gorm:"status"`
	ImToken      string    `gorm:"im_token"`
	CreatedTime  time.Time `gorm:"created_time"`
	UpdatedTime  time.Time `gorm:"updated_time"`
	AppKey       string    `gorm:"app_key"`
}

func (UserDao) TableName() string {
	return "users"
}

func (d *UserDao) toModel() *models.User {
	return &models.User{
		ID:           d.ID,
		UserId:       d.UserId,
		Nickname:     d.Nickname,
		UserPortrait: d.UserPortrait,
		LoginAccount: d.LoginAccount,
		Email:        d.Email,
		LoginPass:    d.LoginPass,
		Role:         models.UserRole(d.Role),
		Status:       d.Status,
		ImToken:      d.ImToken,
		CreatedTime:  d.CreatedTime.UnixMilli(),
		UpdatedTime:  d.UpdatedTime.UnixMilli(),
		AppKey:       d.AppKey,
	}
}

func (d *UserDao) FindByAccount(appkey, account string) (*models.User, error) {
	var item UserDao
	err := dbcommons.GetDb().Where("app_key=? and login_account=?", appkey, account).Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

func (d *UserDao) FindByUserId(appkey, userId string) (*models.User, error) {
	var item UserDao
	err := dbcommons.GetDb().Where("app_key=? and user_id=?", appkey, userId).Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

func (d *UserDao) Create(item models.User) error {
	dao := &UserDao{
		UserId:       item.UserId,
		Nickname:     item.Nickname,
		UserPortrait: item.UserPortrait,
		LoginAccount: item.LoginAccount,
		Email:        item.Email,
		LoginPass:    item.LoginPass,
		Role:         int(item.Role),
		Status:       item.Status,
		ImToken:      item.ImToken,
		AppKey:       item.AppKey,
	}
	if item.CreatedTime > 0 {
		dao.CreatedTime = time.UnixMilli(item.CreatedTime)
	} else {
		dao.CreatedTime = time.Now()
	}
	if item.UpdatedTime > 0 {
		dao.UpdatedTime = time.UnixMilli(item.UpdatedTime)
	} else {
		dao.UpdatedTime = time.Now()
	}
	if dao.Role == 0 {
		dao.Role = int(models.UserRoleCustomerService)
	}
	return dbcommons.GetDb().Create(dao).Error
}

func (d *UserDao) UpdateImToken(appkey, userId, imToken string) error {
	return dbcommons.GetDb().Model(&UserDao{}).
		Where("app_key=? and user_id=?", appkey, userId).
		Updates(map[string]interface{}{
			"im_token":     imToken,
			"updated_time": time.Now(),
		}).Error
}

func (d *UserDao) FindByAccountWithAppkey(account, appkey string) (*models.User, error) {
	var item UserDao
	err := dbcommons.GetDb().Where("login_account=? and app_key=?", account, appkey).Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return item.toModel(), nil
}

func GenerateUserId() string {
	return "u_" + tools.GenerateUUIDShort11()
}
