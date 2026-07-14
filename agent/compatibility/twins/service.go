package twins

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	agentdto "github.com/juggleim/jugglemate-server/agent/modules/agent/dto"
	agentmodel "github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	agentservice "github.com/juggleim/jugglemate-server/agent/modules/agent/service"
	knowledgedto "github.com/juggleim/jugglemate-server/agent/modules/knowledge/dto"
	knowledgemodel "github.com/juggleim/jugglemate-server/agent/modules/knowledge/model"
	knowledgeservice "github.com/juggleim/jugglemate-server/agent/modules/knowledge/service"
	reasoningmodel "github.com/juggleim/jugglemate-server/agent/modules/reasoning/model"
	reasoningservice "github.com/juggleim/jugglemate-server/agent/modules/reasoning/service"
	"github.com/juggleim/jugglemate-server/commons/agentclient"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service 将旧 Twin、Material、Training、Version、Evaluation 和 Chat 契约映射到 Go Agent。
type Service struct {
	db        *gorm.DB
	agents    *agentservice.Service
	knowledge *knowledgeservice.Service
	reasoning *reasoningservice.Service
}

// New 创建 Twin 进程内兼容服务。
func New(db *gorm.DB, agents *agentservice.Service, knowledge *knowledgeservice.Service, reasoning *reasoningservice.Service) *Service {
	return &Service{db: db, agents: agents, knowledge: knowledge, reasoning: reasoning}
}

// CreateTwin 创建 Agent、专属知识库与稳定 unique_name 映射。
//
// 简要描述：先通过 Agent/Knowledge 领域服务建立合法聚合，再创建兼容映射；任一后续
// 步骤失败会尽力清理前置数据，避免旧 MySQL 已同步但新 Agent 侧留下不可见孤儿。
func (service *Service) CreateTwin(ctx context.Context, ownerID string, request map[string]any) (*agentclient.Twin, int, error) {
	uniqueName := valueString(request, "unique_name")
	displayName := valueString(request, "display_name")
	if ownerID == "" || uniqueName == "" || displayName == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("owner_id、unique_name 和 display_name 不能为空")
	}
	if _, err := service.findMapping(ctx, ownerID, uniqueName); err == nil {
		return nil, http.StatusConflict, fmt.Errorf("Twin 已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, http.StatusInternalServerError, err
	}
	prompt := firstNonEmpty(valueString(request, "prompts"), valueString(request, "greeting"), "你是一个友好、专业的智能助手。")
	agentType := "assistant"
	detail, err := service.agents.CreateAgent(ctx, agentservice.Actor{OwnerID: ownerID}, agentdto.CreateRequest{Name: &displayName, Type: &agentType, Prompt: &prompt})
	if err != nil {
		return nil, statusFromError(err), err
	}
	trigger := false
	knowledge, err := service.knowledge.Create(ctx, ownerID, knowledgedto.Create{Name: displayName + " 知识库", Type: "offline_document", SourceType: "file", TriggerVectorize: &trigger})
	if err != nil {
		_ = service.agents.DeleteAgent(context.WithoutCancel(ctx), agentservice.Actor{OwnerID: ownerID}, detail.ID)
		return nil, statusFromError(err), err
	}
	if err := service.db.WithContext(ctx).Model(&knowledgemodel.Knowledge{}).Where("id = ?", knowledge.ID).Update("status", "ready").Error; err != nil {
		service.cleanupCreatedTwin(ctx, ownerID, detail.ID, knowledge.ID)
		return nil, http.StatusInternalServerError, err
	}
	if _, err := service.agents.MountKnowledge(ctx, agentservice.Actor{OwnerID: ownerID}, detail.ID, knowledge.ID); err != nil {
		service.cleanupCreatedTwin(ctx, ownerID, detail.ID, knowledge.ID)
		return nil, statusFromError(err), err
	}
	if _, err := service.agents.ActivateAgent(ctx, agentservice.Actor{OwnerID: ownerID}, detail.ID); err != nil {
		service.cleanupCreatedTwin(ctx, ownerID, detail.ID, knowledge.ID)
		return nil, statusFromError(err), err
	}
	entity := mapping{ID: uuid.NewString(), OwnerID: ownerID, UniqueName: uniqueName, AgentID: detail.ID, KnowledgeID: knowledge.ID, DisplayName: displayName, AvatarURL: valueString(request, "avatar_url"), Greeting: valueString(request, "greeting"), Status: "untrained"}
	if err := service.db.WithContext(ctx).Create(&entity).Error; err != nil {
		service.cleanupCreatedTwin(ctx, ownerID, detail.ID, knowledge.ID)
		return nil, http.StatusConflict, err
	}
	result := toTwin(entity)
	return &result, http.StatusCreated, nil
}

