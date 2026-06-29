package services

import (
	"context"
	"strings"

	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
)

const (
	CustomerSourceIDPrefix  string = "customer_"
	ConversationType_Ticket int    = 2
)

var (
	newInboxMemberStorageForCustomer = storages.NewInboxMemberStorage
	newUserStorageForCustomer        = storages.NewUserStorage
	registerIMUserForCustomer        = registerIMUser
)

func registerIMUser(sdk *juggleimsdk.JuggleIMSdk, userId, nickname, portrait string) errs.IMErrorCode {
	resp, code, _, err := sdk.Register(juggleimsdk.User{
		UserId:       userId,
		Nickname:     nickname,
		UserPortrait: portrait,
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		return errs.IMErrorCode(code)
	}
	if resp == nil || resp.Token == "" {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	return errs.IMErrorCode_SUCCESS
}

func buildTicketGroupMemberIds(sourceId string, inboxMemberIds []string) []string {
	memberIds := []string{sourceId}
	seen := map[string]struct{}{sourceId: {}}
	for _, memberId := range inboxMemberIds {
		memberId = strings.TrimSpace(memberId)
		if memberId == "" {
			continue
		}
		if _, ok := seen[memberId]; ok {
			continue
		}
		seen[memberId] = struct{}{}
		memberIds = append(memberIds, memberId)
	}
	return memberIds
}

func prepareTicketGroupMemberIds(
	appkey, inboxId, sourceId string,
	sdk *juggleimsdk.JuggleIMSdk,
	memberStorage storageModels.IInboxMemberStorage,
	userStorage storageModels.IUserStorage,
) (errs.IMErrorCode, []string) {
	members, err := memberStorage.QryByInbox(appkey, inboxId, 0, 1000)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	inboxMemberIds := make([]string, 0, len(members))
	for _, member := range members {
		userId := strings.TrimSpace(member.MemberId)
		if userId == "" || userId == sourceId {
			continue
		}
		user, err := userStorage.FindByUserId(appkey, userId)
		if err != nil {
			return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
		}
		if user == nil {
			continue
		}
		if code := registerIMUserForCustomer(sdk, user.UserId, user.Nickname, user.Avator); code != errs.IMErrorCode_SUCCESS {
			return code, nil
		}
		inboxMemberIds = append(inboxMemberIds, userId)
	}
	return errs.IMErrorCode_SUCCESS, buildTicketGroupMemberIds(sourceId, inboxMemberIds)
}

func validateWidgetInbox(inbox *storageModels.Inbox) errs.IMErrorCode {
	if inbox == nil || inbox.ChannelType != string(ChannelType_Widget) {
		return errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}
	return errs.IMErrorCode_SUCCESS
}

func StartWebCustom(ctx context.Context, req *apiModels.StartCustomReq) (errs.IMErrorCode, *apiModels.StartCustomResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	inboxId := strings.TrimSpace(req.InboxId)
	if appkey == "" || req == nil || req.Identifier == "" || inboxId == "" {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	customerStorage := storages.NewCustomerStorage()
	inboxStorage := storages.NewInboxStorage()
	relStorage := storages.NewCustomerInboxRelStorage()
	ticketStorage := storages.NewTicketStorage()
	nickname := strings.TrimSpace(req.Nickname)

	customer, err := customerStorage.FindByIdentifier(appkey, req.Identifier)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if customer == nil {
		customerId := tools.GenerateUUIDShort22()
		if nickname == "" {
			nickname = tools.GenerateHaikunator()
		}
		customer = &storageModels.Customer{
			CustomerId: customerId,
			Nickname:   nickname,
			Identifier: req.Identifier,
			AppKey:     appkey,
		}
		if err := customerStorage.Create(*customer); err != nil {
			return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
		}
	}

	inbox, err := inboxStorage.FindByInboxId(appkey, inboxId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if code := validateWidgetInbox(inbox); code != errs.IMErrorCode_SUCCESS {
		return code, nil
	}
	widgetConf := ParseWebWidgetChannelConf(inbox.ChannelConf)

	rel, err := relStorage.FindByCustomerInbox(appkey, customer.CustomerId, inbox.InboxId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if rel == nil {
		rel = &storageModels.CustomerInboxRel{
			CustomerId: customer.CustomerId,
			InboxId:    inbox.InboxId,
			SourceId:   CustomerSourceIDPrefix + tools.GenerateUUIDShort22(),
			AppKey:     appkey,
		}
		if err := relStorage.Create(*rel); err != nil {
			return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
		}
	}

	sdk := imsdk.GetImSdk(appkey)
	if sdk == nil {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}

	ticket, err := ticketStorage.FindBySource(appkey, rel.SourceId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	imResp, code, _, err := sdk.Register(juggleimsdk.User{
		UserId:       rel.SourceId,
		Nickname:     customer.Nickname,
		UserPortrait: customer.Avator,
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		return errs.IMErrorCode(code), nil
	}
	if imResp == nil || imResp.Token == "" {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	if ticket == nil {
		ticketId := tools.GenerateUUIDShort22()
		groupName := nickname
		if groupName == "" {
			groupName = customer.Nickname
		}
		memberCode, memberIds := prepareTicketGroupMemberIds(
			appkey,
			inbox.InboxId,
			rel.SourceId,
			sdk,
			newInboxMemberStorageForCustomer(),
			newUserStorageForCustomer(),
		)
		if memberCode != errs.IMErrorCode_SUCCESS {
			return memberCode, nil
		}
		groupCode, _, err := sdk.CreateGroup(juggleimsdk.GroupMembersReq{
			GroupId:   ticketId,
			GroupName: groupName,
			MemberIds: memberIds,
		})
		if err != nil {
			return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
		}
		if groupCode != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
			return errs.IMErrorCode(groupCode), nil
		}
		ticket = &storageModels.Ticket{
			TicketId:   ticketId,
			SourceId:   rel.SourceId,
			CustomerId: customer.CustomerId,
			InboxId:    inbox.InboxId,
			Status:     storageModels.TicketStatusPending,
			AppKey:     appkey,
		}
		if err := ticketStorage.Create(*ticket); err != nil {
			return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
		}
	}

	return errs.IMErrorCode_SUCCESS, &apiModels.StartCustomResp{
		ConversationId:   ticket.TicketId,
		ConversationType: ConversationType_Ticket,
		UserId:           rel.SourceId,
		Nickname:         customer.Nickname,
		ImToken:          imResp.Token,
		WelcomeMessage:   widgetConf.WelcomeMessage,
	}
}
