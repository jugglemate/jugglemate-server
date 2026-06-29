package services

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	juggleimapi "github.com/juggleim/jugglemate-server/commons/juggleim"
	telegramapi "github.com/juggleim/jugglemate-server/commons/telegram"
	"github.com/juggleim/jugglemate-server/commons/tools"
	consoleModels "github.com/juggleim/jugglemate-server/console/apis/models"
	appServices "github.com/juggleim/jugglemate-server/services"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

const maxWidgetWelcomeMessageLen = 500

var (
	newInboxStorageForConsole       = storages.NewInboxStorage
	newInboxMemberStorageForConsole = storages.NewInboxMemberStorage
	newUserStorageForConsoleInbox   = storages.NewUserStorage
	telegramGetMeForConsole         = func(botToken string) (*telegramapi.BotInfo, error) {
		return telegramapi.NewClient().GetMe(botToken)
	}
	telegramDeleteWebhookForConsole = func(botToken string) error {
		return telegramapi.NewClient().DeleteWebhook(botToken)
	}
	telegramSetWebhookForConsole = func(botToken, callbackURL string) error {
		return telegramapi.NewClient().SetWebhook(botToken, callbackURL)
	}
	juggleIMSetupBotForConsole = func(botName, botToken, callbackURL string) (*juggleimapi.BotInfo, error) {
		return juggleimapi.NewClient().SetupBot(botName, botToken, callbackURL)
	}
)

func QryInboxes(ctx context.Context, limit, offset int64) (errs.IMErrorCode, *consoleModels.InboxListResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	inboxStorage := newInboxStorageForConsole()
	result, err := inboxStorage.QryByApp(appkey, "", limit, offset)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	inboxIds := make([]string, 0, len(result.List))
	for _, inbox := range result.List {
		inboxIds = append(inboxIds, inbox.InboxId)
	}
	memberCounts, err := newInboxMemberStorageForConsole().CountByInboxes(appkey, inboxIds)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	items := make([]consoleModels.InboxItem, 0, len(result.List))
	for _, inbox := range result.List {
		items = append(items, ToInboxItem(inbox, memberCounts[inbox.InboxId]))
	}
	return errs.IMErrorCode_SUCCESS, &consoleModels.InboxListResp{List: items, Total: result.Total}
}