// ListTwins 查询指定 Owner 的全部兼容 Twin。
func (service *Service) ListTwins(ctx context.Context, ownerID, _ string) ([]agentclient.Twin, int, error) {
	var values []mapping
	if err := service.db.WithContext(ctx).Where("owner_id = ?", ownerID).Order("updated_at DESC").Find(&values).Error; err != nil {
		return nil, http.StatusInternalServerError, err
	}
	result := make([]agentclient.Twin, 0, len(values))
	for _, item := range values {
		result = append(result, toTwin(item))
	}
	return result, http.StatusOK, nil
}

// GetTwin 查询 Owner 可访问的单个 Twin。
func (service *Service) GetTwin(ctx context.Context, ownerID, uniqueName string) (*agentclient.Twin, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	result := toTwin(*entity)
	return &result, http.StatusOK, nil
}

// UpdateTwin 同步更新兼容展示字段与 Agent Profile。
func (service *Service) UpdateTwin(ctx context.Context, ownerID, uniqueName string, request map[string]any) (*agentclient.Twin, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	updates := map[string]any{}
	agentUpdate := agentdto.UpdateRequest{AgentID: entity.AgentID}
	if value := valueString(request, "display_name"); value != "" {
		updates["display_name"] = value
		agentUpdate.Name = &value
	}
	if value, ok := request["avatar_url"]; ok {
		text := fmt.Sprint(value)
		updates["avatar_url"] = text
	}
	if value, ok := request["greeting"]; ok {
		text := fmt.Sprint(value)
		updates["greeting"] = text
	}
	if value := valueString(request, "prompts"); value != "" {
		agentUpdate.Prompt = &value
	}
	if agentUpdate.Name != nil || agentUpdate.Prompt != nil {
		if _, err := service.agents.UpdateAgent(ctx, agentservice.Actor{OwnerID: ownerID}, agentUpdate); err != nil {
			return nil, statusFromError(err), err
		}
	}
	if len(updates) > 0 {
		if err := service.db.WithContext(ctx).Model(entity).Updates(updates).Error; err != nil {
			return nil, http.StatusInternalServerError, err
		}
	}
	entity, err = service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	result := toTwin(*entity)
	return &result, http.StatusOK, nil
}

// DeleteTwin 归档新 Agent 并删除旧 unique_name 映射。
func (service *Service) DeleteTwin(ctx context.Context, ownerID, uniqueName string) (int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return statusFromError(err), err
	}
	detail, err := service.agents.GetAgent(ctx, agentservice.Actor{OwnerID: ownerID}, entity.AgentID)
	if err == nil {
		switch detail.Status {
		case "draft":
			err = service.agents.DeleteAgent(ctx, agentservice.Actor{OwnerID: ownerID}, entity.AgentID)
		case "active":
			_, err = service.agents.PauseAgent(ctx, agentservice.Actor{OwnerID: ownerID}, entity.AgentID)
			if err == nil {
				_, err = service.agents.ArchiveAgent(ctx, agentservice.Actor{OwnerID: ownerID}, entity.AgentID, detail.Name)
			}
		case "paused":
			_, err = service.agents.ArchiveAgent(ctx, agentservice.Actor{OwnerID: ownerID}, entity.AgentID, detail.Name)
		}
	}
	if err != nil {
		return statusFromError(err), err
	}
	if err := service.db.WithContext(ctx).Delete(entity).Error; err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusNoContent, nil
}

