package dbcommons

import (
	"fmt"
	"time"

	"github.com/juggleim/jugglemate-server/commons/configures"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var db *gorm.DB

func InitMysql() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		configures.Config.Mysql.User,
		configures.Config.Mysql.Password,
		configures.Config.Mysql.Address,
		configures.Config.Mysql.DbName)
	logMode := logger.Silent
	if configures.Config.Mysql.Debug {
		logMode = logger.Info
	}
	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
		Logger:         logger.Default.LogMode(logMode),
	})
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

func GetDb() *gorm.DB {
	return db
}

func CloseDB() {
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	sqlDB.Close()
}
