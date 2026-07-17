package dbs

import "testing"

// TestEmailPtrForDB 空 email 必须落成空字符串。
//
// TIPS: 这里断言的不是“函数返回什么”，而是“能不能写进 users 表”。users.email 是
// NOT NULL DEFAULT ''，返回 nil 会写 NULL 并违反约束，让注册接口静默报 17006。
// 本用例此前断言空 email 返回 nil（MySQL 时代 email 可空的遗留），等于把这个 bug 固化住了。
func TestEmailPtrForDB(t *testing.T) {
	for _, input := range []string{"", "   "} {
		ptr := emailPtrForDB(input)
		if ptr == nil {
			t.Fatalf("空 email(%q) 不能写 NULL —— users.email 是 NOT NULL", input)
		}
		if *ptr != "" {
			t.Fatalf("空 email(%q) 应落成空字符串，实际 %q", input, *ptr)
		}
	}
	ptr := emailPtrForDB("alice@example.com")
	if ptr == nil || *ptr != "alice@example.com" {
		t.Fatalf("email ptr = %v", ptr)
	}
	if ptr := emailPtrForDB("  alice@example.com  "); ptr == nil || *ptr != "alice@example.com" {
		t.Fatalf("email 应去除首尾空格: %v", ptr)
	}
}

func TestEmailStringFromDB(t *testing.T) {
	if got := emailStringFromDB(nil); got != "" {
		t.Fatalf("nil email = %q, want empty string", got)
	}
	email := "alice@example.com"
	if got := emailStringFromDB(&email); got != email {
		t.Fatalf("email = %q", got)
	}
}
