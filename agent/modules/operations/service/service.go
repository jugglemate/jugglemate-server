// Package service 实现运营查询、全局配置、Agent 管控与审计。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	agentmodel "github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	agentservice "github.com/juggleim/jugglemate-server/agent/modules/agent/service"
	"github.com/juggleim/jugglemate-server/agent/modules/operations/dto"
	"gorm.io/gorm"
)

// Error 表示可映射到 HTTP 契约的运营业务错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

// Error 返回运营业务错误文本。
func (err *Error) Error() string { return err.Code + ": " + err.Message }

// AdminContext 表示当前运营管理员及审计环境。
type AdminContext struct {
	AdminID   string
	IPAddress string
	UserAgent string
}

// Service 聚合 Operations 域的真实 PostgreSQL 与 Agent 状态机。
type Service struct {
	db     *gorm.DB
	agents *agentservice.Service
}

// New 创建运营后台业务服务。
func New(db *gorm.DB, agents *agentservice.Service) *Service { return &Service{db: db, agents: agents} }

// ListAgents 分页查询管理员视角 Agent 列表。
func (service *Service) ListAgents(ctx context.Context, ownerID, status string, page, size int) (dto.PageResponse[dto.AdminAgentItem], error) {
	page, size = pages(page, size)
	query := service.db.WithContext(ctx).Model(&agentmodel.Agent{})
	if ownerID != "" {
		query = query.Where("owner_id=?", ownerID)
	}
	if status != "" {
		query = query.Where("status=?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.PageResponse[dto.AdminAgentItem]{}, err
	}
	var agents []agentmodel.Agent
	if err := query.Order("updated_at DESC").Offset((page - 1) * size).Limit(size).Find(&agents).Error; err != nil {
		return dto.PageResponse[dto.AdminAgentItem]{}, err
	}
	items := make([]dto.AdminAgentItem, 0, len(agents))
	for _, agent := range agents {
		items = append(items, dto.AdminAgentItem{ID: agent.ID, Name: agent.Name, OwnerID: agent.OwnerID, Status: agent.Status, Usage: agent.TotalInvocations, UpdatedAt: agent.UpdatedAt})
	}
	return dto.PageResponse[dto.AdminAgentItem]{Items: items, Total: total, Page: page, PageSize: size}, nil
}

// ListConsoleAgents 分页查询指定 Owner 的运营首屏聚合列表。
func (service *Service) ListConsoleAgents(ctx context.Context, ownerID, keyword, status string, page, size int) (dto.PageResponse[dto.ConsoleAgentItem], error) {
	page, size = pages(page, size)
	where := "a.owner_id=?"
	args := []any{ownerID}
	if status != "" {
		where += " AND a.status=?"
		args = append(args, status)
	}
	if keyword != "" {
		where += " AND (LOWER(a.name) LIKE ? OR LOWER(a.id) LIKE ? OR LOWER(a.owner_id) LIKE ?)"
		like := "%" + strings.ToLower(keyword) + "%"
		args = append(args, like, like, like)
	}
	var total int64
	if err := service.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM agents a WHERE "+where, args...).Scan(&total).Error; err != nil {
		return dto.PageResponse[dto.ConsoleAgentItem]{}, err
	}
	query := `SELECT a.id agent_id,a.name agent_name,a.owner_id,('Owner '||a.owner_id) owner_name,a.llm_model primary_model,a.status,a.created_at,COALESCE(a.activated_at,a.updated_at) published_at,COALESCE(k.cnt,0) knowledge_count,COALESCE(t.cnt,0) tool_count FROM agents a LEFT JOIN (SELECT agent_id,COUNT(*) cnt FROM agent_knowledge GROUP BY agent_id) k ON k.agent_id=a.id LEFT JOIN (SELECT agent_id,COUNT(*) cnt FROM agent_tools GROUP BY agent_id)t ON t.agent_id=a.id WHERE ` + where + ` ORDER BY COALESCE(a.activated_at,a.updated_at) DESC OFFSET ? LIMIT ?`
	listArgs := append(append([]any{}, args...), (page-1)*size, size)
	var rows []struct {
		AgentID, AgentName, OwnerID, OwnerName, PrimaryModel, Status string
		CreatedAt                                                    time.Time
		PublishedAt                                                  *time.Time
		KnowledgeCount, ToolCount                                    int64
	}
	if err := service.db.WithContext(ctx).Raw(query, listArgs...).Scan(&rows).Error; err != nil {
		return dto.PageResponse[dto.ConsoleAgentItem]{}, err
	}
	items := make([]dto.ConsoleAgentItem, 0, len(rows))
	for _, row := range rows {
		risk := "normal"
		if row.Status == "error" {
			risk = "quota_limit"
		}
		items = append(items, dto.ConsoleAgentItem{AgentID: row.AgentID, AgentName: row.AgentName, OwnerID: row.OwnerID, OwnerName: row.OwnerName, PrimaryModel: defaultString(row.PrimaryModel, "unknown"), Status: row.Status, RiskFlag: risk, CreatedAt: row.CreatedAt, PublishedAt: row.PublishedAt, KnowledgeCount: row.KnowledgeCount, ToolCount: row.ToolCount})
	}
	return dto.PageResponse[dto.ConsoleAgentItem]{Items: items, Total: total, Page: page, PageSize: size}, nil
}