// AddMaterial 将文本、URL 或文件元数据写入 Twin 专属知识库。
func (service *Service) AddMaterial(ctx context.Context, ownerID, uniqueName string, request map[string]any) (*agentclient.Material, int, error) {
	id := firstNonEmpty(valueString(request, "id"), "mat_"+uuid.NewString())
	materialType := firstNonEmpty(valueString(request, "type"), "text")
	content := firstNonEmpty(valueString(request, "content"), valueString(request, "url"), valueString(request, "source"))
	return service.addMaterial(ctx, ownerID, uniqueName, material{ID: id, MaterialType: materialType, Title: valueString(request, "title"), Source: firstNonEmpty(valueString(request, "source"), content), SizeBytes: int64(len(content)), Content: content})
}

// AddMaterialFile 将上传文件内容写入 Twin 专属知识库；二进制内容只保留可检索元数据。
func (service *Service) AddMaterialFile(ctx context.Context, ownerID, uniqueName, fileName string, content []byte, title string, fields map[string]string) (*agentclient.Material, int, error) {
	id := ""
	if fields != nil {
		id = fields["id"]
	}
	id = firstNonEmpty(id, "mat_"+uuid.NewString())
	text := string(content)
	if !utf8.Valid(content) {
		text = firstNonEmpty(title, fileName)
	}
	return service.addMaterial(ctx, ownerID, uniqueName, material{ID: id, MaterialType: "file", Title: firstNonEmpty(title, fileName), Source: fileName, SizeBytes: int64(len(content)), Content: text})
}

// ListMaterials 查询 Twin 已同步到 Go Knowledge 的材料列表。
func (service *Service) ListMaterials(ctx context.Context, ownerID, uniqueName, _ string) ([]agentclient.Material, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	var values []material
	if err := service.db.WithContext(ctx).Where("mapping_id = ?", entity.ID).Order("created_at DESC").Find(&values).Error; err != nil {
		return nil, http.StatusInternalServerError, err
	}
	result := make([]agentclient.Material, 0, len(values))
	for _, item := range values {
		result = append(result, toMaterial(item))
	}
	return result, http.StatusOK, nil
}

// DeleteMaterial 删除兼容材料及对应知识分块。
func (service *Service) DeleteMaterial(ctx context.Context, ownerID, uniqueName, materialID string) (int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return statusFromError(err), err
	}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ? AND mapping_id = ?", materialID, entity.ID).Delete(&material{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("id = ? AND knowledge_id = ?", materialID, entity.KnowledgeID).Delete(&knowledgemodel.Vector{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&mapping{}).Where("id = ?", entity.ID).UpdateColumn("materials_count", gorm.Expr("GREATEST(materials_count - 1, 0)")).Error; err != nil {
			return err
		}
		return tx.Model(&knowledgemodel.Knowledge{}).Where("id = ?", entity.KnowledgeID).Updates(map[string]any{"document_count": gorm.Expr("GREATEST(document_count - 1, 0)"), "vector_count": gorm.Expr("GREATEST(vector_count - 1, 0)")}).Error
	})
	if err != nil {
		return statusFromError(err), err
	}
	return http.StatusNoContent, nil
}

