package dbs

import (
	"os"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestCreateUserWithoutEmailAgainstPostgres 验收「注册时不填邮箱」能真正落库。
//
// TIPS: 这个缺陷只有跑在真实 PostgreSQL 上才会暴露 —— emailPtrForDB 的单元测试只看返回值，
// 返回 nil 也能“通过”，但写进 users.email(NOT NULL) 会违反约束，让注册接口静默返回 17006。
func TestCreateUserWithoutEmailAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("AGENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("未设置 AGENT_TEST_POSTGRES_DSN")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 PostgreSQL 失败: %v", err)
	}
	dbcommons.UsePostgres(db)
	t.Cleanup(func() { dbcommons.UsePostgres(nil) })

	const appKey = "test_userdao_appkey"
	cleanup := func() { db.Exec("DELETE FROM users WHERE app_key=?", appKey) }
	cleanup()
	t.Cleanup(cleanup)

	dao := UserDao{}
	// 注册流程构造的 User 不带 Email —— 正是线上注册失败的那条路径。
	if err := dao.Create(storageModels.User{
		UserId: "u_probe_1", Nickname: "probe", LoginAccount: "probe_acc_1",
		LoginPass: "x", Role: storageModels.UserRoleCustomerService, Status: 1, AppKey: appKey,
	}); err != nil {
		t.Fatalf("不填邮箱注册失败（users.email NOT NULL 回归）: %v", err)
	}

	// 同一 AppKey 下允许多个无邮箱账号：唯一索引条件是 WHERE email <> ''。
	if err := dao.Create(storageModels.User{
		UserId: "u_probe_2", Nickname: "probe2", LoginAccount: "probe_acc_2",
		LoginPass: "x", Role: storageModels.UserRoleCustomerService, Status: 1, AppKey: appKey,
	}); err != nil {
		t.Fatalf("第二个无邮箱账号应当允许（部分唯一索引仅约束非空邮箱）: %v", err)
	}

	// 读回来应当是空字符串，对业务层与历史 NULL 数据表现一致。
	got, err := dao.FindByAccountWithAppkey("probe_acc_1", appKey)
	if err != nil || got == nil {
		t.Fatalf("查询失败: user=%v err=%v", got, err)
	}
	if got.Email != "" {
		t.Fatalf("无邮箱账号读回应为空字符串，实际 %q", got.Email)
	}

	// 真实邮箱仍要保证同 AppKey 下唯一。
	if err := dao.Create(storageModels.User{
		UserId: "u_probe_3", Nickname: "p3", LoginAccount: "probe_acc_3",
		LoginPass: "x", Role: storageModels.UserRoleCustomerService, Status: 1, AppKey: appKey,
		Email: "dup@example.com",
	}); err != nil {
		t.Fatalf("创建带邮箱账号失败: %v", err)
	}
	if err := dao.Create(storageModels.User{
		UserId: "u_probe_4", Nickname: "p4", LoginAccount: "probe_acc_4",
		LoginPass: "x", Role: storageModels.UserRoleCustomerService, Status: 1, AppKey: appKey,
		Email: "dup@example.com",
	}); err == nil {
		t.Fatal("同一 AppKey 下重复邮箱应当被唯一索引拒绝")
	}
}
