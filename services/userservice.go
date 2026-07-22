package services

import (
	"context"
	"fmt"
	"log"
	"regexp"

	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"github.com/juggleim/jugglemate-server/storages"
	"github.com/juggleim/jugglemate-server/storages/dbs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
)

var accountRegex *regexp.Regexp

func init() {
	accountRegex = regexp.MustCompile(`^[a-zA-Z0-9]{6,20}$`)
}

func checkAccount(account string) bool {
	return accountRegex.MatchString(account)
}

// rollbackRegisteredUser 在 IM 注册失败后删掉刚落库的用户。
//
// TIPS: 不回滚会留下"库里有、IM 侧没有"的僵尸账号，代价有两层：本人重试注册直接撞
// USER_EXISTED 再也注册不上；该账号一旦被加进 Inbox，转人工时还要为它做一次补注册
// （见 ensureSeatIMRegistered）。这里与 console 的 CreateUser 保持同样的处理。
//
// @param userStorage 用户存储
// @param appkey 应用 AppKey
// @param userId 待回滚的用户 ID
func rollbackRegisteredUser(userStorage storageModels.IUserStorage, appkey, userId string) {
	if err := userStorage.Delete(appkey, userId); err != nil {
		log.Printf("[Register] IM 注册失败后回滚用户失败 appkey=%s user_id=%s: %v", appkey, userId, err)
	}
}

func Register(ctx context.Context, account, password string) (errs.IMErrorCode, *apiModels.LoginResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	if !checkAccount(account) {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}
	if len(password) < 6 {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}

	userStorage := storages.NewUserStorage()
	existing, err := userStorage.FindByAccountWithAppkey(account, appkey)
	if err != nil {
		return errs.IMErrorCode_APP_USER_EXISTED, nil
	}
	if existing != nil {
		return errs.IMErrorCode_APP_USER_EXISTED, nil
	}

	userId := dbs.GenerateUserId()
	nickname := fmt.Sprintf("user%05d", tools.RandInt(100000))

	user := storageModels.User{
		UserId:       userId,
		Nickname:     nickname,
		LoginAccount: account,
		LoginPass:    tools.SHA1(password),
		Role:         storageModels.UserRoleCustomerService,
		AppKey:       appkey,
		Status:       1,
	}
	if err := userStorage.Create(user); err != nil {
		// TIPS: 这里原来静默返回 17006，注册失败时日志里没有任何线索。
		log.Printf("[Register] 创建用户失败 appkey=%s account=%s user_id=%s: %v", appkey, account, userId, err)
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	// Register to IM server to get token
	fmt.Printf("[Register] Start: appkey=%s, userId=%s\n", appkey, userId)
	sdk := imsdk.GetImSdk(appkey)
	if sdk == nil {
		fmt.Printf("[Register] GetImSdk failed for appkey: %s\n", appkey)
		rollbackRegisteredUser(userStorage, appkey, userId)
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	fmt.Printf("[Register] GetImSdk success, calling Register API\n")
	resp, code, _, err := sdk.Register(juggleimsdk.User{
		UserId:       userId,
		Nickname:     nickname,
		UserPortrait: "",
	})
	fmt.Printf("[Register] resp=%+v, code=%d, err=%v\n", resp, code, err)
	if err != nil {
		fmt.Printf("[Register] IM Register failed: appkey=%s, userId=%s, err=%v\n", appkey, userId, err)
		rollbackRegisteredUser(userStorage, appkey, userId)
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		fmt.Printf("[Register] IM Register returned error code: %d\n", code)
		rollbackRegisteredUser(userStorage, appkey, userId)
		return errs.IMErrorCode(code), nil
	}
	if resp == nil || resp.Token == "" {
		fmt.Printf("[Register] IM Register returned empty token\n")
		rollbackRegisteredUser(userStorage, appkey, userId)
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	// Save IM token
	_ = userStorage.UpdateImToken(appkey, userId, resp.Token)

	authorization, err := GenerateToken(appkey, userId, int32(user.Role))
	if err != nil {
		fmt.Printf("[Register] GenerateToken failed: appkey=%s, userId=%s, err=%v\n", appkey, userId, err)
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	return errs.IMErrorCode_SUCCESS, &apiModels.LoginResp{
		UserId:        userId,
		NickName:      nickname,
		Avatar:        "",
		Role:          int(user.Role),
		Authorization: authorization,
		ImToken:       resp.Token,
	}
}

func Login(ctx context.Context, account, password string) (errs.IMErrorCode, *apiModels.LoginResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}

	userStorage := storages.NewUserStorage()
	user, err := userStorage.FindByAccountWithAppkey(account, appkey)
	if err != nil || user == nil {
		return errs.IMErrorCode_APP_USER_NOT_EXIST, nil
	}

	if user.LoginPass != tools.SHA1(password) {
		return errs.IMErrorCode_APP_LOGIN_ERR_PASS, nil
	}

	// Register to IM server to get fresh token
	sdk := imsdk.GetImSdk(appkey)
	if sdk == nil {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	resp, code, _, err := sdk.Register(juggleimsdk.User{
		UserId:       user.UserId,
		Nickname:     user.Nickname,
		UserPortrait: user.Avator,
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		return errs.IMErrorCode(code), nil
	}

	// Update and return saved IM token
	_ = userStorage.UpdateImToken(appkey, user.UserId, resp.Token)

	authorization, err := GenerateToken(appkey, user.UserId, int32(user.Role))
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	return errs.IMErrorCode_SUCCESS, &apiModels.LoginResp{
		UserId:        user.UserId,
		NickName:      user.Nickname,
		Avatar:        user.Avator,
		Role:          int(user.Role),
		Authorization: authorization,
		ImToken:       resp.Token,
	}
}