// StartTraining 以同步成功任务表达 Go Knowledge 已可直接用于推理，并生成新版本。
func (service *Service) StartTraining(ctx context.Context, ownerID, uniqueName string, request map[string]any) (*agentclient.Job, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	mode := firstNonEmpty(valueString(request, "mode"), "mind")
	now := time.Now().UTC()
	progress := 100
	created := job{ID: "job_" + uuid.NewString(), MappingID: entity.ID, JobType: "training", Status: "succeeded", Progress: &progress, Result: map[string]any{"knowledge_id": entity.KnowledgeID}, ErrorValue: map[string]any{}, StartedAt: &now, FinishedAt: &now}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&version{}).Where("mapping_id = ?", entity.ID).Count(&count).Error; err != nil {
			return err
		}
		versionName := "v" + strconv.FormatInt(count+1, 10)
		if err := tx.Create(&version{ID: uuid.NewString(), MappingID: entity.ID, Version: versionName, Mode: mode, TrainingJobID: created.ID, MaterialsCount: entity.MaterialsCount}).Error; err != nil {
			return err
		}
		score := 0.5
		comment := "知识库尚无材料，完成结构与链路检查"
		if entity.MaterialsCount > 0 {
			score = 1
			comment = "知识材料已就绪且可检索"
		}
		dimensions := []map[string]any{{"name": "knowledge_readiness", "score": score, "comment": comment}}
		if err := tx.Create(&evaluation{ID: "eval_" + uuid.NewString(), MappingID: entity.ID, Version: versionName, OverallScore: score, Dimensions: dimensions, SummaryMD: "训练版本的知识就绪度检查已完成。"}).Error; err != nil {
			return err
		}
		return tx.Model(&mapping{}).Where("id = ?", entity.ID).Updates(map[string]any{"status": "trained", "training_mode": mode}).Error
	})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	result := toJob(created, entity.UniqueName)
	return &result, http.StatusAccepted, nil
}

// StartEvaluation 创建确定性评估结果和已完成任务。
func (service *Service) StartEvaluation(ctx context.Context, ownerID, uniqueName string, request map[string]any) (*agentclient.Job, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	now := time.Now().UTC()
	progress := 100
	evaluationID := "eval_" + uuid.NewString()
	created := job{ID: "job_" + uuid.NewString(), MappingID: entity.ID, JobType: "evaluation", Status: "succeeded", Progress: &progress, Result: map[string]any{"evaluation_id": evaluationID}, ErrorValue: map[string]any{}, StartedAt: &now, FinishedAt: &now}
	dimensions := []map[string]any{{"name": "knowledge_readiness", "score": 1.0, "comment": "Go Knowledge 已就绪"}}
	if err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		return tx.Create(&evaluation{ID: evaluationID, MappingID: entity.ID, Version: firstNonEmpty(valueString(request, "version"), entity.ActiveVersion), OverallScore: 1, Dimensions: dimensions, SummaryMD: "Go Agent 兼容评估通过。"}).Error
	}); err != nil {
		return nil, http.StatusInternalServerError, err
	}
	result := toJob(created, uniqueName)
	return &result, http.StatusAccepted, nil
}

// GetEvaluation 查询一条兼容评估详情。
func (service *Service) GetEvaluation(ctx context.Context, ownerID, uniqueName, evaluationID string) (*agentclient.Evaluation, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	var value evaluation
	if err := service.db.WithContext(ctx).Where("id = ? AND mapping_id = ?", evaluationID, entity.ID).First(&value).Error; err != nil {
		return nil, statusFromError(err), err
	}
	result := toEvaluation(value, uniqueName)
	return &result, http.StatusOK, nil
}

// ListEvaluations 查询 Twin 的知识就绪度评估历史。
func (service *Service) ListEvaluations(ctx context.Context, ownerID, uniqueName, _ string) ([]agentclient.Evaluation, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	var values []evaluation
	if err := service.db.WithContext(ctx).Where("mapping_id = ?", entity.ID).Order("created_at DESC").Find(&values).Error; err != nil {
		return nil, http.StatusInternalServerError, err
	}
	result := make([]agentclient.Evaluation, 0, len(values))
	for _, item := range values {
		result = append(result, toEvaluation(item, uniqueName))
	}
	return result, http.StatusOK, nil
}