func CreateTelegramInbox(ctx context.Context, req *consoleModels.CreateTelegramInboxReq) (errs.IMErrorCode, *consoleModels.InboxItem) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	if req == nil {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}
	name := strings.TrimSpace(req.Name)
	botName := strings.TrimSpace(req.BotName)
	botToken := strings.TrimSpace(req.BotToken)
	if name == "" || botName == "" || botToken == "" {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(configures.Config.JmateBaseUrl), "/")
	if baseURL == "" {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}
	botInfo, err := telegramGetMeForConsole(botToken)
	if err != nil {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}
	if botInfo != nil && strings.TrimSpace(botInfo.Username) != "" {
		botName = strings.TrimSpace(botInfo.Username)
	}

	inboxId := tools.GenerateUUIDShort22()
	callbackURL := telegramWebhookCallbackURL(baseURL, inboxId)
	if err := telegramDeleteWebhookForConsole(botToken); err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if err := telegramSetWebhookForConsole(botToken, callbackURL); err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	channelConf, err := json.Marshal(appServices.TelegramChannelConf{
		BotName:  botName,
		BotToken: botToken,
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	inbox := storageModels.Inbox{
		InboxId:     inboxId,
		ChannelType: string(appServices.ChannelType_Telegram),
		ChannelConf: string(channelConf),
		Name:        name,
		AppKey:      appkey,
	}
	if err := newInboxStorageForConsole().Create(inbox); err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	item := ToInboxItem(&inbox, 0)
	return errs.IMErrorCode_SUCCESS, &item
}

func telegramWebhookCallbackURL(baseURL, inboxId string) string {
	return strings.TrimRight(baseURL, "/") + "/jmate/webhooks/telegram/" + inboxId
}

func CreateJuggleIMInbox(ctx context.Context, req *consoleModels.CreateJuggleIMInboxReq) (errs.IMErrorCode, *consoleModels.InboxItem) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	if req == nil {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}
	name := strings.TrimSpace(req.Name)
	botName := strings.TrimSpace(req.BotName)
	botToken := strings.TrimSpace(req.BotToken)
	if name == "" || botName == "" || botToken == "" {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(configures.Config.JmateBaseUrl), "/")
	if baseURL == "" {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}

	inboxId := tools.GenerateUUIDShort22()
	callbackURL := juggleIMWebhookCallbackURL(baseURL, inboxId)
	botInfo, err := juggleIMSetupBotForConsole(botName, botToken, callbackURL)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if botInfo != nil && strings.TrimSpace(botInfo.Username) != "" {
		botName = strings.TrimSpace(botInfo.Username)
	}

	channelConf, err := json.Marshal(appServices.JuggleIMChannelConf{
		BotName:  botName,
		BotToken: botToken,
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	inbox := storageModels.Inbox{
		InboxId:     inboxId,
		ChannelType: string(appServices.ChannelType_JuggleIM),
		ChannelConf: string(channelConf),
		Name:        name,
		AppKey:      appkey,
	}
	if err := newInboxStorageForConsole().Create(inbox); err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	item := ToInboxItem(&inbox, 0)
	return errs.IMErrorCode_SUCCESS, &item
}

func juggleIMWebhookCallbackURL(baseURL, inboxId string) string {
	return strings.TrimRight(baseURL, "/") + "/jmate/webhooks/juggleim/" + inboxId
}

func CreateWidgetInbox(ctx context.Context, req *consoleModels.CreateWidgetInboxReq) (errs.IMErrorCode, *consoleModels.InboxItem) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	if req == nil {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}
	welcomeMessage := strings.TrimSpace(req.WelcomeMessage)
	if len(welcomeMessage) > maxWidgetWelcomeMessageLen {
		return errs.IMErrorCode_APP_REQ_BODY_ILLEGAL, nil
	}

	channelConf, err := json.Marshal(appServices.WebWidgetChannelConf{
		WelcomeMessage: welcomeMessage,
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	inbox := storageModels.Inbox{
		InboxId:     tools.GenerateUUIDShort22(),
		ChannelType: string(appServices.ChannelType_Widget),
		ChannelConf: string(channelConf),
		Name:        name,
		AppKey:      appkey,
	}
	if err := newInboxStorageForConsole().Create(inbox); err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	item := ToInboxItem(&inbox, 0)
	return errs.IMErrorCode_SUCCESS, &item
}

func QryInboxMembers(ctx context.Context, inboxId string) (errs.IMErrorCode, *consoleModels.InboxMembersResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	if inboxId == "" {
		return errs.IMErrorCode_APP_ParamError, nil
	}
	if ok, code := inboxExists(appkey, inboxId); !ok {
		return code, nil
	}

	members, err := newInboxMemberStorageForConsole().QryByInbox(appkey, inboxId, 0, 1000)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	userStorage := newUserStorageForConsoleInbox()
	items := make([]consoleModels.InboxMemberItem, 0, len(members))
	for _, member := range members {
		user, err := userStorage.FindByUserId(appkey, member.MemberId)
		if err != nil {
			return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
		}
		if user == nil {
			continue
		}
		items = append(items, toInboxMemberItem(user))
	}
	return errs.IMErrorCode_SUCCESS, &consoleModels.InboxMembersResp{List: items}
}

func ReplaceInboxMembers(ctx context.Context, inboxId string, req *consoleModels.ReplaceInboxMembersReq) errs.IMErrorCode {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_EXISTED
	}
	if inboxId == "" || req == nil {
		return errs.IMErrorCode_APP_ParamError
	}
	if ok, code := inboxExists(appkey, inboxId); !ok {
		return code
	}

	memberIds, code := normalizeAndValidateMemberIds(appkey, req.UserIds)
	if code != errs.IMErrorCode_SUCCESS {
		return code
	}
	if err := newInboxMemberStorageForConsole().ReplaceByInbox(appkey, inboxId, memberIds); err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	return errs.IMErrorCode_SUCCESS
}

func ToInboxItem(inbox *storageModels.Inbox, memberCount int64) consoleModels.InboxItem {
	item := consoleModels.InboxItem{
		ID:          inbox.InboxId,
		Name:        inbox.Name,
		ChannelType: inbox.ChannelType,
		MemberCount: memberCount,
		CreatedTime: inbox.CreatedTime,
		UpdatedTime: inbox.UpdatedTime,
	}
	switch inbox.ChannelType {
	case string(appServices.ChannelType_Telegram):
		conf := consoleModels.TelegramConfigItem{}
		var stored appServices.TelegramChannelConf
		if err := json.Unmarshal([]byte(inbox.ChannelConf), &stored); err == nil {
			conf.BotName = stored.BotName
		}
		item.ChannelConf = conf
	case string(appServices.ChannelType_JuggleIM):
		conf := consoleModels.JuggleIMConfigItem{}
		var stored appServices.JuggleIMChannelConf
		if err := json.Unmarshal([]byte(inbox.ChannelConf), &stored); err == nil {
			conf.BotName = stored.BotName
		}
		item.ChannelConf = conf
	case string(appServices.ChannelType_Widget):
		conf := appServices.ParseWebWidgetChannelConf(inbox.ChannelConf)
		item.ChannelConf = consoleModels.WidgetConfigItem{
			WelcomeMessage: conf.WelcomeMessage,
		}
	default:
		item.ChannelConf = map[string]string{}
	}
	return item
}

func inboxExists(appkey, inboxId string) (bool, errs.IMErrorCode) {
	inbox, err := newInboxStorageForConsole().FindByInboxId(appkey, inboxId)
	if err != nil {
		return false, errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if inbox == nil {
		return false, errs.IMErrorCode_APP_ParamError
	}
	switch inbox.ChannelType {
	case string(appServices.ChannelType_Telegram), string(appServices.ChannelType_Widget), string(appServices.ChannelType_JuggleIM):
		return true, errs.IMErrorCode_SUCCESS
	default:
		return false, errs.IMErrorCode_APP_ParamError
	}
}

func normalizeAndValidateMemberIds(appkey string, userIds []string) ([]string, errs.IMErrorCode) {
	userStorage := newUserStorageForConsoleInbox()
	seen := map[string]struct{}{}
	memberIds := make([]string, 0, len(userIds))
	for _, userId := range userIds {
		userId = strings.TrimSpace(userId)
		if userId == "" {
			return nil, errs.IMErrorCode_APP_REQ_BODY_ILLEGAL
		}
		if _, ok := seen[userId]; ok {
			continue
		}
		user, err := userStorage.FindByUserId(appkey, userId)
		if err != nil {
			return nil, errs.IMErrorCode_APP_INTERNAL_TIMEOUT
		}
		if user == nil {
			return nil, errs.IMErrorCode_APP_ParamError
		}
		seen[userId] = struct{}{}
		memberIds = append(memberIds, userId)
	}
	return memberIds, errs.IMErrorCode_SUCCESS
}

func toInboxMemberItem(user *storageModels.User) consoleModels.InboxMemberItem {
	return consoleModels.InboxMemberItem{
		ID:       user.UserId,
		Username: user.LoginAccount,
		Avatar:   user.Avator,
		Email:    user.Email,
	}
}
