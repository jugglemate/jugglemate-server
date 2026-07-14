package handlers

import (
	"errors"
	"strings"
	"testing"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
)

// TestSendSessionFeedbackReply 校验运营反馈只在真实 IM 成功状态下视为发送成功。
func TestSendSessionFeedbackReply(t *testing.T) {
	originalGet := getFeedbackIMSDK
	originalSend := sendFeedbackIM
	t.Cleanup(func() {
		getFeedbackIMSDK = originalGet
		sendFeedbackIM = originalSend
	})

	if err := sendSessionFeedbackReply("", "bot", "customer", "回复"); err == nil {
		t.Fatal("缺少 appkey 时应拒绝发送")
	}

	sdk := &juggleimsdk.JuggleIMSdk{}
	getFeedbackIMSDK = func(string) *juggleimsdk.JuggleIMSdk { return sdk }
	var captured juggleimsdk.Message
	sendFeedbackIM = func(actual *juggleimsdk.JuggleIMSdk, message juggleimsdk.Message) (juggleimsdk.ApiCode, string, error) {
		if actual != sdk {
			t.Fatal("应使用当前应用对应的 IM SDK")
		}
		captured = message
		return juggleimsdk.ApiCode_Success, "message-1", nil
	}
	if err := sendSessionFeedbackReply("app", "bot", "customer", "已编辑回复"); err != nil {
		t.Fatalf("发送成功状态不应报错: %v", err)
	}
	if captured.SenderId != "bot" || len(captured.TargetIds) != 1 || captured.TargetIds[0] != "customer" || !strings.Contains(captured.MsgContent, "已编辑回复") {
		t.Fatalf("IM 消息字段异常: %+v", captured)
	}

	sendFeedbackIM = func(*juggleimsdk.JuggleIMSdk, juggleimsdk.Message) (juggleimsdk.ApiCode, string, error) {
		return 500, "", nil
	}
	if err := sendSessionFeedbackReply("app", "bot", "customer", "回复"); err == nil {
		t.Fatal("IM 业务失败状态应返回错误")
	}

	sendFeedbackIM = func(*juggleimsdk.JuggleIMSdk, juggleimsdk.Message) (juggleimsdk.ApiCode, string, error) {
		return 0, "", errors.New("network unavailable")
	}
	if err := sendSessionFeedbackReply("app", "bot", "customer", "回复"); err == nil {
		t.Fatal("IM 网络错误应向上返回")
	}

	getFeedbackIMSDK = func(string) *juggleimsdk.JuggleIMSdk { return nil }
	if err := sendSessionFeedbackReply("app", "bot", "customer", "回复"); err == nil {
		t.Fatal("IM SDK 不可用时应返回错误")
	}
}