// ListJobs 查询 Twin 训练与评估任务。
func (service *Service) ListJobs(ctx context.Context, ownerID, uniqueName, _ string) ([]agentclient.Job, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	var values []job
	if err := service.db.WithContext(ctx).Where("mapping_id = ?", entity.ID).Order("created_at DESC").Find(&values).Error; err != nil {
		return nil, http.StatusInternalServerError, err
	}
	result := make([]agentclient.Job, 0, len(values))
	for _, item := range values {
		result = append(result, toJob(item, uniqueName))
	}
	return result, http.StatusOK, nil
}

// GetJob 按 Owner 查询一个兼容任务。
func (service *Service) GetJob(ctx context.Context, ownerID, jobID string) (*agentclient.Job, int, error) {
	var value job
	query := service.db.WithContext(ctx).Table("twin_compat_jobs j").Select("j.*").Joins("JOIN twin_compat_mappings m ON m.id=j.mapping_id").Where("j.id = ? AND m.owner_id = ?", jobID, ownerID)
	if err := query.First(&value).Error; err != nil {
		return nil, statusFromError(err), err
	}
	var entity mapping
	if err := service.db.WithContext(ctx).First(&entity, "id = ?", value.MappingID).Error; err != nil {
		return nil, statusFromError(err), err
	}
	result := toJob(value, entity.UniqueName)
	return &result, http.StatusOK, nil
}

// CancelJob 取消尚未终止的兼容任务；已完成任务保持幂等返回。
func (service *Service) CancelJob(ctx context.Context, ownerID, jobID string) (*agentclient.Job, int, error) {
	result, code, err := service.GetJob(ctx, ownerID, jobID)
	if err != nil {
		return nil, code, err
	}
	if result.Status != "succeeded" && result.Status != "failed" && result.Status != "cancelled" {
		now := time.Now().UTC()
		if err := service.db.WithContext(ctx).Model(&job{}).Where("id = ?", jobID).Updates(map[string]any{"status": "cancelled", "finished_at": now}).Error; err != nil {
			return nil, http.StatusInternalServerError, err
		}
		result.Status = "cancelled"
		result.FinishedAt = now.Format(time.RFC3339)
	}
	return result, http.StatusOK, nil
}

// ListVersions 查询 Twin 训练生成的版本。
func (service *Service) ListVersions(ctx context.Context, ownerID, uniqueName string) ([]agentclient.Version, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	var values []version
	if err := service.db.WithContext(ctx).Where("mapping_id = ?", entity.ID).Order("created_at DESC").Find(&values).Error; err != nil {
		return nil, http.StatusInternalServerError, err
	}
	result := make([]agentclient.Version, 0, len(values))
	for _, item := range values {
		result = append(result, toVersion(item))
	}
	return result, http.StatusOK, nil
}

// ActivateVersion 原子切换当前 Twin 版本。
func (service *Service) ActivateVersion(ctx context.Context, ownerID, uniqueName, versionName string) (*agentclient.Twin, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&version{}).Where("mapping_id = ? AND version = ?", entity.ID, versionName).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&version{}).Where("mapping_id = ?", entity.ID).Update("active", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&version{}).Where("mapping_id = ? AND version = ?", entity.ID, versionName).Update("active", true).Error; err != nil {
			return err
		}
		return tx.Model(&mapping{}).Where("id = ?", entity.ID).Update("active_version", versionName).Error
	})
	if err != nil {
		return nil, statusFromError(err), err
	}
	return service.GetTwin(ctx, ownerID, uniqueName)
}

// GetCurrentVersion 查询当前激活版本。
func (service *Service) GetCurrentVersion(ctx context.Context, ownerID, uniqueName string) (*agentclient.Version, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	var value version
	query := service.db.WithContext(ctx).Where("mapping_id = ?", entity.ID)
	if entity.ActiveVersion != "" {
		query = query.Where("version = ?", entity.ActiveVersion)
	} else {
		query = query.Where("active = true")
	}
	if err := query.Order("created_at DESC").First(&value).Error; err != nil {
		return nil, statusFromError(err), err
	}
	result := toVersion(value)
	return &result, http.StatusOK, nil
}

