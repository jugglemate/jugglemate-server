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
	TicketIDPrefix          string = "ticket_"
	ConversationType_Ticket int    = 2
)

var (
	newInboxMemberStorageForCustomer      = storages.NewInboxMemberStorage
	newUserStorageForCustomer             = storages.NewUserStorage
	newCustomerStorageForCustomer         = storages.NewCustomerStorage
	newInboxStorageForCustomer            = storages.NewInboxStorage
	newCustomerInboxRelStorageForCustomer = storages.NewCustomerInboxRelStorage
	newTicketStorageForCustomer           = storages.NewTicketStorage
	getImSdkForCustomer                   = imsdk.GetImSdk
	registerIMUserForCustomer             = registerIMUser
	registerCustomerIMUserForCustomer     = registerCustomerIMUser
	createGroupForCustomer                = func(sdk *juggleimsdk.JuggleIMSdk, req juggleimsdk.GroupMembersReq) (juggleimsdk.ApiCode, string, error) {
		return sdk.CreateGroup(req)
	}
	resolveInboxAgentBotForCustomer             = ResolveInboxAgentBot
	syncTicketGlobalConversationTagsForCustomer = SyncTicketGlobalConversationTags
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

func registerCustomerIMUser(sdk *juggleimsdk.JuggleIMSdk, userId, nickname, portrait string) (errs.IMErrorCode, string) {
	resp, code, _, err := sdk.Register(juggleimsdk.User{
		UserId:       userId,
		Nickname:     nickname,
		UserPortrait: portrait,
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, ""
	}
	if code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		return errs.IMErrorCode(code), ""
	}
	if resp == nil || resp.Token == "" {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, ""
	}
	return errs.IMErrorCode_SUCCESS, resp.Token
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

func generateTicketId() string {
	return TicketIDPrefix + tools.GenerateUUIDShort22()
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
	// TIPS: Ticket 才是实际 IM 群；建群时直接加入 Inbox 当前 Agent Bot，避免创建后短暂漏接消息。
	agentBotUserID, err := resolveInboxAgentBotForCustomer(context.Background(), appkey, inboxId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if strings.TrimSpace(agentBotUserID) != "" {
		inboxMemberIds = append(inboxMemberIds, agentBotUserID)
	}
	return errs.IMErrorCode_SUCCESS, buildTicketGroupMemberIds(sourceId, inboxMemberIds)
}

func validateWidgetInbox(inbox *storageModels.Inbox) errs.IMErrorCode {
	if inbox == nil || inbox.ChannelType != string(ChannelType_Widget) {
		return errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}
	return errs.IMErrorCode_SUCCESS
}

func validateTelegramInbox(inbox *storageModels.Inbox) errs.IMErrorCode {
	if inbox == nil || inbox.ChannelType != string(ChannelType_Telegram) {
		return errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}
	return errs.IMErrorCode_SUCCESS
}

func validateJuggleIMInbox(inbox *storageModels.Inbox) errs.IMErrorCode {
	if inbox == nil || inbox.ChannelType != string(ChannelType_JuggleIM) {
		return errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}
	return errs.IMErrorCode_SUCCESS
}

type customerTicketStartReq struct {
	AppKey           string
	InboxId          string
	Identifier       string
	Nickname         string
	Avatar           string
	ChannelType      ChannelType
	SourceId         string
	GenerateSourceId func() string
}

type customerTicketStartResp struct {
	Customer *storageModels.Customer
	Inbox    *storageModels.Inbox
	Rel      *storageModels.CustomerInboxRel
	Ticket   *storageModels.Ticket
	ImToken  string
}

func startCustomerTicket(req customerTicketStartReq) (errs.IMErrorCode, *customerTicketStartResp) {
	appkey := req.AppKey
	inboxId := strings.TrimSpace(req.InboxId)
	identifier := strings.TrimSpace(req.Identifier)
	nickname := strings.TrimSpace(req.Nickname)
	if appkey == "" || inboxId == "" || identifier == "" {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	customerStorage := newCustomerStorageForCustomer()
	inboxStorage := newInboxStorageForCustomer()
	relStorage := newCustomerInboxRelStorageForCustomer()
	ticketStorage := newTicketStorageForCustomer()

	customer, err := customerStorage.FindByIdentifier(appkey, identifier)
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
			Avator:     strings.TrimSpace(req.Avatar),
			Identifier: identifier,
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
	switch req.ChannelType {
	case ChannelType_Widget:
		if code := validateWidgetInbox(inbox); code != errs.IMErrorCode_SUCCESS {
			return code, nil
		}
	case ChannelType_Telegram:
		if code := validateTelegramInbox(inbox); code != errs.IMErrorCode_SUCCESS {
			return code, nil
		}
	case ChannelType_JuggleIM:
		if code := validateJuggleIMInbox(inbox); code != errs.IMErrorCode_SUCCESS {
			return code, nil
		}
	default:
		return errs.IMErrorCode_APP_CHANNEL_NOT_EXIST, nil
	}

	rel, err := relStorage.FindByCustomerInbox(appkey, customer.CustomerId, inbox.InboxId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if rel == nil {
		sourceId := strings.TrimSpace(req.SourceId)
		if sourceId == "" && req.GenerateSourceId != nil {
			sourceId = strings.TrimSpace(req.GenerateSourceId())
		}
		if sourceId == "" {
			sourceId = CustomerSourceIDPrefix + tools.GenerateUUIDShort22()
		}
		rel = &storageModels.CustomerInboxRel{
			CustomerId: customer.CustomerId,
			InboxId:    inbox.InboxId,
			SourceId:   sourceId,
			AppKey:     appkey,
		}
		if err := relStorage.Create(*rel); err != nil {
			return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
		}
	}

	sdk := getImSdkForCustomer(appkey)
	if sdk == nil {
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}

	ticket, err := ticketStorage.FindBySource(appkey, rel.SourceId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	code, imToken := registerCustomerIMUserForCustomer(sdk, rel.SourceId, customer.Nickname, customer.Avator)
	if code != errs.IMErrorCode_SUCCESS {
		return code, nil
	}

	if ticket == nil {
		ticketId := generateTicketId()
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
		groupCode, _, err := createGroupForCustomer(sdk, juggleimsdk.GroupMembersReq{
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
			TicketId:    ticketId,
			SourceId:    rel.SourceId,
			CustomerId:  customer.CustomerId,
			InboxId:     inbox.InboxId,
			ChannelType: inbox.ChannelType,
			Status:      storageModels.TicketStatusPending,
			AppKey:      appkey,
		}
		if err := ticketStorage.Create(*ticket); err != nil {
			return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
		}
		if code := syncTicketGlobalConversationTagsForCustomer(appkey, ticketId); code != errs.IMErrorCode_SUCCESS {
			return code, nil
		}
	}

	return errs.IMErrorCode_SUCCESS, &customerTicketStartResp{
		Customer: customer,
		Inbox:    inbox,
		Rel:      rel,
		Ticket:   ticket,
		ImToken:  imToken,
	}
}

func StartWebCustom(ctx context.Context, req *apiModels.StartCustomReq) (errs.IMErrorCode, *apiModels.StartCustomResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if req == nil {
		return errs.IMErrorCode_APP_ParamError, nil
	}
	resultCode, result := startCustomerTicket(customerTicketStartReq{
		AppKey:      appkey,
		InboxId:     req.InboxId,
		Identifier:  req.Identifier,
		Nickname:    req.Nickname,
		ChannelType: ChannelType_Widget,
		GenerateSourceId: func() string {
			return CustomerSourceIDPrefix + tools.GenerateUUIDShort22()
		},
	})
	if resultCode != errs.IMErrorCode_SUCCESS {
		return resultCode, nil
	}
	widgetConf := ParseWebWidgetChannelConf(result.Inbox.ChannelConf)

	return errs.IMErrorCode_SUCCESS, &apiModels.StartCustomResp{
		ConversationId:   result.Ticket.TicketId,
		ConversationType: ConversationType_Ticket,
		UserId:           result.Rel.SourceId,
		Nickname:         result.Customer.Nickname,
		ImToken:          result.ImToken,
		WelcomeMessage:   widgetConf.WelcomeMessage,
	}
}

func QryCustomerInfo(ctx context.Context, customerId string) (errs.IMErrorCode, *apiModels.UserInfo) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	requesterId := ctxs.GetRequesterIdFromCtx(ctx)
	customerId = strings.TrimSpace(customerId)
	if appkey == "" || requesterId == "" {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}
	if customerId == "" {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	customer, err := newCustomerStorageForCustomer().FindByCustomerId(appkey, customerId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if customer == nil {
		return errs.IMErrorCode_APP_USER_NOT_EXIST, nil
	}
	return errs.IMErrorCode_SUCCESS, &apiModels.UserInfo{
		Id:       customer.CustomerId,
		Nickname: customer.Nickname,
		Avatar:   customer.Avator,
	}
}

func QryCustomerTickets(ctx context.Context, req *apiModels.QryCustomerTicketsReq) (errs.IMErrorCode, *apiModels.QryCustomerTicketsResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	requesterId := ctxs.GetRequesterIdFromCtx(ctx)
	if appkey == "" || requesterId == "" {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}
	if req == nil || strings.TrimSpace(req.CustomerId) == "" || req.StartId < 0 || req.Limit <= 0 || req.Limit > 100 {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	customerId := strings.TrimSpace(req.CustomerId)
	customer, err := newCustomerStorageForCustomer().FindByCustomerId(appkey, customerId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if customer == nil {
		return errs.IMErrorCode_APP_USER_NOT_EXIST, nil
	}

	// Fetch one extra row to determine whether another page exists without a
	// separate count query.
	tickets, err := newTicketStorageForCustomer().QryByCustomer(appkey, customerId, req.StartId, req.Limit+1)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	hasMore := int64(len(tickets)) > req.Limit
	if hasMore {
		tickets = tickets[:req.Limit]
	}
	ticketResp, err := ticketsToAPI(appkey, tickets)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	resp := &apiModels.QryCustomerTicketsResp{Items: ticketResp.Items}
	if hasMore {
		for i := len(tickets) - 1; i >= 0; i-- {
			if tickets[i] != nil {
				resp.NextStartId = tickets[i].ID
				break
			}
		}
	}
	return errs.IMErrorCode_SUCCESS, resp
}
