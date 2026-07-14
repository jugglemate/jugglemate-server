// Package service 实现旧版 Capability Tool/Skill 注册、发现与测试。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/capability/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/capability/model"
	"gorm.io/gorm"
)

// Error 表示 Capability 业务异常。
type Error struct {
	Status  int
	Code    string
	Message string
}

// Error 返回异常消息。
func (err *Error) Error() string { return err.Message }

// Service 管理 Tool/Skill 聚合并执行测试调用。
type Service struct{ db *gorm.DB }

// New 创建 Capability 服务。
func New(db *gorm.DB) *Service { return &Service{db: db} }

// CreateTool 创建 Tool 并计算风险等级。
func (service *Service) CreateTool(ctx context.Context, ownerID string, payload dto.ToolWrite) (dto.ToolRead, error) {
	applyToolDefaults(&payload)
	if err := validateTool(payload); err != nil {
		return dto.ToolRead{}, err
	}
	risk := evaluateRisk(payload.HTTPMethod, payload.APIEndpoint, payload.Name, pointerValue(payload.Description))
	entity := model.Tool{ID: uuid.NewString(), OwnerID: ownerID, Name: payload.Name, Description: payload.Description, Visibility: payload.Visibility, Status: "draft", APIEndpoint: payload.APIEndpoint, HTTPMethod: strings.ToUpper(payload.HTTPMethod), InputSchema: payload.InputSchema, OutputSchema: payload.OutputSchema, RiskLevel: risk, RequiresApproval: risk == "high" || risk == "critical", TimeoutSeconds: int64(payload.TimeoutSeconds)}
	if err := service.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return dto.ToolRead{}, err
	}
	return toolRead(entity), nil
}

// GetTool 查询 Owner 的 Tool。
func (service *Service) GetTool(ctx context.Context, ownerID, id string) (dto.ToolRead, error) {
	entity, err := service.findTool(ctx, id, ownerID, false)
	if err != nil {
		return dto.ToolRead{}, err
	}
	return toolRead(*entity), nil
}

// UpdateTool 更新 Owner 的 Tool 配置并重新评估风险。
func (service *Service) UpdateTool(ctx context.Context, ownerID, id string, payload dto.ToolUpdate) (dto.ToolRead, error) {
	entity, err := service.findTool(ctx, id, ownerID, false)
	if err != nil {
		return dto.ToolRead{}, err
	}
	values := map[string]any{}
	if payload.Name != nil {
		values["name"] = strings.TrimSpace(*payload.Name)
		entity.Name = strings.TrimSpace(*payload.Name)
	}
	if payload.Description != nil {
		values["description"] = payload.Description
		entity.Description = payload.Description
	}
	if payload.Visibility != nil {
		values["visibility"] = *payload.Visibility
		entity.Visibility = *payload.Visibility
	}
	if payload.APIEndpoint != nil {
		values["api_endpoint"] = *payload.APIEndpoint
		entity.APIEndpoint = *payload.APIEndpoint
	}
	if payload.HTTPMethod != nil {
		values["http_method"] = strings.ToUpper(*payload.HTTPMethod)
		entity.HTTPMethod = strings.ToUpper(*payload.HTTPMethod)
	}
	if payload.InputSchema != nil {
		values["input_schema"] = payload.InputSchema
	}
	if payload.OutputSchema != nil {
		values["output_schema"] = payload.OutputSchema
	}
	if payload.TimeoutSeconds != nil {
		values["timeout_seconds"] = *payload.TimeoutSeconds
		entity.TimeoutSeconds = int64(*payload.TimeoutSeconds)
	}
	write := dto.ToolWrite{Name: entity.Name, Description: entity.Description, Visibility: entity.Visibility, APIEndpoint: entity.APIEndpoint, HTTPMethod: entity.HTTPMethod, InputSchema: entity.InputSchema, OutputSchema: entity.OutputSchema, TimeoutSeconds: int(entity.TimeoutSeconds)}
	if payload.InputSchema != nil {
		write.InputSchema = payload.InputSchema
	}
	if payload.OutputSchema != nil {
		write.OutputSchema = payload.OutputSchema
	}
	if err := validateTool(write); err != nil {
		return dto.ToolRead{}, err
	}
	risk := evaluateRisk(entity.HTTPMethod, entity.APIEndpoint, entity.Name, pointerValue(entity.Description))
	values["risk_level"], values["requires_approval"] = risk, risk == "high" || risk == "critical"
	if err := service.db.WithContext(ctx).Model(entity).Updates(values).Error; err != nil {
		return dto.ToolRead{}, err
	}
	return service.GetTool(ctx, ownerID, id)
}

