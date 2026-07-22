package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"

	consoleModels "github.com/juggleim/jugglemate-server/console/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"github.com/juggleim/jugglemate-server/storages"
	"github.com/juggleim/jugglemate-server/storages/dbs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"gorm.io/gorm"
)

var accountRegex *regexp.Regexp

func init() {
	accountRegex = regexp.MustCompile(`^[a-zA-Z0-9]{6,20}$`)
}

func checkAccount(account string) bool {
	return accountRegex.MatchString(account)
}

func RoleToStrings(role storageModels.UserRole) []string {
	switch role {
	case storageModels.UserRoleAdmin:
		return []string{"admin"}
	default:
		return []string{"customer"}
	}
}

func RolesToStorageRole(roles []string) storageModels.UserRole {
	for _, r := range roles {
		if r == "admin" {
			return storageModels.UserRoleAdmin
		}
	}
	return storageModels.UserRoleCustomerService
}

func FilterRoleToStorageRole(role string) *storageModels.UserRole {
	switch role {
	case "admin":
		r := storageModels.UserRoleAdmin
		return &r
	case "customer", "editor", "customer_service":
		r := storageModels.UserRoleCustomerService
		return &r
	default:
		return nil
	}
}

func ToUserItem(user *storageModels.User) consoleModels.UserItem {
	email := user.Email
	return consoleModels.UserItem{
		ID:          user.UserId,
		Username:    user.LoginAccount,
		Avatar:      user.Avator,
		Email:       email,
		Roles:       RoleToStrings(user.Role),
		Permissions: []string{},
	}
}

func QryUsers(ctx context.Context, req *consoleModels.QryUsersReq) (errs.IMErrorCode, *consoleModels.UserListResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	if req == nil {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	filter := storageModels.UserListFilter{
		Keyword:   req.Keyword,
		Role:      FilterRoleToStorageRole(req.Role),
		SortField: req.SortField,
		SortOrder: req.SortOrder,
	}

	userStorage := storages.NewUserStorage()
	result, err := userStorage.QryByApp(appkey, filter, limit, offset)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	items := make([]consoleModels.UserItem, 0, len(result.List))
	for _, user := range result.List {
		items = append(items, ToUserItem(user))
	}
	return errs.IMErrorCode_SUCCESS, &consoleModels.UserListResp{
		List:  items,
		Total: result.Total,
	}
}

func CreateUser(ctx context.Context, req *consoleModels.CreateUserReq) (errs.IMErrorCode, *consoleModels.UserItem) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	if req == nil || req.Username == "" || req.Password == "" || len(req.Roles) == 0 {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}
	if !checkAccount(req.Username) || len(req.Password) < 6 {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}

	userStorage := storages.NewUserStorage()
	existing, err := userStorage.FindByAccountWithAppkey(req.Username, appkey)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if existing != nil {
		return errs.IMErrorCode_APP_USER_EXISTED, nil
	}

	userId := dbs.GenerateUserId()
	user := storageModels.User{
		UserId:       userId,
		Nickname:     req.Username,
		LoginAccount: req.Username,
		Email:        req.Email,
		LoginPass:    tools.SHA1(req.Password),
		Role:         RolesToStorageRole(req.Roles),
		AppKey:       appkey,
		Status:       1,
	}
	if err := userStorage.Create(user); err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	sdk := imsdk.GetImSdk(appkey)
	if sdk == nil {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	resp, code, _, err := sdk.Register(juggleimsdk.User{
		UserId:       userId,
		Nickname:     user.Nickname,
		UserPortrait: user.Avator,
	})
	if err != nil || code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) || resp == nil || resp.Token == "" {
		_ = userStorage.Delete(appkey, userId)
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	_ = userStorage.UpdateImToken(appkey, userId, resp.Token)

	created, err := userStorage.FindByUserId(appkey, userId)
	if err != nil || created == nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	item := ToUserItem(created)
	return errs.IMErrorCode_SUCCESS, &item
}

func UpdateUser(ctx context.Context, userId string, req *consoleModels.UpdateUserReq) (errs.IMErrorCode, *consoleModels.UserItem) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" || userId == "" {
		return errs.IMErrorCode_APP_ParamError, nil
	}
	if req == nil {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}

	userStorage := storages.NewUserStorage()
	existing, err := userStorage.FindByUserId(appkey, userId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if existing == nil {
		return errs.IMErrorCode_APP_USER_NOT_EXIST, nil
	}

	updates := storageModels.UserUpdate{}
	if req.Username != "" {
		if !checkAccount(req.Username) {
			return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
		}
		if req.Username != existing.LoginAccount {
			dup, err := userStorage.FindByAccountWithAppkey(req.Username, appkey)
			if err != nil {
				return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
			}
			if dup != nil && dup.UserId != userId {
				return errs.IMErrorCode_APP_USER_EXISTED, nil
			}
		}
		updates.LoginAccount = &req.Username
		nickname := req.Username
		updates.Nickname = &nickname
	}
	if req.Email != existing.Email {
		email := req.Email
		updates.Email = &email
	}
	if len(req.Roles) > 0 {
		role := RolesToStorageRole(req.Roles)
		updates.Role = &role
	}

	if updates.LoginAccount == nil && updates.Email == nil && updates.Role == nil {
		item := ToUserItem(existing)
		return errs.IMErrorCode_SUCCESS, &item
	}

	if err := userStorage.UpdateUser(appkey, userId, updates); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.IMErrorCode_APP_USER_NOT_EXIST, nil
		}
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	updated, err := userStorage.FindByUserId(appkey, userId)
	if err != nil || updated == nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	// TIPS: 昵称改了要同步回 IM，否则群成员列表、消息发送者一直显示旧名字。IM 的 register 是
	// upsert，再调一次即可。这里失败不阻断请求：Console 侧已经改完，且该用户下次登录
	// （services.Login 同样会带最新昵称调 register）会自愈。
	if updates.Nickname != nil {
		syncUserProfileToIM(appkey, updated)
	}
	item := ToUserItem(updated)
	return errs.IMErrorCode_SUCCESS, &item
}

