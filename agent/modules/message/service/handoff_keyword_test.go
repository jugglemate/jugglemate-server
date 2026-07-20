package service

import "testing"

// TestMatchHandoffKeyword 覆盖转人工关键词的命中与误触发边界。
//
// TIPS: 误触发的代价远大于漏触发 —— 一旦误判，Bot 会被移出 Ticket 群且不会自动回来，
// 必须人工恢复。因此这里重点锁死"包含关键词但整体不是转人工诉求"的用例。
func TestMatchHandoffKeyword(t *testing.T) {
	shouldMatch := []string{
		"转人工",
		"人工",
		"人工客服",
		"转客服",
		"转人工客服",
		"要人工",
		"找人工",
		"人工服务",
		"转接人工",
		"  转人工  ",
		"转人工。",
		"转人工！",
		"转人工?",
		"转 人 工",
	}
	for _, text := range shouldMatch {
		if !MatchHandoffKeyword(text) {
			t.Fatalf("%q 应命中转人工关键词", text)
		}
	}

	shouldNotMatch := []string{
		"",
		"   ",
		"你好",
		"这个是人工审核吗",
		"人工费怎么算",
		"我想问下人工智能相关的问题",
		"你们的人工客服电话是多少",
		"不用转人工了谢谢",
		"转人工之前我想先问一下",
		"人工智能",
		"转账",
	}
	for _, text := range shouldNotMatch {
		if MatchHandoffKeyword(text) {
			t.Fatalf("%q 不应命中转人工关键词，会造成误触发", text)
		}
	}
}