// DeleteTool 软删除 Tool。
func (service *Service) DeleteTool(ctx context.Context, ownerID, id string) error {
	entity, err := service.findTool(ctx, id, ownerID, false)
	if err != nil {
		return err
	}
	now := time.Now()
	return service.db.WithContext(ctx).Model(entity).Updates(map[string]any{"status": "deleted", "deleted_at": now}).Error
}

// TestTool 调用 Tool endpoint，并按结果激活/测试中状态与累计成功率。
func (service *Service) TestTool(ctx context.Context, ownerID, id string, payload map[string]any) (dto.TestResponse, error) {
	entity, err := service.findTool(ctx, id, ownerID, false)
	if err != nil {
		return dto.TestResponse{}, err
	}
	result := execute(ctx, *entity, payload)
	status := entity.Status
	if result.Success {
		status = "active"
	} else if status == "draft" {
		status = "testing"
	}
	successCount := entity.SuccessRate * float64(entity.CallCount)
	if result.Success {
		successCount++
	}
	newCount := entity.CallCount + 1
	_ = service.db.WithContext(ctx).Model(entity).Updates(map[string]any{"status": status, "call_count": newCount, "success_rate": successCount / float64(newCount)}).Error
	return result, nil
}

// ListTools 查询 public 或 Owner Tool。
func (service *Service) ListTools(ctx context.Context, ownerID string, public bool, q, risk string, page, pageSize int) ([]dto.ToolRead, int64, error) {
	query := service.db.WithContext(ctx).Model(&model.Tool{}).Where("status <> ? AND deleted_at IS NULL", "deleted")
	if public {
		if ownerID != "" {
			query = query.Where("visibility='public' OR owner_id=?", ownerID)
		} else {
			query = query.Where("visibility='public'")
		}
		query = query.Where("status IN ?", []string{"active", "deprecated"})
	} else {
		query = query.Where("owner_id=?", ownerID)
	}
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", like, like)
	}
	if risk != "" {
		query = query.Where("risk_level=?", risk)
	}
	page, pageSize = paging(page, pageSize)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var entities []model.Tool
	if err := query.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return nil, 0, err
	}
	items := make([]dto.ToolRead, 0, len(entities))
	for _, entity := range entities {
		items = append(items, toolRead(entity))
	}
	return items, total, nil
}

// CreateSkill 创建 Skill 并按所含 Tool 最高风险定级。
func (service *Service) CreateSkill(ctx context.Context, ownerID string, payload dto.SkillWrite) (dto.SkillRead, error) {
	applySkillDefaults(&payload)
	if err := validateSchema(payload.InputSchema); err != nil {
		return dto.SkillRead{}, err
	}
	if err := validateSchema(payload.OutputSchema); err != nil {
		return dto.SkillRead{}, err
	}
	var created model.Skill
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tools, err := activeTools(tx, payload.ToolIDs)
		if err != nil {
			return err
		}
		risk := highestRisk(tools)
		created = model.Skill{ID: uuid.NewString(), OwnerID: ownerID, Name: payload.Name, Description: payload.Description, Visibility: payload.Visibility, Status: "draft", Category: payload.Category, InputSchema: payload.InputSchema, OutputSchema: payload.OutputSchema, RiskLevel: risk, RequiresApproval: risk == "high" || risk == "critical"}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		return replaceSkillTools(tx, created.ID, payload.ToolIDs)
	})
	if err != nil {
		return dto.SkillRead{}, err
	}
	return service.GetSkill(ctx, ownerID, created.ID)
}

// GetSkill 查询 Owner 的 Skill 及 Tool 顺序。
func (service *Service) GetSkill(ctx context.Context, ownerID, id string) (dto.SkillRead, error) {
	entity, err := service.findSkill(ctx, id, ownerID, false)
	if err != nil {
		return dto.SkillRead{}, err
	}
	ids, err := service.skillToolIDs(ctx, id)
	if err != nil {
		return dto.SkillRead{}, err
	}
	return skillRead(*entity, ids), nil
}