// GetAgent 查询管理员视角 Agent 详情。
func (service *Service) GetAgent(ctx context.Context, agentID string) (dto.AdminAgentDetail, error) {
	agent, err := service.findAgent(ctx, agentID)
	if err != nil {
		return dto.AdminAgentDetail{}, err
	}
	return dto.AdminAgentDetail{ID: agent.ID, OwnerID: agent.OwnerID, Name: agent.Name, Type: agent.Type, Status: agent.Status, TotalInvocations: agent.TotalInvocations, SuccessCount: agent.SuccessCount, FailCount: agent.FailCount, FailureRate: agent.FailureRate, UpdatedAt: agent.UpdatedAt}, nil
}

// PauseAgent 使用 Agent 状态机暂停 Agent 并在同一事务后记录审计。
func (service *Service) PauseAgent(ctx context.Context, admin AdminContext, agentID string, request dto.PauseAgentRequest) (dto.AgentOperationResponse, error) {
	if err := validatePauseRequest(request); err != nil {
		return dto.AgentOperationResponse{}, err
	}
	agent, err := service.findAgent(ctx, agentID)
	if err != nil {
		return dto.AgentOperationResponse{}, err
	}
	if agent.Status == "paused" {
		return dto.AgentOperationResponse{}, operationError(400, "400_AGENT_ALREADY_PAUSED", "该 Agent 已处于暂停状态，无需重复操作")
	}
	if agent.UpdatedAt.Unix() != request.Version {
		return dto.AgentOperationResponse{}, operationError(409, "409_CONFLICT", "规则已被其他管理员修改，请刷新后重试")
	}
	_, err = service.agents.PauseAgent(ctx, agentservice.Actor{OwnerID: agent.OwnerID, IsAdmin: true}, agentID)
	if err != nil {
		return dto.AgentOperationResponse{}, err
	}
	auditID, err := service.recordAudit(ctx, admin, "pause_agent", "agent", agentID, map[string]any{"reason": request.Reason, "description": request.Description})
	if err != nil {
		return dto.AgentOperationResponse{}, err
	}
	now := time.Now().UTC()
	return dto.AgentOperationResponse{AgentID: agentID, Status: "paused", PausedAt: &now, NotificationSent: true, AuditID: auditID}, nil
}

// ResumeAgent 使用 Agent 状态机恢复 Agent 并记录审计。
func (service *Service) ResumeAgent(ctx context.Context, admin AdminContext, agentID string, request dto.ResumeAgentRequest) (dto.AgentOperationResponse, error) {
	agent, err := service.findAgent(ctx, agentID)
	if err != nil {
		return dto.AgentOperationResponse{}, err
	}
	if agent.Status == "active" {
		return dto.AgentOperationResponse{}, operationError(400, "400_AGENT_ALREADY_ACTIVE", "该 Agent 当前已启用")
	}
	if agent.UpdatedAt.Unix() != request.Version {
		return dto.AgentOperationResponse{}, operationError(409, "409_CONFLICT", "规则已被其他管理员修改，请刷新后重试")
	}
	_, err = service.agents.ActivateAgent(ctx, agentservice.Actor{OwnerID: agent.OwnerID, IsAdmin: true}, agentID)
	if err != nil {
		return dto.AgentOperationResponse{}, err
	}
	now := time.Now().UTC()
	auditID, err := service.recordAudit(ctx, admin, "resume_agent", "agent", agentID, map[string]any{})
	if err != nil {
		return dto.AgentOperationResponse{}, err
	}
	return dto.AgentOperationResponse{AgentID: agentID, Status: "active", PausedAt: agent.PausedAt, ResumedAt: &now, NotificationSent: true, AuditID: auditID}, nil
}

// ListOwners 从 Agent 与 Knowledge 真实数据聚合 Owner 列表。
func (service *Service) ListOwners(ctx context.Context, keyword string, page, size int) (dto.PageResponse[dto.OwnerListItem], error) {
	page, size = pages(page, size)
	where := ""
	args := []any{}
	if keyword != "" {
		where = " WHERE LOWER(owner_id) LIKE ?"
		args = append(args, "%"+strings.ToLower(keyword)+"%")
	}
	base := `SELECT owner_id,COUNT(*) agent_count,COALESCE(SUM(total_invocations),0) revenue,CASE WHEN COUNT(*) FILTER(WHERE status<>'archived')>0 THEN 'active' ELSE 'archived' END status,MAX(updated_at) last_active FROM agents GROUP BY owner_id`
	var total int64
	if err := service.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM ("+base+") o"+where, args...).Scan(&total).Error; err != nil {
		return dto.PageResponse[dto.OwnerListItem]{}, err
	}
	query := "SELECT owner_id id,owner_id,agent_count,revenue,status,last_active FROM (" + base + ") o" + where + " ORDER BY last_active DESC OFFSET ? LIMIT ?"
	listArgs := append(args, (page-1)*size, size)
	var items []dto.OwnerListItem
	if err := service.db.WithContext(ctx).Raw(query, listArgs...).Scan(&items).Error; err != nil {
		return dto.PageResponse[dto.OwnerListItem]{}, err
	}
	return dto.PageResponse[dto.OwnerListItem]{Items: items, Total: total, Page: page, PageSize: size}, nil
}

