package service

import (
	"context"
	"errors"
	"time"

	agentservice "github.com/juggleim/jugglemate-server/agent/modules/agent/service"
	"github.com/juggleim/jugglemate-server/agent/modules/operations/dto"
	"gorm.io/gorm"
)

// PlatformStats 查询快照优先、实时聚合兜底的平台统计。
func (service *Service) PlatformStats(ctx context.Context, from, to time.Time) (dto.PlatformStats, error) {
	if to.IsZero() {
		to = time.Now().UTC()
	}
	if from.IsZero() {
		from = to
	}
	if from.After(to) {
		from, to = to, from
	}
	start := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	stats, found, err := service.snapshotStats(ctx, start, end)
	if err != nil {
		return dto.PlatformStats{}, err
	}
	if !found {
		stats, err = service.realtimeStats(ctx, start, end)
		if err != nil {
			return dto.PlatformStats{}, err
		}
	}
	rate := 0.0
	if stats.TotalCalls > 0 {
		rate = float64(stats.ErrorCount) / float64(stats.TotalCalls)
	}
	return dto.PlatformStats{Period: map[string]string{"from": start.Format("2006-01-02"), "to": to.Format("2006-01-02")}, Stats: map[string]any{"total_calls": stats.TotalCalls, "total_revenue": stats.TotalRevenue, "error_rate": rate, "active_agents": stats.ActiveAgents, "active_owners": stats.ActiveOwners, "active_users": stats.ActiveUsers}, Trends: map[string]string{"calls_trend": "+0%", "revenue_trend": "+0%", "error_rate_trend": "0%"}}, nil
}

type platformStats struct{ TotalCalls, ErrorCount, TotalRevenue, ActiveAgents, ActiveOwners, ActiveUsers int64 }

// snapshotStats 汇总日期范围内的平台快照；活跃指标取区间最大值以避免跨日重复累计。
func (service *Service) snapshotStats(ctx context.Context, start, end time.Time) (platformStats, bool, error) {
	var result platformStats
	var count int64
	query := `SELECT COUNT(*) snapshot_count,COALESCE(SUM(total_calls),0) total_calls,COALESCE(SUM(error_count),0) error_count,COALESCE(SUM(total_revenue),0) total_revenue,COALESCE(MAX(active_agents),0) active_agents,COALESCE(MAX(active_owners),0) active_owners,COALESCE(MAX(active_users),0) active_users FROM platform_stats_snapshots WHERE date>=? AND date<?`
	var row struct {
		SnapshotCount int64
		TotalCalls    int64
		ErrorCount    int64
		TotalRevenue  int64
		ActiveAgents  int64
		ActiveOwners  int64
		ActiveUsers   int64
	}
	if err := service.db.WithContext(ctx).Raw(query, start, end).Scan(&row).Error; err != nil {
		return result, false, err
	}
	count = row.SnapshotCount
	result = platformStats{TotalCalls: row.TotalCalls, ErrorCount: row.ErrorCount, TotalRevenue: row.TotalRevenue, ActiveAgents: row.ActiveAgents, ActiveOwners: row.ActiveOwners, ActiveUsers: row.ActiveUsers}
	return result, count > 0, nil
}

// realtimeStats 在没有快照时从模型调用、Agent 和会话数据实时计算统计值。
func (service *Service) realtimeStats(ctx context.Context, start, end time.Time) (platformStats, error) {
	var result platformStats
	var calls struct {
		TotalCalls, ErrorCount int64
		TotalRevenue           float64
	}
	if err := service.db.WithContext(ctx).Raw(`SELECT COUNT(*) total_calls,COUNT(*) FILTER(WHERE status<>'success') error_count,COALESCE(SUM(cost),0) total_revenue FROM llm_model_calls WHERE request_time>=? AND request_time<?`, start, end).Scan(&calls).Error; err != nil {
		return result, err
	}
	if calls.TotalCalls == 0 {
		if err := service.db.WithContext(ctx).Raw(`SELECT COALESCE(SUM(total_invocations),0) total_calls,COALESCE(SUM(fail_count),0) error_count,COALESCE(SUM(total_invocations),0) total_revenue FROM agents`).Scan(&calls).Error; err != nil {
			return result, err
		}
	}
	var agents struct{ ActiveAgents, ActiveOwners int64 }
	if err := service.db.WithContext(ctx).Raw("SELECT COUNT(*) FILTER(WHERE status='active') active_agents,COUNT(DISTINCT owner_id) FILTER(WHERE status<>'archived') active_owners FROM agents").Scan(&agents).Error; err != nil {
		return result, err
	}
	var activeUsers int64
	if err := service.db.WithContext(ctx).Raw("SELECT COUNT(DISTINCT user_id) FROM conversations WHERE created_at>=? AND created_at<?", start, end).Scan(&activeUsers).Error; err != nil {
		return result, err
	}
	return platformStats{TotalCalls: calls.TotalCalls, ErrorCount: calls.ErrorCount, TotalRevenue: int64(calls.TotalRevenue + 0.5), ActiveAgents: agents.ActiveAgents, ActiveOwners: agents.ActiveOwners, ActiveUsers: activeUsers}, nil
}