// UpdateSkill 更新 Skill 与关联 Tool。
func (service *Service) UpdateSkill(ctx context.Context, ownerID, id string, payload dto.SkillUpdate) (dto.SkillRead, error) {
	entity, err := service.findSkill(ctx, id, ownerID, false)
	if err != nil {
		return dto.SkillRead{}, err
	}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		values := map[string]any{}
		if payload.Name != nil {
			values["name"] = strings.TrimSpace(*payload.Name)
		}
		if payload.Description != nil {
			values["description"] = payload.Description
		}
		if payload.Visibility != nil {
			if !validVisibility(*payload.Visibility) {
				return bad("400_INVALID_VISIBILITY", "visibility 非法")
			}
			values["visibility"] = *payload.Visibility
		}
		if payload.Category != nil {
			values["category"] = payload.Category
		}
		if payload.InputSchema != nil {
			if err := validateSchema(payload.InputSchema); err != nil {
				return err
			}
			values["input_schema"] = payload.InputSchema
		}
		if payload.OutputSchema != nil {
			if err := validateSchema(payload.OutputSchema); err != nil {
				return err
			}
			values["output_schema"] = payload.OutputSchema
		}
		if payload.ToolIDs != nil {
			tools, err := activeTools(tx, *payload.ToolIDs)
			if err != nil {
				return err
			}
			risk := highestRisk(tools)
			values["risk_level"], values["requires_approval"] = risk, risk == "high" || risk == "critical"
			if err := replaceSkillTools(tx, id, *payload.ToolIDs); err != nil {
				return err
			}
		}
		return tx.Model(entity).Updates(values).Error
	})
	if err != nil {
		return dto.SkillRead{}, err
	}
	return service.GetSkill(ctx, ownerID, id)
}

// DeleteSkill 软删除 Skill。
func (service *Service) DeleteSkill(ctx context.Context, ownerID, id string) error {
	entity, err := service.findSkill(ctx, id, ownerID, false)
	if err != nil {
		return err
	}
	now := time.Now()
	return service.db.WithContext(ctx).Model(entity).Updates(map[string]any{"status": "deleted", "deleted_at": now}).Error
}

// TestSkill 按固定顺序执行 Skill 所含 Tool。
func (service *Service) TestSkill(ctx context.Context, ownerID, id string, payload map[string]any) (dto.TestResponse, error) {
	entity, err := service.findSkill(ctx, id, ownerID, false)
	if err != nil {
		return dto.TestResponse{}, err
	}
	ids, err := service.skillToolIDs(ctx, id)
	if err != nil {
		return dto.TestResponse{}, err
	}
	started := time.Now()
	for _, toolID := range ids {
		tool, err := service.findTool(ctx, toolID, "", true)
		if err != nil {
			failed := toolID
			code, msg := "404_TOOL_NOT_FOUND", "Skill 所含 Tool 不存在"
			return dto.TestResponse{ErrorCode: &code, ErrorMessage: &msg, FailedToolID: &failed, LatencyMS: int(time.Since(started).Milliseconds())}, nil
		}
		result := execute(ctx, *tool, payload)
		if !result.Success {
			result.FailedToolID = &toolID
			return result, nil
		}
	}
	_ = service.db.WithContext(ctx).Model(entity).Update("status", "active").Error
	return dto.TestResponse{Success: true, Output: map[string]any{}, LatencyMS: int(time.Since(started).Milliseconds())}, nil
}

// ListSkills 查询 public 或 Owner Skill。
func (service *Service) ListSkills(ctx context.Context, ownerID string, public bool, q, category, risk string, page, pageSize int) ([]dto.SkillRead, int64, error) {
	query := service.db.WithContext(ctx).Model(&model.Skill{}).Where("status <> ? AND deleted_at IS NULL", "deleted")
	if public {
		query = query.Where("visibility='public' AND status IN ?", []string{"active", "deprecated"})
	} else {
		query = query.Where("owner_id=?", ownerID)
	}
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", like, like)
	}
	if category != "" {
		query = query.Where("category=?", category)
	}
	if risk != "" {
		query = query.Where("risk_level=?", risk)
	}
	page, pageSize = paging(page, pageSize)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var entities []model.Skill
	if err := query.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return nil, 0, err
	}
	items := make([]dto.SkillRead, 0, len(entities))
	for _, entity := range entities {
		ids, err := service.skillToolIDs(ctx, entity.ID)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, skillRead(entity, ids))
	}
	return items, total, nil
}

