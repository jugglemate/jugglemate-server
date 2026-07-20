package logs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// slogHandler 把标准库 slog 的输出接到本项目的 logrus 双通道上。
//
// TIPS: agent 模块（imbot 连接、入站推理）全部用 slog 打点，而 slog 默认 Handler 只写
// stderr，永远不会进 logs/<name>.log。运维只翻日志文件时会误以为 agent 链路"没有日志"，
// 实际是日志走了另一条通道。这里做统一收口：WARN 及以上进 _err 文件，其余进主日志文件。
type slogHandler struct {
	mu     *sync.Mutex
	prefix string
	groups []string
}

// newSlogHandler 创建接入 logrus 的 slog Handler。
func newSlogHandler() *slogHandler {
	return &slogHandler{mu: &sync.Mutex{}}
}

// Enabled 放行 Debug 及以上级别，实际过滤交给 logrus 的 Level 控制。
func (handler *slogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

// Handle 将一条 slog 记录格式化为 "消息\tkey:value" 并按级别分流。
func (handler *slogHandler) Handle(_ context.Context, record slog.Record) error {
	var builder strings.Builder
	builder.WriteString(record.Message)
	builder.WriteString(handler.prefix)
	record.Attrs(func(attr slog.Attr) bool {
		builder.WriteString(formatSlogAttr(handler.groups, attr))
		return true
	})
	line := builder.String()

	handler.mu.Lock()
	defer handler.mu.Unlock()
	switch {
	case record.Level >= slog.LevelError:
		Errorf("%s", line)
	case record.Level >= slog.LevelWarn:
		Warnf("%s", line)
	case record.Level >= slog.LevelInfo:
		Infof("%s", line)
	default:
		Debugf("%s", line)
	}
	return nil
}

// WithAttrs 返回携带固定字段的派生 Handler。
func (handler *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return handler
	}
	var builder strings.Builder
	builder.WriteString(handler.prefix)
	for _, attr := range attrs {
		builder.WriteString(formatSlogAttr(handler.groups, attr))
	}
	return &slogHandler{mu: handler.mu, prefix: builder.String(), groups: handler.groups}
}

// WithGroup 返回带分组前缀的派生 Handler。
func (handler *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return handler
	}
	groups := make([]string, 0, len(handler.groups)+1)
	groups = append(groups, handler.groups...)
	groups = append(groups, name)
	return &slogHandler{mu: handler.mu, prefix: handler.prefix, groups: groups}
}

func formatSlogAttr(groups []string, attr slog.Attr) string {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return ""
	}
	if attr.Value.Kind() == slog.KindGroup {
		nested := append(append([]string{}, groups...), attr.Key)
		var builder strings.Builder
		for _, item := range attr.Value.Group() {
			builder.WriteString(formatSlogAttr(nested, item))
		}
		return builder.String()
	}
	key := attr.Key
	if len(groups) > 0 {
		key = strings.Join(groups, ".") + "." + key
	}
	return fmt.Sprintf("\t%s:%v", key, attr.Value.Any())
}
