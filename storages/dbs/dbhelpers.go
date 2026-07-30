package dbs

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// isErrRecordNotFound 统一封装 GORM ErrRecordNotFound 判定。
func isErrRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// isDuplicateKeyError 判断是否是数据库唯一约束冲突。
//
// 跨方言策略：MySQL 抛 "Duplicate entry ... for key ..." (1062)；
// Postgres 抛 "duplicate key value violates unique constraint" (SQLSTATE 23505)。
// 这里用字符串兜底匹配，避免引入 mysql/pgconn 驱动依赖；DAO 调用者拿到的就是
// 一个 sql.Err 已经被外层 GORM 包装过的字符串。
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "SQLSTATE 23505") ||
		strings.Contains(msg, "unique constraint")
}
