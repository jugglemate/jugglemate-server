package routers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestAgentAdminDirector_RewritesAndInjects 验证目标地址重写、路径拼接与查询保留。
func TestAgentAdminDirector_RewritesAndInjects(t *testing.T) {
	target, _ := url.Parse("http://127.0.0.1:8000")
	req := httptest.NewRequest(http.MethodGet, "http://console/jmate/agentapi/agents?limit=20", nil)

	agentAdminDirector(target, "/agents", "u_abc", "s3cr3t")(req)

	if req.URL.Host != "127.0.0.1:8000" || req.URL.Scheme != "http" {
		t.Fatalf("target not rewritten: %s://%s", req.URL.Scheme, req.URL.Host)
	}
	if req.URL.Path != "/api/v1/agents" {
		t.Fatalf("path = %q, want /api/v1/agents", req.URL.Path)
	}
	if req.URL.RawQuery != "limit=20" {
		t.Fatalf("query = %q, want limit=20", req.URL.RawQuery)
	}
	if got := req.Header.Get(headerInternalAuth); got != "s3cr3t" {
		t.Fatalf("internal auth = %q, want s3cr3t", got)
	}
	if got := req.Header.Get(headerUserId); got != "u_abc" {
		t.Fatalf("X-User-ID = %q, want u_abc", got)
	}
}

// TestAgentAdminDirector_StripsClientSpoofedHeaders 验证客户端伪造的身份/密钥头被覆盖。
func TestAgentAdminDirector_StripsClientSpoofedHeaders(t *testing.T) {
	target, _ := url.Parse("http://127.0.0.1:8000")
	req := httptest.NewRequest(http.MethodPost, "http://console/jmate/agentapi/agents/create", nil)
	// 客户端尝试伪造身份与内部密钥。
	req.Header.Set("Authorization", "Bearer forged")
	req.Header.Set(headerUserId, "u_victim")
	req.Header.Set(headerInternalAuth, "forged-secret")
	req.Header.Set("appkey", "forged")

	agentAdminDirector(target, "/agents/create", "u_real", "real-secret")(req)

	if got := req.Header.Get(headerUserId); got != "u_real" {
		t.Fatalf("X-User-ID = %q, want u_real (spoof must be overwritten)", got)
	}
	if got := req.Header.Get(headerInternalAuth); got != "real-secret" {
		t.Fatalf("internal auth = %q, want real-secret", got)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty (stripped)", got)
	}
	if got := req.Header.Get("appkey"); got != "" {
		t.Fatalf("appkey = %q, want empty (stripped)", got)
	}
}
