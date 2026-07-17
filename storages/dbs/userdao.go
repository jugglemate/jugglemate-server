package dbs

import (
	"errors"
	"strings"
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
	Avator       string    `gorm:"avator"`
	LoginAccount string    `gorm:"login_account"`
	Email        *string   `gorm:"email"`
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

// emailPtrForDB 把业务层的 email 转换成入库值。
//
// TIPS: 空 email 必须写空字符串而不是 NULL。users.email 是 NOT NULL DEFAULT ''，写 NULL 会
// 直接违反约束（表现为注册接口报 17006）；唯一索引 uq_users_app_email_nonempty 的条件是
// `WHERE email <> ''`，也说明这张表就是用空串表示“没有邮箱”，多个空串不会互相冲突。
// 返回指针类型是为了兼容历史遗留的 NULL 数据（读取见 emailStringFromDB）。
func emailPtrForDB(email string) *string {
	trimmed := strings.TrimSpace(email)
	return &trimmed
}

func emailStringFromDB(email *string) string {
	if email == nil {
		return ""
	}
	return *email
}

func (d *UserDao) toModel() *models.User {
	return &models.User{
		ID:           d.ID,
		UserId:       d.UserId,
		Nickname:     d.Nickname,
		Avator:       d.Avator,
		LoginAccount: d.LoginAccount,
		Email:        emailStringFromDB(d.Email),
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
		Avator:       item.Avator,
		LoginAccount: item.LoginAccount,
		Email:        emailPtrForDB(item.Email),
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

func (d *UserDao) QryByApp(appkey string, filter models.UserListFilter, limit, offset int64) (*models.UserListResult, error) {
	db := dbcommons.GetDb().Model(&UserDao{}).Where("app_key = ?", appkey)
	if filter.Keyword != "" {
		kw := "%" + filter.Keyword + "%"
		db = db.Where("login_account LIKE ? OR nickname LIKE ? OR email LIKE ?", kw, kw, kw)
	}
	if filter.Role != nil {
		db = db.Where("role = ?", int(*filter.Role))
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	orderCol := userSortColumn(filter.SortField)
	orderDir := "DESC"
	if filter.SortOrder == "ascend" {
		orderDir = "ASC"
	}

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var items []UserDao
	err := db.Order(orderCol + " " + orderDir).Limit(int(limit)).Offset(int(offset)).Find(&items).Error
	if err != nil {
		return nil, err
	}

	list := make([]*models.User, 0, len(items))
	for i := range items {
		list = append(list, items[i].toModel())
	}
	return &models.UserListResult{List: list, Total: total}, nil
}

func userSortColumn(sortField string) string {
	switch sortField {
	case "id":
		return "user_id"
	case "username":
		return "login_account"
	case "email":
		return "email"
	case "roles":
		return "role"
	default:
		return "id"
	}
}

func (d *UserDao) UpdateUser(appkey, userId string, updates models.UserUpdate) error {
	fields := map[string]interface{}{
		"updated_time": time.Now(),
	}
	if updates.LoginAccount != nil {
		fields["login_account"] = *updates.LoginAccount
	}
	if updates.Nickname != nil {
		fields["nickname"] = *updates.Nickname
	}
	if updates.Email != nil {
		fields["email"] = emailPtrForDB(*updates.Email)
	}
	if updates.Role != nil {
		fields["role"] = int(*updates.Role)
	}
	if updates.LoginPass != nil {
		fields["login_pass"] = *updates.LoginPass
	}
	result := dbcommons.GetDb().Model(&UserDao{}).
		Where("app_key=? and user_id=?", appkey, userId).
		Updates(fields)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *UserDao) Delete(appkey, userId string) error {
	result := dbcommons.GetDb().
		Where("app_key=? and user_id=?", appkey, userId).
		Delete(&UserDao{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func GenerateUserId() string {
	return "u_" + tools.GenerateUUIDShort11()
}
