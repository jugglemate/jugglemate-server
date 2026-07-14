package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

// TestSetGlobalConverTagsForTicketUsesServerAPIContract 验证兼容层调用真实 Server API 契约。
func TestSetGlobalConverTagsForTicketUsesServerAPIContract(t *testing.T) {
	tags := []string{"assigned", "ticket_status_processing", "channel_widget"}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/apigateway/convers/globaltags/set" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body globalConverTagsRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.ConverID != "ticket_1" || body.ChannelType != int(juggleimsdk.ChannelType_Group) || body.GlobalConverTags == nil || !reflect.DeepEqual(*body.GlobalConverTags, tags) {
			t.Fatalf("request body = %+v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer server.Close()

	sdk := juggleimsdk.NewJuggleIMSdk("app_1", "secret", server.URL)
	code, _, err := setGlobalConverTagsForTicket(sdk, globalConverTagsRequest{
		ConverID:         "ticket_1",
		ChannelType:      int(juggleimsdk.ChannelType_Group),
		GlobalConverTags: &tags,
	})
	if err != nil || code != juggleimsdk.ApiCode_Success {
		t.Fatalf("code = %d, err = %v", code, err)
	}
}

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

	var got globalConverTagsRequest
	setGlobalConverTagsForTicket = func(_ *juggleimsdk.JuggleIMSdk, req globalConverTagsRequest) (juggleimsdk.ApiCode, string, error) {
		got = req
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	if code := SyncTicketGlobalConversationTags("app_1", "ticket_1"); code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if got.ConverID != "ticket_1" || got.ChannelType != int(juggleimsdk.ChannelType_Group) || got.GlobalConverTags == nil {
		t.Fatalf("request = %+v", got)
	}
	want := []string{"assigned", "ticket_status_processing", "channel_telegram"}
	if !reflect.DeepEqual(*got.GlobalConverTags, want) {
		t.Fatalf("tags = %+v, want %+v", *got.GlobalConverTags, want)
	}
}
