package services

import (
	"context"
	"strings"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

var (
	newUserStorageForTicket        = storages.NewUserStorage
	newCustomerStorageForTicket    = storages.NewCustomerStorage
	newTicketStorageForQuery       = storages.NewTicketStorage
	newInboxMemberStorageForTicket = storages.NewInboxMemberStorage
	getImSdkForTicketClaim         = imsdk.GetImSdk
	tagConversForClaim             = func(sdk *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		return sdk.TagConvers(req)
	}
	sendTicketAssignedNtfMsgForClaim = SendTicketAssignedNtfMsg
)

const (
	ticketConversationTagAssigned = "assigned"
	ticketConversationTagMyTicket = "my_ticket"
)

func QryTickets(ctx context.Context, req *apiModels.QryTicketsReq) (errs.IMErrorCode, *apiModels.QryTicketsResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	requesterId := ctxs.GetRequesterIdFromCtx(ctx)
	if appkey == "" || requesterId == "" || req == nil {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}

	status, ok := toTicketStatus(req.Status)
	if !ok {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	user, err := newUserStorageForTicket().FindByUserId(appkey, requesterId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if user == nil {
		return errs.IMErrorCode_APP_USER_NOT_EXIST, nil
	}

	var tickets []*storageModels.Ticket
	ticketStorage := newTicketStorageForQuery()
	switch user.Role {
	case storageModels.UserRoleAdmin:
		tickets, err = ticketStorage.QryAll(appkey, status, req.Limit, req.Offset)
	case storageModels.UserRoleCustomerService:
		tickets, err = ticketStorage.QryVisible(appkey, requesterId, status, req.Limit, req.Offset)
	default:
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	resp, err := ticketsToAPI(appkey, tickets)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	return errs.IMErrorCode_SUCCESS, resp
}

func ClaimTicket(ctx context.Context, ticketId string) (errs.IMErrorCode, *apiModels.ClaimTicketResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	requesterId := ctxs.GetRequesterIdFromCtx(ctx)
	if appkey == "" || requesterId == "" || ticketId == "" {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}

	user, err := newUserStorageForTicket().FindByUserId(appkey, requesterId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if user == nil {
		return errs.IMErrorCode_APP_USER_NOT_EXIST, nil
	}
	switch user.Role {
	case storageModels.UserRoleAdmin, storageModels.UserRoleCustomerService:
	default:
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}

	ticketStorage := newTicketStorageForQuery()
	ticket, err := ticketStorage.ClaimIfPending(appkey, ticketId, requesterId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if ticket == nil {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	sdk := getImSdkForTicketClaim(appkey)
	if sdk == nil {
		_ = ticketStorage.RevertClaimIfAssignee(appkey, ticketId, requesterId)
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}

	if code := syncTicketClaimConversationTags(appkey, ticket, requesterId, sdk); code != errs.IMErrorCode_SUCCESS {
		_ = ticketStorage.RevertClaimIfAssignee(appkey, ticketId, requesterId)
		return code, nil
	}

	resp, err := ticketsToAPI(appkey, []*storageModels.Ticket{ticket})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if len(resp.Items) == 0 {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	ticketInfo := resp.Items[0]
	sendTicketAssignedNtfMsgForClaim(ctx, &TicketAssignedNtfMsg{
		TicketId:   ticketId,
		Operator:   ticketInfo.Assignee,
		Assignee:   ticketInfo.Assignee,
		AssignType: AssignType_Claim,
	})
	return errs.IMErrorCode_SUCCESS, &apiModels.ClaimTicketResp{
		Ticket: ticketInfo,
	}
}

func toTicketStatus(status *int) (*storageModels.TicketStatus, bool) {
	if status == nil {
		return nil, true
	}
	switch *status {
	case int(storageModels.TicketStatusPending),
		int(storageModels.TicketStatusProcessing),
		int(storageModels.TicketStatusClosed):
		val := storageModels.TicketStatus(*status)
		return &val, true
	default:
		return nil, false
	}
}

func ticketsToAPI(appkey string, tickets []*storageModels.Ticket) (*apiModels.QryTicketsResp, error) {
	resp := &apiModels.QryTicketsResp{
		Items: make([]*apiModels.TicketInfo, 0, len(tickets)),
	}
	customerStorage := newCustomerStorageForTicket()
	userStorage := newUserStorageForTicket()
	customers := map[string]*apiModels.UserInfo{}
	assignees := map[string]*apiModels.UserInfo{}
	for _, ticket := range tickets {
		if ticket == nil {
			continue
		}
		customer, err := ticketCustomerInfo(customerStorage, customers, appkey, ticket.CustomerId)
		if err != nil {
			return nil, err
		}
		assignee, err := ticketAssigneeInfo(userStorage, assignees, appkey, ticket.AssigneeId)
		if err != nil {
			return nil, err
		}
		resp.Items = append(resp.Items, &apiModels.TicketInfo{
			TicketId:    ticket.TicketId,
			SourceId:    ticket.SourceId,
			CustomerId:  ticket.CustomerId,
			Customer:    customer,
			InboxId:     ticket.InboxId,
			AssigneeId:  ticket.AssigneeId,
			Assignee:    assignee,
			Status:      int(ticket.Status),
			CreatedTime: ticket.CreatedTime,
			UpdatedTime: ticket.UpdatedTime,
		})
	}
	return resp, nil
}

func syncTicketClaimConversationTags(appkey string, ticket *storageModels.Ticket, assigneeId string, sdk *juggleimsdk.JuggleIMSdk) errs.IMErrorCode {
	if ticket == nil {
		return errs.IMErrorCode_APP_ParamError
	}
	inboxMemberIds, err := ticketInboxMemberIds(appkey, ticket.InboxId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	conversation := ticketGroupConversation(ticket.TicketId)
	for _, memberId := range inboxMemberIds {
		code := tagTicketConversation(sdk, memberId, ticketConversationTagAssigned, conversation)
		if code != errs.IMErrorCode_SUCCESS {
			return code
		}
	}
	return tagTicketConversation(sdk, assigneeId, ticketConversationTagMyTicket, conversation)
}

func ticketInboxMemberIds(appkey, inboxId string) ([]string, error) {
	if strings.TrimSpace(inboxId) == "" {
		return nil, nil
	}
	memberStorage := newInboxMemberStorageForTicket()
	const pageSize int64 = 1000
	var startId int64
	memberIds := []string{}
	seen := map[string]struct{}{}
	for {
		members, err := memberStorage.QryByInbox(appkey, inboxId, startId, pageSize)
		if err != nil {
			return nil, err
		}
		if len(members) == 0 {
			return memberIds, nil
		}
		for _, member := range members {
			if member == nil {
				continue
			}
			if member.ID > 0 && (startId == 0 || member.ID < startId) {
				startId = member.ID
			}
			memberId := strings.TrimSpace(member.MemberId)
			if memberId == "" {
				continue
			}
			if _, ok := seen[memberId]; ok {
				continue
			}
			seen[memberId] = struct{}{}
			memberIds = append(memberIds, memberId)
		}
		if int64(len(members)) < pageSize || startId == 0 {
			return memberIds, nil
		}
	}
}

func ticketGroupConversation(ticketId string) *juggleimsdk.Conversation {
	return &juggleimsdk.Conversation{
		TargetId:    ticketId,
		ChannelType: int(juggleimsdk.ChannelType_Group),
	}
}

func tagTicketConversation(sdk *juggleimsdk.JuggleIMSdk, userId, tag string, conversation *juggleimsdk.Conversation) errs.IMErrorCode {
	userId = strings.TrimSpace(userId)
	if userId == "" {
		return errs.IMErrorCode_APP_ParamError
	}
	code, _, err := tagConversForClaim(sdk, juggleimsdk.TagConversReq{
		UserId:  userId,
		Tag:     tag,
		Convers: []*juggleimsdk.Conversation{conversation},
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		return errs.IMErrorCode(code)
	}
	return errs.IMErrorCode_SUCCESS
}

func ticketCustomerInfo(storage storageModels.ICustomerStorage, cache map[string]*apiModels.UserInfo, appkey, customerId string) (*apiModels.UserInfo, error) {
	if customerId == "" {
		return nil, nil
	}
	if item, ok := cache[customerId]; ok {
		return item, nil
	}
	customer, err := storage.FindByCustomerId(appkey, customerId)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		cache[customerId] = nil
		return nil, nil
	}
	item := &apiModels.UserInfo{
		Id:       customer.CustomerId,
		Nickname: customer.Nickname,
		Avatar:   customer.Avator,
	}
	cache[customerId] = item
	return item, nil
}

func ticketAssigneeInfo(storage storageModels.IUserStorage, cache map[string]*apiModels.UserInfo, appkey, assigneeId string) (*apiModels.UserInfo, error) {
	if assigneeId == "" {
		return nil, nil
	}
	if item, ok := cache[assigneeId]; ok {
		return item, nil
	}
	user, err := storage.FindByUserId(appkey, assigneeId)
	if err != nil {
		return nil, err
	}
	if user == nil {
		cache[assigneeId] = nil
		return nil, nil
	}
	item := &apiModels.UserInfo{
		Id:       user.UserId,
		Nickname: user.Nickname,
		Avatar:   user.Avator,
	}
	cache[assigneeId] = item
	return item, nil
}
