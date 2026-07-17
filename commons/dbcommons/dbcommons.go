package dbcommons

import "gorm.io/gorm"

var db *gorm.DB

// UsePostgres 注入由 Agent 模块统一管理的 PostgreSQL 连接池。
//
// TIPS: 客服域 DAO 不拥有连接生命周期，关闭动作只由 Agent 模块执行，避免共享连接被重复关闭。
func UsePostgres(postgres *gorm.DB) {
	db = postgres
}

func GetDb() *gorm.DB {
	return db
}

func CloseDB() {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	sqlDB.Close()
}
