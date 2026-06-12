package dbcommons

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var db *gorm.DB

type GlobalConfDao struct {
	ID         int64  `gorm:"primary_key"`
	ConfKey    string `gorm:"conf_key"`
	ConfValue  string `gorm:"conf_value"`
	CreaterId  string `gorm:"creater_id"`
	CreateTime time.Time `gorm:"create_time"`
	UpdateTime time.Time `gorm:"update_time"`
}

func (GlobalConfDao) TableName() string {
	return "global_conf"
}

func (d *GlobalConfDao) FindByKey(key string) (*GlobalConfDao, error) {
	var item GlobalConfDao
	err := GetDb().Where("conf_key=?", key).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (d *GlobalConfDao) Upsert(item GlobalConfDao) error {
	return GetDb().Where("conf_key=?", item.ConfKey).Assign(item).FirstOrCreate(&item).Error
}

func InitMysql() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		"",
		"",
		"",
		"",
	)

	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return nil
}

func InitMysqlWithConfig(user, password, address, dbName string, debug bool) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, address, dbName)

	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	if debug {
		db = db.Debug()
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return nil
}

func GetDb() *gorm.DB {
	return db
}

type CountResult struct {
	Count int64 `gorm:"count"`
}