// ListAuditConversations 分页查询审计会话及消息数量。
func (service *Service) ListAuditConversations(ctx context.Context, page, size int, from, to *time.Time, agentID, userID string) (map[string]any, error) {
	page, size = pages(page, size)
	where := "1=1"
	args := []any{}
	if from != nil {
		where += " AND c.created_at>=?"
		args = append(args, *from)
	}
	if to != nil {
		where += " AND c.created_at<?"
		args = append(args, to.Add(24*time.Hour))
	}
	if agentID != "" {
		where += " AND c.agent_id=?"
		args = append(args, agentID)
	}
	if userID != "" {
		where += " AND c.user_id=?"
		args = append(args, userID)
	}
	var total int64
	if err := service.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM conversations c WHERE "+where, args...).Scan(&total).Error; err != nil {
		return nil, err
	}
	query := `SELECT c.id,c.agent_id,c.user_id,COALESCE(m.cnt,0) message_count,c.created_at,c.status FROM conversations c LEFT JOIN(SELECT conversation_id,COUNT(*) cnt FROM messages GROUP BY conversation_id)m ON m.conversation_id=c.id WHERE ` + where + ` ORDER BY c.created_at DESC OFFSET ? LIMIT ?`
	listArgs := append(append([]any{}, args...), (page-1)*size, size)
	var items []dto.AuditConversationItem
	if err := service.db.WithContext(ctx).Raw(query, listArgs...).Scan(&items).Error; err != nil {
		return nil, err
	}
	return map[string]any{"conversations": items, "pagination": map[string]any{"page": page, "pageSize": size, "total": total}}, nil
}

// GetAuditConversation 查询只读审计会话详情并记录查看审计。
func (service *Service) GetAuditConversation(ctx context.Context, admin AdminContext, conversationID string) (map[string]any, error) {
	var conversation struct {
		ID, AgentID, UserID string
		CreatedAt           time.Time
	}
	if err := service.db.WithContext(ctx).Table("conversations").First(&conversation, "id=?", conversationID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, operationError(404, "404_NOT_FOUND", "目标数据不存在或已被归档")
		}
		return nil, err
	}
	var messages []struct{ Role, Content string }
	if err := service.db.WithContext(ctx).Table("messages").Select("role,content").Where("conversation_id=?", conversationID).Order("created_at").Find(&messages).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		items = append(items, map[string]any{"role": message.Role, "content": message.Content})
	}
	if _, err := service.recordAudit(ctx, admin, "view_conversation", "conversation", conversationID, map[string]any{}); err != nil {
		return nil, err
	}
	return map[string]any{"conversation": map[string]any{"id": conversation.ID, "agentId": conversation.AgentID, "userId": conversation.UserID, "createdAt": conversation.CreatedAt, "messages": items}, "readonly": true}, nil
}

// ErrorCode 返回运营错误业务码。
func ErrorCode(err error) string {
	var target *Error
	if errors.As(err, &target) {
		return target.Code
	}
	var agentErr *agentservice.Error
	if errors.As(err, &agentErr) {
		return agentErr.Code
	}
	return "500_OPERATIONS_ERROR"
}

// StatusCode 返回运营错误语义状态。
func StatusCode(err error) int {
	var target *Error
	if errors.As(err, &target) {
		return target.Status
	}
	var agentErr *agentservice.Error
	if errors.As(err, &agentErr) {
		return agentErr.Status
	}
	return 500
}
