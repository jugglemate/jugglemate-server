package services

import "testing"

func TestParseWebWidgetChannelConf(t *testing.T) {
	tests := []struct {
		name       string
		channelConf string
		want       string
	}{
		{
			name:        "empty conf",
			channelConf: "",
			want:        "",
		},
		{
			name:        "welcome message",
			channelConf: `{"welcome_message":"您好，有什么可以帮您？"}`,
			want:        "您好，有什么可以帮您？",
		},
		{
			name:        "invalid json",
			channelConf: `{`,
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseWebWidgetChannelConf(tt.channelConf).WelcomeMessage
			if got != tt.want {
				t.Fatalf("WelcomeMessage = %q, want %q", got, tt.want)
			}
		})
	}
}
