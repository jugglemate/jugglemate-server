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
	newUserStorageForTicket     = storages.NewUserStorage
	newCustomerStorageForTicket = storages.NewCustomerStorage
	newTicketStorageForQuery    = storages.NewTicketStorage
	getImSdkForTicketClaim      = imsdk.GetImSdk
	getImSdkForTicketTransfer   = imsdk.GetImSdk
	tagConversForClaim          = func(sdk *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		return sdk.TagConvers(req)
	}
	tagConversForTransfer = func(sdk *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		return sdk.TagConvers(req)
	}
	unTagConversForTransfer = func(sdk *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		return sdk.UnTagConvers(req)
	}
	syncTicketGlobalConversationTagsForClaim    = SyncTicketGlobalConversationTags
	syncTicketGlobalConversationTagsForTransfer = SyncTicketGlobalConversationTags
	sendTicketAssignedNtfMsgForClaim            = SendTicketAssignedNtfMsg
	sendTicketAssignedNtfMsgForTransfer         = SendTicketAssignedNtfMsg
)

const ticketConversationTagMyTicket = "my_ticket"

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
	if code := syncTicketGlobalConversationTagsForClaim(appkey, ticketId); code != errs.IMErrorCode_SUCCESS {
		_ = ticketStorage.RevertClaimIfAssignee(appkey, ticketId, requesterId)
		_ = syncTicketGlobalConversationTagsForClaim(appkey, ticketId)
		return code, nil
	}

	sdk := getImSdkForTicketClaim(appkey)
	if sdk == nil {
		_ = ticketStorage.RevertClaimIfAssignee(appkey, ticketId, requesterId)
		_ = syncTicketGlobalConversationTagsForClaim(appkey, ticketId)
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}

	if code := syncTicketClaimConversationTags(appkey, ticket, requesterId, sdk); code != errs.IMErrorCode_SUCCESS {
		_ = ticketStorage.RevertClaimIfAssignee(appkey, ticketId, requesterId)
		_ = syncTicketGlobalConversationTagsForClaim(appkey, ticketId)
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

func TransferTicket(ctx context.Context, ticketId string, req *apiModels.TransferTicketReq) (errs.IMErrorCode, *apiModels.TransferTicketResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	requesterId := ctxs.GetRequesterIdFromCtx(ctx)
	if appkey == "" || requesterId == "" || strings.TrimSpace(ticketId) == "" || req == nil {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}
	assigneeId := strings.TrimSpace(req.AssigneeId)
	if assigneeId == "" || assigneeId == requesterId {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	userStorage := newUserStorageForTicket()
	operator, err := userStorage.FindByUserId(appkey, requesterId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if operator == nil {
		return errs.IMErrorCode_APP_USER_NOT_EXIST, nil
	}
	if operator.Role != storageModels.UserRoleAdmin && operator.Role != storageModels.UserRoleCustomerService {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}

	assignee, err := userStorage.FindByUserId(appkey, assigneeId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if assignee == nil {
		return errs.IMErrorCode_APP_USER_NOT_EXIST, nil
	}
	if assignee.Role != storageModels.UserRoleAdmin && assignee.Role != storageModels.UserRoleCustomerService {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	ticketStorage := newTicketStorageForQuery()
	ticket, err := ticketStorage.FindByTicketId(appkey, ticketId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if ticket == nil || ticket.Status != storageModels.TicketStatusProcessing || ticket.AssigneeId == "" {
		return errs.IMErrorCode_APP_ParamError, nil
	}
	if operator.Role != storageModels.UserRoleAdmin && ticket.AssigneeId != requesterId {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}
	if ticket.AssigneeId == assigneeId {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	oldAssigneeId := ticket.AssigneeId
	ticket, err = ticketStorage.TransferIfAssignee(appkey, ticketId, oldAssigneeId, assigneeId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	if ticket == nil {
		return errs.IMErrorCode_APP_ParamError, nil
	}
	if code := syncTicketGlobalConversationTagsForTransfer(appkey, ticketId); code != errs.IMErrorCode_SUCCESS {
		_, _ = ticketStorage.TransferIfAssignee(appkey, ticketId, assigneeId, oldAssigneeId)
		_ = syncTicketGlobalConversationTagsForTransfer(appkey, ticketId)
		return code, nil
	}

	sdk := getImSdkForTicketTransfer(appkey)
	if sdk == nil {
		_, _ = ticketStorage.TransferIfAssignee(appkey, ticketId, assigneeId, oldAssigneeId)
		_ = syncTicketGlobalConversationTagsForTransfer(appkey, ticketId)
		return errs.IMErrorCode_APP_NOT_EXISTED, nil
	}
	if code := syncTicketTransferConversationTags(ticketId, oldAssigneeId, assigneeId, sdk); code != errs.IMErrorCode_SUCCESS {
		_, _ = ticketStorage.TransferIfAssignee(appkey, ticketId, assigneeId, oldAssigneeId)
		_ = syncTicketGlobalConversationTagsForTransfer(appkey, ticketId)
		return code, nil
	}

	resp, err := ticketsToAPI(appkey, []*storageModels.Ticket{ticket})
	if err != nil || len(resp.Items) == 0 {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	ticketInfo := resp.Items[0]
	sendTicketAssignedNtfMsgForTransfer(ctx, &TicketAssignedNtfMsg{
		TicketId: ticketId,
		Operator: &apiModels.UserInfo{
			Id:       operator.UserId,
			Nickname: operator.Nickname,
			Avatar:   operator.Avator,
		},
		Assignee:   ticketInfo.Assignee,
		AssignType: AssignType_AssignUser,
	})
	return errs.IMErrorCode_SUCCESS, &apiModels.TransferTicketResp{Ticket: ticketInfo}
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
			ChannelType: ticket.ChannelType,
			AssigneeId:  ticket.AssigneeId,
			Assignee:    assignee,
			Status:      int(ticket.Status),
			CreatedTime: ticket.CreatedTime,
			UpdatedTime: ticket.UpdatedTime,
		})
	}
	return resp, nil
}

func syncTicketClaimConversationTags(_ string, ticket *storageModels.Ticket, assigneeId string, sdk *juggleimsdk.JuggleIMSdk) errs.IMErrorCode {
	if ticket == nil {
		return errs.IMErrorCode_APP_ParamError
	}
	return tagTicketConversation(sdk, assigneeId, ticketConversationTagMyTicket, ticketGroupConversation(ticket.TicketId))
}

func syncTicketTransferConversationTags(ticketId, oldAssigneeId, newAssigneeId string, sdk *juggleimsdk.JuggleIMSdk) errs.IMErrorCode {
	conversation := ticketGroupConversation(ticketId)
	if code := updateTicketConversationTag(unTagConversForTransfer, sdk, oldAssigneeId, ticketConversationTagMyTicket, conversation); code != errs.IMErrorCode_SUCCESS {
		return code
	}
	if code := updateTicketConversationTag(tagConversForTransfer, sdk, newAssigneeId, ticketConversationTagMyTicket, conversation); code != errs.IMErrorCode_SUCCESS {
		// Best-effort compensation: restore the previous assignee's tag before
		// the caller rolls the persisted assignee back.
		_ = updateTicketConversationTag(tagConversForTransfer, sdk, oldAssigneeId, ticketConversationTagMyTicket, conversation)
		return code
	}
	return errs.IMErrorCode_SUCCESS
}

func ticketGroupConversation(ticketId string) *juggleimsdk.Conversation {
	return &juggleimsdk.Conversation{
		TargetId:    ticketId,
		ChannelType: int(juggleimsdk.ChannelType_Group),
	}
}

func tagTicketConversation(sdk *juggleimsdk.JuggleIMSdk, userId, tag string, conversation *juggleimsdk.Conversation) errs.IMErrorCode {
	return updateTicketConversationTag(tagConversForClaim, sdk, userId, tag, conversation)
}

type ticketConversationTagOperation func(*juggleimsdk.JuggleIMSdk, juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error)

func updateTicketConversationTag(operation ticketConversationTagOperation, sdk *juggleimsdk.JuggleIMSdk, userId, tag string, conversation *juggleimsdk.Conversation) errs.IMErrorCode {
	userId = strings.TrimSpace(userId)
	if operation == nil || sdk == nil || userId == "" || conversation == nil {
		return errs.IMErrorCode_APP_ParamError
	}
	code, _, err := operation(sdk, juggleimsdk.TagConversReq{
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