// Chat 使用映射后的 Agent 执行完整 Reasoning 链路。
func (service *Service) Chat(ctx context.Context, customerID, customerSource, uniqueName string, request agentclient.ChatRequest) (*agentclient.ChatResponse, int, error) {
	var entity mapping
	if err := service.db.WithContext(ctx).Where("unique_name = ?", uniqueName).First(&entity).Error; err != nil {
		return nil, statusFromError(err), err
	}
	result, err := service.reasoning.Run(ctx, reasoningservice.Request{AgentID: entity.AgentID, OwnerID: entity.OwnerID, UserID: customerID, Input: request.Message, EnableHistoryContext: true, Metadata: map[string]any{"source": "auto", "customer_source": customerSource}})
	if err != nil {
		return nil, statusFromError(err), err
	}
	messageID := "msg_" + uuid.NewString()
	var message reasoningmodel.Message
	if service.db.WithContext(ctx).Where("conversation_id = ? AND role = 'assistant'", result.ConversationID).Order("created_at DESC").First(&message).Error == nil {
		messageID = message.ID
	}
	return &agentclient.ChatResponse{MessageID: messageID, SessionID: result.ConversationID, Reply: result.Answer, Fallback: result.Status != "completed", CreatedAt: time.Now().UTC().Format(time.RFC3339)}, http.StatusOK, nil
}

// addMaterial 以事务保证兼容材料、知识分块和双侧计数一致。
func (service *Service) addMaterial(ctx context.Context, ownerID, uniqueName string, item material) (*agentclient.Material, int, error) {
	entity, err := service.findMapping(ctx, ownerID, uniqueName)
	if err != nil {
		return nil, statusFromError(err), err
	}
	item.MappingID = entity.ID
	if item.Source == "" {
		item.Source = item.MaterialType
	}
	if item.SizeBytes == 0 {
		item.SizeBytes = int64(len(item.Content))
	}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error; err != nil {
			return err
		}
		var exists int64
		if err := tx.Model(&knowledgemodel.Vector{}).Where("id = ?", item.ID).Count(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			source := fmt.Sprintf(`{"material_id":%q,"title":%q,"source":%q}`, item.ID, item.Title, item.Source)
			if err := tx.Exec(`INSERT INTO knowledge_vectors(id,knowledge_id,chunk_id,document_index,chunk_index,content_chunk,source_position,created_at) VALUES (?,?,?,?,?,?,?::jsonb,?)`, item.ID, entity.KnowledgeID, item.ID, entity.MaterialsCount, 0, firstNonEmpty(item.Content, item.Title, item.Source), source, time.Now().UTC()).Error; err != nil {
				return err
			}
			if err := tx.Model(&mapping{}).Where("id = ?", entity.ID).UpdateColumn("materials_count", gorm.Expr("materials_count + 1")).Error; err != nil {
				return err
			}
			return tx.Model(&knowledgemodel.Knowledge{}).Where("id = ?", entity.KnowledgeID).Updates(map[string]any{"document_count": gorm.Expr("document_count + 1"), "vector_count": gorm.Expr("vector_count + 1")}).Error
		}
		return nil
	})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	result := toMaterial(item)
	return &result, http.StatusCreated, nil
}

// cleanupCreatedTwin 补偿清理尚未建立兼容映射的 Agent 与 Knowledge。
//
// 简要描述：Twin 创建跨越多个领域服务，无法用一个领域事务包裹；失败后使用独立短事务
// 删除本次刚创建且尚未对外可见的数据，保证重试不会留下 active Agent 或孤立知识库。
func (service *Service) cleanupCreatedTwin(parent context.Context, ownerID, agentID, knowledgeID string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer cancel()
	_ = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ? AND knowledge_id = ?", agentID, knowledgeID).Delete(&agentmodel.KnowledgeBinding{}).Error; err != nil {
			return err
		}
		for _, table := range []string{"knowledge_vectors", "knowledge_retrieval_logs", "knowledge_stats", "knowledge_task", "knowledge_file"} {
			if err := tx.Exec("DELETE FROM "+table+" WHERE knowledge_id = ?", knowledgeID).Error; err != nil {
				return err
			}
		}
		if err := tx.Exec("DELETE FROM agents WHERE id = ? AND owner_id = ?", agentID, ownerID).Error; err != nil {
			return err
		}
		return tx.Exec("DELETE FROM knowledge WHERE id = ? AND owner_id = ?", knowledgeID, ownerID).Error
	})
}

