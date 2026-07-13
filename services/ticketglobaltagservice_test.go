package services

import (
	"reflect"
	"testing"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestTicketGlobalConversationTags(t *testing.T) {
	tests := []struct {
		name        string
		assigneeId  string
		status      storageModels.TicketStatus
		channelType string
		want        []string
	}{
		{"pending widget unassigned", "", storageModels.TicketStatusPending, "widget", []string{"unassigned", "ticket_status_pending", "channel_widget"}},
		{"processing telegram assigned", "u_1", storageModels.TicketStatusProcessing, "telegram", []string{"assigned", "ticket_status_processing", "channel_telegram"}},
		{"closed juggleim assigned", "u_1", storageModels.TicketStatusClosed, "juggleim", []string{"assigned", "ticket_status_closed", "channel_juggleim"}},
		{"reopen widget assigned", "u_1", storageModels.TicketStatusReOpen, "widget", []string{"assigned", "ticket_status_reopen", "channel_widget"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ticketGlobalConversationTags(&storageModels.Ticket{
				AssigneeId:  tt.assigneeId,
				Status:      tt.status,
				ChannelType: tt.channelType,
			})
			if !ok || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("tags = %+v ok=%v, want %+v", got, ok, tt.want)
			}
		})
	}
}

func TestSyncTicketGlobalConversationTagsUsesLatestPersistedTicket(t *testing.T) {
	ticketStorage := &mockTicketStorage{findTicket: &storageModels.Ticket{
		TicketId: "ticket_1", AssigneeId: "u_1", Status: storageModels.TicketStatusProcessing,
		ChannelType: "telegram", AppKey: "app_1",
	}}
	origStorage := newTicketStorageForGlobalTags
	origGetSDK := getImSdkForTicketGlobalTags
	origSetTags := setGlobalConverTagsForTicket
	defer func() {
		newTicketStorageForGlobalTags = origStorage
		getImSdkForTicketGlobalTags = origGetSDK
		setGlobalConverTagsForTicket = origSetTags
	}()
	newTicketStorageForGlobalTags = func() storageModels.ITicketStorage { return ticketStorage }
	getImSdkForTicketGlobalTags = func(string) *juggleimsdk.JuggleIMSdk { return &juggleimsdk.JuggleIMSdk{} }

	var got juggleimsdk.GlobalConverTagsReq
	setGlobalConverTagsForTicket = func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.GlobalConverTagsReq) (juggleimsdk.ApiCode, string, error) {
		got = req
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	if code := SyncTicketGlobalConversationTags("app_1", "ticket_1"); code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if got.ConverId != "ticket_1" || got.ChannelType != int(juggleimsdk.ChannelType_Group) || got.GlobalConverTags == nil {
		t.Fatalf("request = %+v", got)
	}
	want := []string{"assigned", "ticket_status_processing", "channel_telegram"}
	if !reflect.DeepEqual(*got.GlobalConverTags, want) {
		t.Fatalf("tags = %+v, want %+v", *got.GlobalConverTags, want)
	}
}
