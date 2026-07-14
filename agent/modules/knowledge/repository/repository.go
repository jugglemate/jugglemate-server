// Package repository 实现 Knowledge 聚合的 PostgreSQL 数据访问。
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/knowledge/model"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

// Repository 负责知识库、文件、任务和向量的一致性读写。
type Repository struct{ db *gorm.DB }

// SearchRow 表示全文或向量检索的一条归一结果。
type SearchRow struct {
	KnowledgeID  string  `gorm:"column:knowledge_id"`
	ContentChunk string  `gorm:"column:content_chunk"`
	Score        float64 `gorm:"column:score"`
}

// New 创建 Knowledge PostgreSQL 仓储。
func New(db *gorm.DB) *Repository { return &Repository{db: db} }

// Create 创建知识库并可选挂载到 Agent。
func (repo *Repository) Create(ctx context.Context, entity *model.Knowledge, agentID *string) error {
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entity).Error; err != nil {
			return err
		}
		if agentID != nil && *agentID != "" {
			var agentCount int64
			if err := tx.Table("agents").Where("id = ? AND status <> ?", *agentID, "archived").Count(&agentCount).Error; err != nil {
				return err
			}
			if agentCount == 0 {
				return gorm.ErrRecordNotFound
			}
			return tx.Exec(`INSERT INTO agent_knowledge(id, agent_id, knowledge_id, mounted_at) VALUES (?, ?, ?, now()) ON CONFLICT(agent_id, knowledge_id) DO NOTHING`, uuid.NewString(), *agentID, entity.ID).Error
		}
		return nil
	})
}