func (service *Service) findMapping(ctx context.Context, ownerID, uniqueName string) (*mapping, error) {
	var entity mapping
	return &entity, service.db.WithContext(ctx).Where("owner_id = ? AND unique_name = ?", ownerID, uniqueName).First(&entity).Error
}

func toTwin(value mapping) agentclient.Twin {
	return agentclient.Twin{UniqueName: value.UniqueName, DisplayName: value.DisplayName, AvatarURL: value.AvatarURL, Greeting: value.Greeting, OwnerID: value.OwnerID, Status: value.Status, ActiveVersion: value.ActiveVersion, TrainingMode: value.TrainingMode, MaterialsCount: value.MaterialsCount, CreatedAt: value.CreatedAt.Format(time.RFC3339), UpdatedAt: value.UpdatedAt.Format(time.RFC3339)}
}
func toMaterial(value material) agentclient.Material {
	return agentclient.Material{ID: value.ID, Type: value.MaterialType, Title: value.Title, Source: value.Source, SizeBytes: value.SizeBytes, CreatedAt: value.CreatedAt.Format(time.RFC3339)}
}
func toJob(value job, uniqueName string) agentclient.Job {
	return agentclient.Job{JobID: value.ID, Type: value.JobType, Twin: uniqueName, Status: value.Status, Progress: value.Progress, Result: value.Result, Error: value.ErrorValue, CreatedAt: value.CreatedAt.Format(time.RFC3339), StartedAt: formatTime(value.StartedAt), FinishedAt: formatTime(value.FinishedAt)}
}
func toVersion(value version) agentclient.Version {
	return agentclient.Version{Version: value.Version, Mode: value.Mode, Active: value.Active, TrainingJobID: value.TrainingJobID, MaterialsCount: value.MaterialsCount, CreatedAt: value.CreatedAt.Format(time.RFC3339)}
}
func toEvaluation(value evaluation, uniqueName string) agentclient.Evaluation {
	result := agentclient.Evaluation{EvaluationID: value.ID, Twin: uniqueName, Version: value.Version, CreatedAt: value.CreatedAt.Format(time.RFC3339), OverallScore: value.OverallScore, SummaryMD: value.SummaryMD}
	for _, dimension := range value.Dimensions {
		result.Dimensions = append(result.Dimensions, struct {
			Name    string  `json:"name"`
			Score   float64 `json:"score"`
			Comment string  `json:"comment"`
		}{Name: valueString(dimension, "name"), Score: valueFloat(dimension, "score"), Comment: valueString(dimension, "comment")})
	}
	return result
}
func valueString(values map[string]any, key string) string {
	if value, ok := values[key]; ok && value != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}
func valueFloat(values map[string]any, key string) float64 {
	switch value := values[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		result, _ := strconv.ParseFloat(fmt.Sprint(value), 64)
		return result
	}
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
func statusFromError(err error) int {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound
	}
	var agentErr *agentservice.Error
	if errors.As(err, &agentErr) {
		return agentErr.Status
	}
	var reasoningErr *reasoningservice.Error
	if errors.As(err, &reasoningErr) {
		return reasoningErr.Status
	}
	var knowledgeErr *knowledgeservice.Error
	if errors.As(err, &knowledgeErr) {
		return knowledgeErr.Status
	}
	return http.StatusInternalServerError
}

var _ agentclient.Backend = (*Service)(nil)
