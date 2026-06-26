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
	CustomChannel_Web       string = "web"
	CustomInbox_Web         string = "web"
	CustomerSourceIDPrefix  string = "customer_"
	ConversationType_Ticket int    = 2
)

func StartWebCustom(ctx context.Context, req *apiModels.StartCustomReq) (errs.IMErrorCode, *apiModels.StartCustomResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" || req == nil || req.Identifier == "" {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	channelId := CustomChannel_Web
	inboxId := CustomInbox_Web
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
	if inbox == nil {
		inbox = &storageModels.Inbox{
			InboxId:     inboxId,
			ChannelType: channelId,
			Name:        channelId,
			AppKey:      appkey,
		}
		if err := inboxStorage.Create(*inbox); err != nil {
			return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
		}
	}

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
		code, _, err := sdk.CreateGroup(juggleimsdk.GroupMembersReq{
			GroupId:   ticketId,
			GroupName: groupName,
			MemberIds: []string{
				rel.SourceId,
			},
		})
		if err != nil {
			return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
		}
		if code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
			return errs.IMErrorCode(code), nil
		}
		ticket = &storageModels.Ticket{
			TicketId:   ticketId,
			SourceId:   rel.SourceId,
			CustomerId: customer.CustomerId,
			ChannelId:  channelId,
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
	}
}