// syncUserProfileToIM 把用户的昵称/头像同步到 IM 侧，失败只记日志。
//
// @param appkey 应用 AppKey
// @param user 已更新的用户资料
func syncUserProfileToIM(appkey string, user *storageModels.User) {
	sdk := imsdk.GetImSdk(appkey)
	if sdk == nil {
		log.Printf("[UpdateUser] 同步用户资料到 IM 失败：无法初始化 SDK appkey=%s user_id=%s", appkey, user.UserId)
		return
	}
	_, code, _, err := sdk.Register(juggleimsdk.User{
		UserId:       user.UserId,
		Nickname:     user.Nickname,
		UserPortrait: user.Avator,
	})
	if err != nil || code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		log.Printf("[UpdateUser] 同步用户资料到 IM 失败 appkey=%s user_id=%s code=%d: %v", appkey, user.UserId, code, err)
	}
}

func DeleteUser(ctx context.Context, userId string) errs.IMErrorCode {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	requesterId := ctxs.GetRequesterIdFromCtx(ctx)
	if appkey == "" || userId == "" {
		return errs.IMErrorCode_APP_ParamError
	}
	if userId == requesterId {
		return errs.IMErrorCode_APP_ParamError
	}

	userStorage := storages.NewUserStorage()
	if err := userStorage.Delete(appkey, userId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.IMErrorCode_APP_USER_NOT_EXIST
		}
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	return errs.IMErrorCode_SUCCESS
}

func ParseQryUsersReq(limit, offset int64, keyword, role, sortField, sortOrder string) *consoleModels.QryUsersReq {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return &consoleModels.QryUsersReq{
		Limit:     limit,
		Offset:    offset,
		Keyword:   keyword,
		Role:      role,
		SortField: sortField,
		SortOrder: sortOrder,
	}
}

func ValidateCreateReq(req *consoleModels.CreateUserReq) error {
	if req == nil || req.Username == "" || req.Password == "" || len(req.Roles) == 0 {
		return fmt.Errorf("invalid create user request")
	}
	if !checkAccount(req.Username) || len(req.Password) < 6 {
		return fmt.Errorf("invalid account or password")
	}
	return nil
}
