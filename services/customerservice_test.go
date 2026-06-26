package services

import (
	"testing"

	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestValidateWidgetInbox(t *testing.T) {
	tests := []struct {
		name  string
		inbox *storageModels.Inbox
		want  errs.IMErrorCode
	}{
		{
			name:  "missing inbox",
			inbox: nil,
			want:  errs.IMErrorCode_APP_CHANNEL_NOT_EXIST,
		},
		{
			name: "telegram inbox rejected",
			inbox: &storageModels.Inbox{
				InboxId:     "inbox_1",
				ChannelType: string(ChannelType_Telegram),
			},
			want: errs.IMErrorCode_APP_CHANNEL_NOT_EXIST,
		},
		{
			name: "widget inbox accepted",
			inbox: &storageModels.Inbox{
				InboxId:     "inbox_widget_1",
				ChannelType: string(ChannelType_Widget),
			},
			want: errs.IMErrorCode_SUCCESS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateWidgetInbox(tt.inbox); got != tt.want {
				t.Fatalf("validateWidgetInbox() = %d, want %d", got, tt.want)
			}
		})
	}
}
