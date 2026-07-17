package main

import (
	"strings"
	"testing"
)

// TestStripLeadingNUL 校验历史 Inbox 只删除第一个首字节 NUL。
func TestStripLeadingNUL(t *testing.T) {
	if got := stripLeadingNUL("\x00inbox-1"); got != "inbox-1" {
		t.Fatalf("stripLeadingNUL() = %q", got)
	}
	if got := stripLeadingNUL("inbox-1\x00tail"); got != "inbox-1\x00tail" {
		t.Fatalf("中间 NUL 不应被修改: %q", got)
	}
}

// TestResolveRecordApp 校验显式 AppKey、唯一候选和冲突候选三种分支。
func TestResolveRecordApp(t *testing.T) {
	if got, ok := resolveRecordApp("app-explicit", map[string]struct{}{"app-other": {}}); !ok || got != "app-explicit" {
		t.Fatalf("显式 AppKey 应优先: got=%q ok=%v", got, ok)
	}
	if got, ok := resolveRecordApp("", map[string]struct{}{"app-1": {}}); !ok || got != "app-1" {
		t.Fatalf("唯一候选应成功: got=%q ok=%v", got, ok)
	}
	if _, ok := resolveRecordApp("", map[string]struct{}{"app-1": {}, "app-2": {}}); ok {
		t.Fatal("跨 App 候选冲突必须失败")
	}
}

// TestTargetInsertUsesParameters 校验生成的 PostgreSQL 写入始终使用参数占位符。
func TestTargetInsertUsesParameters(t *testing.T) {
	definition := tableDefinitions(false)[0]
	statement := targetInsert(definition)
	if !strings.Contains(statement, "VALUES ($1,$2,$3,$4,$5,$6,$7)") {
		t.Fatalf("参数化 INSERT 不符合预期: %s", statement)
	}
	if strings.Contains(statement, "app-secret") {
		t.Fatal("INSERT 模板不得拼接业务值")
	}
}

// TestNormalizePostgresDSN 校验 URL、键值和显式 SSL 三种 DSN 处理。
func TestNormalizePostgresDSN(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "keyword", input: "host=127.0.0.1 dbname=jmate", want: "host=127.0.0.1 dbname=jmate sslmode=disable"},
		{name: "url", input: "postgresql://agent@127.0.0.1/jmate", want: "postgresql://agent@127.0.0.1/jmate?sslmode=disable"},
		{name: "explicit", input: "host=db sslmode=require", want: "host=db sslmode=require"},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			got, err := normalizePostgresDSN(item.input)
			if err != nil || got != item.want {
				t.Fatalf("normalizePostgresDSN() = %q, %v; want %q", got, err, item.want)
			}
		})
	}
}