// Find 查询未物理删除的知识库。
func (repo *Repository) Find(ctx context.Context, id string) (*model.Knowledge, error) {
	var entity model.Knowledge
	err := repo.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// ListOwner 分页查询 Owner 的知识库。
func (repo *Repository) ListOwner(ctx context.Context, ownerID, status, agentID string, page, pageSize int) ([]model.Knowledge, int64, error) {
	query := repo.db.WithContext(ctx).Model(&model.Knowledge{}).Where("knowledge.owner_id = ? AND knowledge.deleted_at IS NULL AND knowledge.status <> ?", ownerID, "archived")
	if status != "" {
		query = query.Where("knowledge.status = ?", status)
	}
	if agentID != "" {
		query = query.Joins("JOIN agent_knowledge ak ON ak.knowledge_id = knowledge.id").Where("ak.agent_id = ?", agentID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Knowledge
	err := query.Order("knowledge.updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

// Update 更新知识库字段。
func (repo *Repository) Update(ctx context.Context, id string, values map[string]any) error {
	result := repo.db.WithContext(ctx).Model(&model.Knowledge{}).Where("id = ? AND deleted_at IS NULL", id).Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CreateFile 创建文件记录并绑定知识库。
func (repo *Repository) CreateFile(ctx context.Context, file *model.File, switchToFileSource bool) error {
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(file).Error; err != nil {
			return err
		}
		values := map[string]any{"file_id": file.ID, "updated_at": gorm.Expr("now()")}
		if switchToFileSource {
			values["source_type"] = "file"
		}
		result := tx.Model(&model.Knowledge{}).Where("id = ?", file.KnowledgeID).Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// FindFile 查询知识文件。
func (repo *Repository) FindFile(ctx context.Context, id string) (*model.File, error) {
	var file model.File
	if err := repo.db.WithContext(ctx).First(&file, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

// StartTask 原子创建任务并把知识库推进到 pending。
func (repo *Repository) StartTask(ctx context.Context, knowledgeID string) (*model.Task, error) {
	task := &model.Task{ID: "kv-" + uuid.NewString(), KnowledgeID: knowledgeID, Status: "pending", Stage: "queued"}
	err := repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		return tx.Model(&model.Knowledge{}).Where("id = ?", knowledgeID).Updates(map[string]any{"status": "pending", "current_task_id": task.ID, "failed_at": nil, "last_error_code": nil, "last_error_message": nil, "updated_at": gorm.Expr("now()")}).Error
	})
	return task, err
}

// FindTask 查询任务；taskID 为空时返回知识库最近任务。
func (repo *Repository) FindTask(ctx context.Context, knowledgeID string, taskID *string) (*model.Task, error) {
	query := repo.db.WithContext(ctx).Where("knowledge_id = ?", knowledgeID)
	if taskID != nil {
		query = query.Where("id = ?", *taskID)
	} else {
		query = query.Order("created_at DESC")
	}
	var task model.Task
	if err := query.First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// MarkTask 更新任务及知识库进度状态。
func (repo *Repository) MarkTask(ctx context.Context, taskID string, taskValues, knowledgeValues map[string]any) error {
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.Task
		if err := tx.Clauses().First(&task, "id = ?", taskID).Error; err != nil {
			return err
		}
		if err := tx.Model(&task).Updates(taskValues).Error; err != nil {
			return err
		}
		return tx.Model(&model.Knowledge{}).Where("id = ?", task.KnowledgeID).Updates(knowledgeValues).Error
	})
}

// ReplaceVectors 原子替换某知识库的全部向量，并标记任务完成。
func (repo *Repository) ReplaceVectors(ctx context.Context, taskID string, vectors []model.Vector, fileText *string) error {
	now := time.Now()
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.Task
		if err := tx.First(&task, "id = ?", taskID).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_id = ?", task.KnowledgeID).Delete(&model.Vector{}).Error; err != nil {
			return err
		}
		if len(vectors) > 0 {
			if err := tx.CreateInBatches(vectors, 64).Error; err != nil {
				return err
			}
		}
		if fileText != nil {
			if err := tx.Model(&model.File{}).Where("knowledge_id = ?", task.KnowledgeID).Updates(map[string]any{"text_content": *fileText, "text_extracted_at": now}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&task).Updates(map[string]any{"status": "completed", "stage": "completed", "progress": 100, "completed_at": now, "error_code": nil, "error_message": nil}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Knowledge{}).Where("id = ?", task.KnowledgeID).Updates(map[string]any{"status": "ready", "document_count": 1, "vector_count": len(vectors), "current_task_id": nil, "last_error_code": nil, "last_error_message": nil, "failed_at": nil}).Error
	})
}

// FailTask 将任务和知识库标记为失败。
func (repo *Repository) FailTask(ctx context.Context, taskID, code, message string, retry int) error {
	now := time.Now()
	return repo.MarkTask(ctx, taskID, map[string]any{"status": "failed", "stage": "failed", "progress": 100, "retry_count": retry, "error_code": code, "error_message": message, "completed_at": now}, map[string]any{"status": "failed", "last_error_code": code, "last_error_message": message, "failed_at": now})
}

// ListChunks 返回真实持久化分块。
func (repo *Repository) ListChunks(ctx context.Context, knowledgeID string) ([]model.Vector, error) {
	var items []model.Vector
	err := repo.db.WithContext(ctx).Where("knowledge_id = ?", knowledgeID).Order("chunk_index ASC").Find(&items).Error
	return items, err
}

// SearchKeyword 在 Owner 可见的 ready 知识库中执行 PostgreSQL 全文排序。
func (repo *Repository) SearchKeyword(ctx context.Context, ownerID, query string, topK int) ([]SearchRow, error) {
	var rows []SearchRow
	err := repo.db.WithContext(ctx).Raw(`SELECT v.knowledge_id, v.content_chunk, ts_rank_cd(to_tsvector('simple', v.content_chunk), plainto_tsquery('simple', ?)) AS score FROM knowledge_vectors v JOIN knowledge k ON k.id=v.knowledge_id WHERE k.owner_id=? AND k.status='ready' AND k.deleted_at IS NULL AND to_tsvector('simple', v.content_chunk) @@ plainto_tsquery('simple', ?) ORDER BY score DESC LIMIT ?`, query, ownerID, query, topK).Scan(&rows).Error
	return rows, err
}

// SearchVector 执行 pgvector 余弦检索。
func (repo *Repository) SearchVector(ctx context.Context, ownerID string, embedding []float32, topK int) ([]SearchRow, error) {
	var rows []SearchRow
	vector := pgvector.NewVector(embedding)
	err := repo.db.WithContext(ctx).Raw(`SELECT v.knowledge_id, v.content_chunk, 1-(v.embedding <=> ?) AS score FROM knowledge_vectors v JOIN knowledge k ON k.id=v.knowledge_id WHERE k.owner_id=? AND k.status='ready' AND k.deleted_at IS NULL AND v.embedding IS NOT NULL ORDER BY v.embedding <=> ? LIMIT ?`, vector, ownerID, vector, topK).Scan(&rows).Error
	return rows, err
}

// SearchKeywordScoped 在指定知识库集合内执行 PostgreSQL 全文检索。
func (repo *Repository) SearchKeywordScoped(ctx context.Context, ownerID string, knowledgeIDs []string, query string, topK int) ([]SearchRow, error) {
	var rows []SearchRow
	err := repo.db.WithContext(ctx).Raw(`SELECT v.knowledge_id, v.content_chunk, ts_rank_cd(to_tsvector('simple', v.content_chunk), plainto_tsquery('simple', ?)) AS score FROM knowledge_vectors v JOIN knowledge k ON k.id=v.knowledge_id WHERE k.owner_id=? AND k.id IN ? AND k.status='ready' AND k.deleted_at IS NULL AND to_tsvector('simple', v.content_chunk) @@ plainto_tsquery('simple', ?) ORDER BY score DESC LIMIT ?`, query, ownerID, knowledgeIDs, query, topK).Scan(&rows).Error
	return rows, err
}

// SearchVectorScoped 在指定知识库集合内执行 pgvector 余弦检索。
func (repo *Repository) SearchVectorScoped(ctx context.Context, ownerID string, knowledgeIDs []string, embedding []float32, topK int) ([]SearchRow, error) {
	var rows []SearchRow
	vector := pgvector.NewVector(embedding)
	err := repo.db.WithContext(ctx).Raw(`SELECT v.knowledge_id, v.content_chunk, 1-(v.embedding <=> ?) AS score FROM knowledge_vectors v JOIN knowledge k ON k.id=v.knowledge_id WHERE k.owner_id=? AND k.id IN ? AND k.status='ready' AND k.deleted_at IS NULL AND v.embedding IS NOT NULL ORDER BY v.embedding <=> ? LIMIT ?`, vector, ownerID, knowledgeIDs, vector, topK).Scan(&rows).Error
	return rows, err
}

// IsNotFound 判断仓储错误是否为记录不存在。
func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
