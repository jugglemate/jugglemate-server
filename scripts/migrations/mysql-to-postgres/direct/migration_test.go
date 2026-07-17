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

// TestSystemOwnerCandidates 校验 system Owner 只在单 App 时可被唯一解析。
func TestSystemOwnerCandidates(t *testing.T) {
	owners := map[string]map[string]struct{}{}
	addCandidate(owners, "system", "app-1")
	if got, ok := resolveRecordApp("", owners["system"]); !ok || got != "app-1" {
		t.Fatalf("单 App system Owner 应成功: got=%q ok=%v", got, ok)
	}
	addCandidate(owners, "system", "app-2")
	if _, ok := resolveRecordApp("", owners["system"]); ok {
		t.Fatal("多 App system Owner 必须拒绝猜测")
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

// TestPostgresCommandEnvironment 校验 URL/键值 DSN 会安全转换为 pg_dump 可识别的独立环境变量。
func TestPostgresCommandEnvironment(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want map[string]string
	}{
		{
			name: "url",
			dsn:  "postgresql://agent:p%40ss@db.example:5544/jmate?sslmode=require",
			want: map[string]string{"PGHOST": "db.example", "PGPORT": "5544", "PGUSER": "agent", "PGPASSWORD": "p@ss", "PGDATABASE": "jmate", "PGSSLMODE": "require"},
		},
		{
			name: "keyword quoted",
			dsn:  `host=127.0.0.1 port=5432 user='agent user' password='p\ ass' dbname=jmate sslmode=disable`,
			want: map[string]string{"PGHOST": "127.0.0.1", "PGPORT": "5432", "PGUSER": "agent user", "PGPASSWORD": "p ass", "PGDATABASE": "jmate", "PGSSLMODE": "disable"},
		},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			environment, err := postgresCommandEnvironment(item.dsn, []string{"PATH=/usr/bin", "PGUSER=stale", "PGDATABASE=stale"})
			if err != nil {
				t.Fatalf("postgresCommandEnvironment() error = %v", err)
			}
			actual := environmentMap(environment)
			for name, want := range item.want {
				if got := actual[name]; got != want {
					t.Fatalf("%s = %q; want %q", name, got, want)
				}
			}
			if actual["PATH"] != "/usr/bin" {
				t.Fatalf("无关环境变量未保留: %q", actual["PATH"])
			}
		})
	}
}

// TestPostgresCommandEnvironmentRejectsInvalidDSN 校验异常键值 DSN 不会降级为本机默认连接。
func TestPostgresCommandEnvironmentRejectsInvalidDSN(t *testing.T) {
	for _, value := range []string{"host", "host='not-closed", `password=broken\`} {
		if _, err := postgresCommandEnvironment(value, nil); err == nil {
			t.Fatalf("非法 DSN 应返回错误: %q", value)
		}
	}
}

func environmentMap(environment []string) map[string]string {
	result := make(map[string]string, len(environment))
	for _, item := range environment {
		name, value, found := strings.Cut(item, "=")
		if found {
			result[name] = value
		}
	}
	return result
}