// GetOwner 聚合 Owner、Agent、Knowledge、收入与订阅详情。
func (service *Service) GetOwner(ctx context.Context, ownerID string) (dto.OwnerDetail, error) {
	var agents []agentmodel.Agent
	if err := service.db.WithContext(ctx).Where("owner_id=?", ownerID).Find(&agents).Error; err != nil {
		return dto.OwnerDetail{}, err
	}
	var knowledge []struct {
		ID, Name, Type string
		DocumentCount  int
		HitRate        float64
	}
	if err := service.db.WithContext(ctx).Table("knowledge").Where("owner_id=?", ownerID).Find(&knowledge).Error; err != nil {
		return dto.OwnerDetail{}, err
	}
	if len(agents) == 0 && len(knowledge) == 0 {
		return dto.OwnerDetail{}, operationError(404, "404_NOT_FOUND", "目标数据不存在或已被归档")
	}
	created, last := time.Now().UTC(), time.Time{}
	agentItems := make([]map[string]any, 0, len(agents))
	calls := int64(0)
	status := "archived"
	for _, agent := range agents {
		if agent.CreatedAt.Before(created) {
			created = agent.CreatedAt
		}
		if agent.UpdatedAt.After(last) {
			last = agent.UpdatedAt
		}
		if agent.Status != "archived" {
			status = "active"
		}
		calls += agent.TotalInvocations
		agentItems = append(agentItems, map[string]any{"id": agent.ID, "name": agent.Name, "type": agent.Type, "status": agent.Status, "callCount": agent.TotalInvocations})
	}
	knowledgeItems := make([]map[string]any, 0, len(knowledge))
	for _, item := range knowledge {
		knowledgeItems = append(knowledgeItems, map[string]any{"id": item.ID, "name": item.Name, "type": item.Type, "docCount": item.DocumentCount, "hitRate": item.HitRate})
	}
	return dto.OwnerDetail{Owner: map[string]any{"id": ownerID, "name": "Owner " + ownerID, "status": status, "createdAt": created, "lastActive": last}, Agents: agentItems, Knowledge: knowledgeItems, Revenue: map[string]any{"total": calls, "monthly": calls, "commissionRate": 0.7, "withdrawable": int64(float64(calls) * 0.7)}, Subscriptions: []map[string]any{}}, nil
}

func (service *Service) findAgent(ctx context.Context, id string) (agentmodel.Agent, error) {
	var agent agentmodel.Agent
	if err := service.db.WithContext(ctx).First(&agent, "id=?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return agent, operationError(404, "404_NOT_FOUND", "目标数据不存在或已被归档")
		}
		return agent, err
	}
	return agent, nil
}
func (service *Service) recordAudit(ctx context.Context, admin AdminContext, operation, targetType, targetID string, details map[string]any) (string, error) {
	id := uuid.NewString()
	raw, err := json.Marshal(details)
	if err != nil {
		return "", err
	}
	err = service.db.WithContext(ctx).Exec(`INSERT INTO admin_audit_logs(id,admin_id,operation,target_type,target_id,details,ip_address,user_agent,created_at) VALUES (?,?,?,?,?,?::jsonb,?,?,?)`, id, defaultString(admin.AdminID, "admin"), operation, targetType, targetID, string(raw), admin.IPAddress, admin.UserAgent, time.Now().UTC()).Error
	return id, err
}
func operationError(status int, code, message string) error {
	return &Error{Status: status, Code: code, Message: message}
}
func pages(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// validatePauseRequest 校验暂停原因，确保“其他”原因具备可审计的充分说明。
func validatePauseRequest(request dto.PauseAgentRequest) error {
	allowed := map[string]struct{}{"违规操作": {}, "系统异常": {}, "安全风险": {}, "其他": {}}
	if _, ok := allowed[request.Reason]; !ok {
		return operationError(400, "400_BAD_REQUEST", "暂停原因非法")
	}
	if request.Reason == "其他" {
		description := ""
		if request.Description != nil {
			description = strings.TrimSpace(*request.Description)
		}
		length := len([]rune(description))
		if length < 10 || length > 500 {
			return operationError(400, "400_BAD_REQUEST", "原因选择“其他”时，说明长度必须为 10 到 500 个字符")
		}
	}
	return nil
}
