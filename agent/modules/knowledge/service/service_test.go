package service

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitTextPreservesOverlapAndTail(t *testing.T) {
	chunks := splitText("1234567890", 4, 1)
	want := []string{"1234", "4567", "7890"}
	if !reflect.DeepEqual(chunks, want) {
		t.Fatalf("分块结果错误: got=%v want=%v", chunks, want)
	}
}

func TestNormalizeVectorPadsAndTruncates(t *testing.T) {
	padded := normalizeVector([]float32{1, 2}, 4).Slice()
	if !reflect.DeepEqual(padded, []float32{1, 2, 0, 0}) {
		t.Fatalf("向量补齐错误: %v", padded)
	}
	truncated := normalizeVector([]float32{1, 2, 3}, 2).Slice()
	if !reflect.DeepEqual(truncated, []float32{1, 2}) {
		t.Fatalf("向量截断错误: %v", truncated)
	}
}

// TestNormalizeVectorMatchesStorageDimension 回归 1024 维模型向量检索 1536 维存储向量的故障。
func TestNormalizeVectorMatchesStorageDimension(t *testing.T) {
	input := make([]float32, 1024)
	input[0], input[1023] = 1, 2
	result := normalizeVector(input, StorageVectorDimension).Slice()
	if len(result) != 1536 || result[0] != 1 || result[1023] != 2 || result[1024] != 0 || result[1535] != 0 {
		t.Fatalf("查询向量未正确归一化到存储维度: len=%d", len(result))
	}
}

func TestValidateRemoteURLRejectsInternalNetwork(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1/a.pdf", "http://localhost/a.pdf", "file:///etc/passwd"} {
		if err := validateRemoteURL(context.Background(), raw); err == nil {
			t.Fatalf("内网或非 HTTP URL 应被拒绝: %s", raw)
		}
	}
}

func TestExtractHTMLText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.html")
	if err := os.WriteFile(path, []byte(`<html><body><h1>标题</h1><p>正文</p></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := extractText(path)
	if err != nil {
		t.Fatal(err)
	}
	if text != "标题\n正文\n" {
		t.Fatalf("HTML 提取结果错误: %q", text)
	}
}