func execute(ctx context.Context, tool model.Tool, payload map[string]any) dto.TestResponse {
	started := time.Now()
	raw, _ := json.Marshal(payload)
	var body io.Reader
	if tool.HTTPMethod != "GET" {
		body = bytes.NewReader(raw)
	}
	endpoint, err := url.Parse(tool.APIEndpoint)
	if err != nil {
		code, msg := "400_INVALID_ENDPOINT", "API endpoint 非法"
		return dto.TestResponse{ErrorCode: &code, ErrorMessage: &msg}
	}
	if tool.HTTPMethod == "GET" {
		query := endpoint.Query()
		for key, value := range payload {
			query.Set(key, fmt.Sprint(value))
		}
		endpoint.RawQuery = query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, tool.HTTPMethod, endpoint.String(), body)
	if err != nil {
		code, msg := "400_INVALID_ENDPOINT", err.Error()
		return dto.TestResponse{ErrorCode: &code, ErrorMessage: &msg}
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Duration(tool.TimeoutSeconds) * time.Second}
	response, err := client.Do(request)
	latency := int(time.Since(started).Milliseconds())
	if err != nil {
		code, msg := "502_TOOL_TEST_FAILED", err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			code, msg = "504_TIMEOUT", "测试超时"
		}
		return dto.TestResponse{ErrorCode: &code, ErrorMessage: &msg, LatencyMS: latency}
	}
	defer response.Body.Close()
	content, _ := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if response.StatusCode >= 300 {
		code, msg := "502_TOOL_TEST_FAILED", fmt.Sprintf("HTTP %d: %s", response.StatusCode, string(content))
		return dto.TestResponse{ErrorCode: &code, ErrorMessage: &msg, LatencyMS: latency}
	}
	output := map[string]any{}
	if json.Unmarshal(content, &output) != nil {
		output["raw"] = string(content)
	}
	return dto.TestResponse{Success: true, Output: output, LatencyMS: latency}
}
func (service *Service) findTool(ctx context.Context, id, owner string, anyOwner bool) (*model.Tool, error) {
	var entity model.Tool
	query := service.db.WithContext(ctx).Where("id=? AND status<>? AND deleted_at IS NULL", id, "deleted")
	if !anyOwner && owner != "" {
		query = query.Where("owner_id=?", owner)
	}
	if err := query.First(&entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, notFound("404_TOOL_NOT_FOUND", "Tool 不存在")
		}
		return nil, err
	}
	return &entity, nil
}
func (service *Service) findSkill(ctx context.Context, id, owner string, anyOwner bool) (*model.Skill, error) {
	var entity model.Skill
	query := service.db.WithContext(ctx).Where("id=? AND status<>? AND deleted_at IS NULL", id, "deleted")
	if !anyOwner {
		query = query.Where("owner_id=?", owner)
	}
	if err := query.First(&entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, notFound("404_TOOL_NOT_FOUND", "Skill 不存在")
		}
		return nil, err
	}
	return &entity, nil
}
func (service *Service) skillToolIDs(ctx context.Context, id string) ([]string, error) {
	var rows []model.SkillTool
	if err := service.db.WithContext(ctx).Where("skill_id=?", id).Order("order_index").Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ToolID)
	}
	return ids, nil
}
func activeTools(tx *gorm.DB, ids []string) ([]model.Tool, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(unique(ids)) != len(ids) {
		return nil, bad("400_INVALID_REQUEST", "toolIds 不允许重复")
	}
	var tools []model.Tool
	if err := tx.Where("id IN ? AND status IN ? AND deleted_at IS NULL", ids, []string{"active", "deprecated"}).Find(&tools).Error; err != nil {
		return nil, err
	}
	if len(tools) != len(unique(ids)) {
		return nil, notFound("409_CAPABILITY_NOT_ACTIVE", "Tool 不存在或未激活")
	}
	return tools, nil
}
func replaceSkillTools(tx *gorm.DB, skillID string, ids []string) error {
	if err := tx.Where("skill_id=?", skillID).Delete(&model.SkillTool{}).Error; err != nil {
		return err
	}
	for index, id := range ids {
		if err := tx.Create(&model.SkillTool{ID: uuid.NewString(), SkillID: skillID, ToolID: id, OrderIndex: index}).Error; err != nil {
			return err
		}
	}
	return nil
}
func validateTool(p dto.ToolWrite) error {
	if strings.TrimSpace(p.Name) == "" || !validVisibility(p.Visibility) {
		return bad("400_INVALID_REQUEST", "Tool 名称或 visibility 非法")
	}
	parsed, err := url.ParseRequestURI(p.APIEndpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return bad("400_INVALID_ENDPOINT", "API endpoint 非法")
	}
	method := strings.ToUpper(p.HTTPMethod)
	if method != "GET" && method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
		return bad("400_INVALID_ENDPOINT", "HTTP method 非法")
	}
	if p.TimeoutSeconds < 1 || p.TimeoutSeconds > 300 {
		return bad("400_INVALID_REQUEST", "timeoutSeconds 必须为 1-300")
	}
	if err := validateSchema(p.InputSchema); err != nil {
		return err
	}
	return validateSchema(p.OutputSchema)
}
func validateSchema(schema map[string]any) error {
	if schema == nil {
		return nil
	}
	if schema["type"] != "object" {
		return bad("400_INVALID_SCHEMA", "Schema 根节点 type 必须为 object")
	}
	return nil
}
func evaluateRisk(method, endpoint, name, description string) string {
	text := strings.ToLower(endpoint + " " + name + " " + description)
	for _, word := range []string{"transfer", "swap", "sign", "trade", "transaction", "withdraw"} {
		if strings.Contains(text, word) {
			return "high"
		}
	}
	if strings.ToUpper(method) == "GET" {
		return "low"
	}
	return "medium"
}
func highestRisk(tools []model.Tool) string {
	rank := map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}
	result := "low"
	for _, tool := range tools {
		if rank[tool.RiskLevel] > rank[result] {
			result = tool.RiskLevel
		}
	}
	return result
}
func applyToolDefaults(p *dto.ToolWrite) {
	if p.Visibility == "" {
		p.Visibility = "private"
	}
	if p.HTTPMethod == "" {
		p.HTTPMethod = "GET"
	}
	if p.TimeoutSeconds == 0 {
		p.TimeoutSeconds = 30
	}
}
func applySkillDefaults(p *dto.SkillWrite) {
	if p.Visibility == "" {
		p.Visibility = "private"
	}
}
func validVisibility(value string) bool { return value == "private" || value == "public" }
func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func toolRead(e model.Tool) dto.ToolRead {
	return dto.ToolRead{ID: e.ID, Name: e.Name, Description: e.Description, Visibility: e.Visibility, Status: e.Status, APIEndpoint: e.APIEndpoint, HTTPMethod: e.HTTPMethod, InputSchema: e.InputSchema, OutputSchema: e.OutputSchema, RiskLevel: e.RiskLevel, RequiresApproval: e.RequiresApproval, TimeoutSeconds: e.TimeoutSeconds, CallCount: e.CallCount, SuccessRate: e.SuccessRate, CreatedAt: e.CreatedAt.Format(time.RFC3339), UpdatedAt: e.UpdatedAt.Format(time.RFC3339)}
}
func skillRead(e model.Skill, ids []string) dto.SkillRead {
	return dto.SkillRead{ID: e.ID, Name: e.Name, Description: e.Description, Visibility: e.Visibility, Status: e.Status, Category: e.Category, ToolIDs: ids, InputSchema: e.InputSchema, OutputSchema: e.OutputSchema, RiskLevel: e.RiskLevel, RequiresApproval: e.RequiresApproval, CallCount: e.CallCount, SuccessRate: e.SuccessRate, CreatedAt: e.CreatedAt.Format(time.RFC3339), UpdatedAt: e.UpdatedAt.Format(time.RFC3339)}
}
func unique(ids []string) map[string]bool {
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}
func paging(page, size int) (int, int) {
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
func bad(code, message string) error { return &Error{Status: 400, Code: code, Message: message} }
func notFound(code, message string) error {
	status := 404
	if strings.HasPrefix(code, "409_") {
		status = 409
	}
	return &Error{Status: status, Code: code, Message: message}
}
