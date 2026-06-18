package dbs

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/juggleim/jugglechat-server-ai/commons/dbcommons"
	"github.com/juggleim/jugglechat-server-ai/storages/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AgentDao struct{}

type AgentTwinDao struct {
	ID             int64      `gorm:"primary_key"`
	AppKey         string     `gorm:"app_key"`
	UniqueName     string     `gorm:"unique_name"`
	BotId          string     `gorm:"bot_id"`
	DisplayName    string     `gorm:"display_name"`
	AvatarURL      string     `gorm:"avatar_url"`
	Greeting       string     `gorm:"greeting"`
	Prompts        string     `gorm:"prompts"`
	OwnerId        string     `gorm:"owner_id"`
	Status         string     `gorm:"status"`
	ActiveVersion  string     `gorm:"active_version"`
	TrainingMode   string     `gorm:"training_mode"`
	MaterialsCount int        `gorm:"materials_count"`
	SyncStatus     string     `gorm:"sync_status"`
	SyncError      string     `gorm:"sync_error"`
	LastSyncedAt   *time.Time `gorm:"last_synced_at"`
	UpdatedTime    time.Time  `gorm:"autoUpdateTime"`
	CreatedTime    time.Time  `gorm:"autoCreateTime"`
}

func (AgentTwinDao) TableName() string { return "agent_twins" }

type AgentTwinTombstoneDao struct {
	ID          int64     `gorm:"primary_key"`
	AppKey      string    `gorm:"app_key"`
	UniqueName  string    `gorm:"unique_name"`
	OwnerId     string    `gorm:"owner_id"`
	CreatedTime time.Time `gorm:"created_time"`
}

func (AgentTwinTombstoneDao) TableName() string { return "agent_twin_tombstones" }

type AgentMaterialDao struct {
	ID           int64      `gorm:"primary_key"`
	AppKey       string     `gorm:"app_key"`
	UniqueName   string     `gorm:"unique_name"`
	MaterialId   string     `gorm:"material_id"`
	Type         string     `gorm:"type"`
	Title        string     `gorm:"title"`
	Source       string     `gorm:"source"`
	Content      string     `gorm:"content"`
	URL          string     `gorm:"url"`
	FilePath     string     `gorm:"file_path"`
	SizeBytes    int64      `gorm:"size_bytes"`
	SyncStatus   string     `gorm:"sync_status"`
	SyncError    string     `gorm:"sync_error"`
	LastSyncedAt *time.Time `gorm:"last_synced_at"`
	UpdatedTime  time.Time  `gorm:"updated_time"`
	CreatedTime  time.Time  `gorm:"created_time"`
}

func (AgentMaterialDao) TableName() string { return "agent_materials" }

type AgentJobDao struct {
	ID             int64           `gorm:"primary_key"`
	AppKey         string          `gorm:"app_key"`
	JobId          string          `gorm:"job_id"`
	AgentJobId     string          `gorm:"agent_job_id"`
	UniqueName     string          `gorm:"unique_name"`
	Type           string          `gorm:"type"`
	Status         string          `gorm:"status"`
	Progress       *int            `gorm:"progress"`
	ResultJSON     json.RawMessage `gorm:"result_json"`
	ErrorCode      string          `gorm:"error_code"`
	ErrorMessage   string          `gorm:"error_message"`
	AgentCreatedAt *time.Time      `gorm:"agent_created_at"`
	StartedAt      *time.Time      `gorm:"started_at"`
	FinishedAt     *time.Time      `gorm:"finished_at"`
	UpdatedTime    time.Time       `gorm:"updated_time"`
	CreatedTime    time.Time       `gorm:"created_time"`
}

func (AgentJobDao) TableName() string { return "agent_jobs" }

type AgentVersionDao struct {
	ID             int64      `gorm:"primary_key"`
	AppKey         string     `gorm:"app_key"`
	UniqueName     string     `gorm:"unique_name"`
	Version        string     `gorm:"version"`
	Mode           string     `gorm:"mode"`
	Active         bool       `gorm:"active"`
	TrainingJobId  string     `gorm:"training_job_id"`
	MaterialsCount int        `gorm:"materials_count"`
	AgentCreatedAt *time.Time `gorm:"agent_created_at"`
	UpdatedTime    time.Time  `gorm:"updated_time"`
	CreatedTime    time.Time  `gorm:"created_time"`
}

func (AgentVersionDao) TableName() string { return "agent_versions" }

type AgentEvaluationDao struct {
	ID             int64           `gorm:"primary_key"`
	AppKey         string          `gorm:"app_key"`
	EvaluationId   string          `gorm:"evaluation_id"`
	UniqueName     string          `gorm:"unique_name"`
	Version        string          `gorm:"version"`
	OverallScore   *float64        `gorm:"overall_score"`
	DimensionsJSON json.RawMessage `gorm:"dimensions_json"`
	SummaryMD      string          `gorm:"summary_md"`
	AgentCreatedAt *time.Time      `gorm:"agent_created_at"`
	UpdatedTime    time.Time       `gorm:"updated_time"`
	CreatedTime    time.Time       `gorm:"created_time"`
}

func (AgentEvaluationDao) TableName() string { return "agent_evaluations" }

type AgentMessageDao struct {
	ID               int64           `gorm:"primary_key"`
	AppKey           string          `gorm:"app_key"`
	UniqueName       string          `gorm:"unique_name"`
	CustomerId       string          `gorm:"customer_id"`
	IMMsgId          string          `gorm:"im_msg_id"`
	AgentMessageId   string          `gorm:"agent_message_id"`
	SessionId        string          `gorm:"session_id"`
	Role             string          `gorm:"role"`
	Text             string          `gorm:"text"`
	Fallback         bool            `gorm:"fallback"`
	SuggestionStatus string          `gorm:"suggestion_status"`
	PendingSource    string          `gorm:"pending_source"`
	Source           string          `gorm:"source"`
	Platform         string          `gorm:"platform"`
	ConverType       int             `gorm:"conver_type"`
	RawPayload       json.RawMessage `gorm:"raw_payload"`
	MsgTime          *time.Time      `gorm:"msg_time"`
	UpdatedTime      time.Time       `gorm:"updated_time"`
	CreatedTime      time.Time       `gorm:"created_time"`
}

func (AgentMessageDao) TableName() string { return "agent_messages" }

func (d *AgentDao) CreateTwin(item models.AgentTwin) error {
	return dbcommons.GetDb().Create(agentTwinToDao(item)).Error
}

func (d *AgentDao) UpdateTwin(item models.AgentTwin) error {
	updates := map[string]interface{}{
		"bot_id":          item.BotId,
		"display_name":    item.DisplayName,
		"avatar_url":      item.AvatarURL,
		"greeting":        item.Greeting,
		"prompts":         item.Prompts,
		"status":          item.Status,
		"active_version":  item.ActiveVersion,
		"training_mode":   item.TrainingMode,
		"materials_count": item.MaterialsCount,
		"sync_status":     item.SyncStatus,
		"sync_error":      item.SyncError,
		"last_synced_at":  milliToTimePtr(item.LastSyncedAt),
	}
	return dbcommons.GetDb().Model(&AgentTwinDao{}).
		Where("app_key=? and unique_name=? and owner_id=?", item.AppKey, item.UniqueName, item.OwnerId).
		Updates(updates).Error
}

func (d *AgentDao) UpdateTwinSyncStatus(appkey, uniqueName, status string) error {
	return dbcommons.GetDb().Model(&AgentTwinDao{}).
		Where("app_key=? and unique_name=?", appkey, uniqueName).
		Updates(map[string]interface{}{
			"sync_status":    status,
			"last_synced_at": time.Now(),
		}).Error
}

func (d *AgentDao) DeleteTwin(appkey, uniqueName string) error {
	return dbcommons.GetDb().Where("app_key=? and unique_name=?", appkey, uniqueName).Delete(&AgentTwinDao{}).Error
}

func (d *AgentDao) FindTwin(appkey, uniqueName string) (*models.AgentTwin, error) {
	var item AgentTwinDao
	err := dbcommons.GetDb().Where("app_key=? and unique_name=?", appkey, uniqueName).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return agentTwinFromDao(item), nil
}

func (d *AgentDao) FindTwinByBotId(appkey, botId string) (*models.AgentTwin, error) {
	var item AgentTwinDao
	err := dbcommons.GetDb().Where("app_key=? and bot_id=?", appkey, botId).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return agentTwinFromDao(item), nil
}

func (d *AgentDao) FindTwinByAnyKey(appkey, key string) (*models.AgentTwin, error) {
	var item AgentTwinDao
	err := dbcommons.GetDb().Where("app_key=? and (unique_name=? or bot_id=?)", appkey, key, key).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return agentTwinFromDao(item), nil
}

func (d *AgentDao) QryTwinsByOwner(appkey, ownerId string, startId, limit int64) ([]*models.AgentTwin, error) {
	db := dbcommons.GetDb().Where("app_key=? and owner_id=?", appkey, ownerId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AgentTwinDao
	err := db.Order("id desc").Limit(normalizeLimit(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentTwin, 0, len(items))
	for _, item := range items {
		ret = append(ret, agentTwinFromDao(item))
	}
	return ret, nil
}

func (d *AgentDao) CreateTwinTombstone(appkey, uniqueName, ownerId string) error {
	return dbcommons.GetDb().Clauses(clause.OnConflict{DoNothing: true}).Create(&AgentTwinTombstoneDao{
		AppKey:     appkey,
		UniqueName: uniqueName,
		OwnerId:    ownerId,
	}).Error
}

func (d *AgentDao) HasTwinTombstone(appkey, uniqueName string) (bool, error) {
	var count int64
	err := dbcommons.GetDb().Model(&AgentTwinTombstoneDao{}).
		Where("app_key=? and unique_name=?", appkey, uniqueName).
		Count(&count).Error
	return count > 0, err
}

func (d *AgentDao) CreateMaterial(item models.AgentMaterial) error {
	return dbcommons.GetDb().Create(agentMaterialToDao(item)).Error
}

func (d *AgentDao) UpdateMaterialSyncStatus(appkey, materialId string, status string) error {
	return dbcommons.GetDb().Model(&AgentMaterialDao{}).
		Where("app_key=? and material_id=?", appkey, materialId).
		Updates(map[string]interface{}{
			"sync_status":    status,
			"last_synced_at": time.Now(),
		}).Error
}

func (d *AgentDao) UpdateMaterialSyncError(appkey, materialId, syncError string) error {
	return dbcommons.GetDb().Model(&AgentMaterialDao{}).
		Where("app_key=? and material_id=?", appkey, materialId).
		Updates(map[string]interface{}{
			"sync_status":    string(models.SyncStatusFailed),
			"sync_error":     syncError,
			"last_synced_at": time.Now(),
		}).Error
}

func (d *AgentDao) DeleteMaterial(appkey, materialId string) error {
	return dbcommons.GetDb().Where("app_key=? and material_id=?", appkey, materialId).Delete(&AgentMaterialDao{}).Error
}

func (d *AgentDao) FindMaterial(appkey, materialId string) (*models.AgentMaterial, error) {
	var item AgentMaterialDao
	err := dbcommons.GetDb().Where("app_key=? and material_id=?", appkey, materialId).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return agentMaterialFromDao(item), nil
}

func (d *AgentDao) QryMaterials(appkey, uniqueName string, startId, limit int64) ([]*models.AgentMaterial, error) {
	db := dbcommons.GetDb().Where("app_key=? and unique_name=?", appkey, uniqueName)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AgentMaterialDao
	err := db.Order("id desc").Limit(normalizeLimit(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentMaterial, 0, len(items))
	for _, item := range items {
		ret = append(ret, agentMaterialFromDao(item))
	}
	return ret, nil
}

func (d *AgentDao) UpsertJob(item models.AgentJob) error {
	now := time.Now()
	dao := agentJobToDao(item)
	dao.UpdatedTime = now
	return dbcommons.GetDb().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "app_key"}, {Name: "job_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"agent_job_id", "unique_name", "type", "status", "progress", "result_json", "error_code", "error_message", "agent_created_at", "started_at", "finished_at", "updated_time"}),
	}).Create(dao).Error
}

func (d *AgentDao) UpdateJobAgentID(appkey, jobId, agentJobId string) error {
	return dbcommons.GetDb().Model(&AgentJobDao{}).
		Where("app_key=? and job_id=?", appkey, jobId).
		Update("agent_job_id", agentJobId).Error
}

func (d *AgentDao) UpdateJobStatus(appkey, jobId, status, msg string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if msg != "" {
		updates["error_message"] = msg
	}
	return dbcommons.GetDb().Model(&AgentJobDao{}).
		Where("app_key=? and job_id=?", appkey, jobId).
		Updates(updates).Error
}

func (d *AgentDao) FindJob(appkey, jobId string) (*models.AgentJob, error) {
	var item AgentJobDao
	err := dbcommons.GetDb().Where("app_key=? and job_id=?", appkey, jobId).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return agentJobFromDao(item), nil
}

func (d *AgentDao) FindJobsByIDs(appkey string, jobIds []string) ([]*models.AgentJob, error) {
	if len(jobIds) == 0 {
		return []*models.AgentJob{}, nil
	}
	var items []AgentJobDao
	err := dbcommons.GetDb().Where("app_key=? and job_id IN ?", appkey, jobIds).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentJob, 0, len(items))
	for _, item := range items {
		ret = append(ret, agentJobFromDao(item))
	}
	return ret, nil
}

func (d *AgentDao) QryJobs(appkey, uniqueName, jobType, status string, startId, limit int64) ([]*models.AgentJob, error) {
	db := dbcommons.GetDb().Where("app_key=? and unique_name=?", appkey, uniqueName)
	if jobType != "" {
		db = db.Where("type=?", jobType)
	}
	if status != "" {
		db = db.Where("status=?", status)
	}
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AgentJobDao
	err := db.Order("id desc").Limit(normalizeLimit(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentJob, 0, len(items))
	for _, item := range items {
		ret = append(ret, agentJobFromDao(item))
	}
	return ret, nil
}

func (d *AgentDao) UpsertVersion(item models.AgentVersion) error {
	return dbcommons.GetDb().Transaction(func(tx *gorm.DB) error {
		if item.Active {
			if err := tx.Model(&AgentVersionDao{}).
				Where("app_key=? and unique_name=?", item.AppKey, item.UniqueName).
				Update("active", false).Error; err != nil {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "app_key"}, {Name: "unique_name"}, {Name: "version"}},
			DoUpdates: clause.AssignmentColumns([]string{"mode", "active", "training_job_id", "materials_count", "agent_created_at"}),
		}).Create(agentVersionToDao(item)).Error
	})
}

func (d *AgentDao) QryVersions(appkey, uniqueName string, startId, limit int64) ([]*models.AgentVersion, error) {
	db := dbcommons.GetDb().Where("app_key=? and unique_name=?", appkey, uniqueName)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AgentVersionDao
	err := db.Order("id desc").Limit(normalizeLimit(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentVersion, 0, len(items))
	for _, item := range items {
		ret = append(ret, agentVersionFromDao(item))
	}
	return ret, nil
}

func (d *AgentDao) FindCurrentVersion(appkey, uniqueName string) (*models.AgentVersion, error) {
	var item AgentVersionDao
	err := dbcommons.GetDb().Where("app_key=? and unique_name=? and active=?", appkey, uniqueName, true).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return agentVersionFromDao(item), nil
}

func (d *AgentDao) SetActiveVersion(appkey, uniqueName, version string) error {
	return dbcommons.GetDb().Transaction(func(tx *gorm.DB) error {
		var item AgentVersionDao
		if err := tx.Where("app_key=? and unique_name=? and version=?", appkey, uniqueName, version).Take(&item).Error; err != nil {
			return err
		}
		if err := tx.Model(&AgentVersionDao{}).Where("app_key=? and unique_name=?", appkey, uniqueName).Update("active", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&AgentVersionDao{}).Where("id=?", item.ID).Update("active", true).Error; err != nil {
			return err
		}
		return tx.Model(&AgentTwinDao{}).Where("app_key=? and unique_name=?", appkey, uniqueName).Updates(map[string]interface{}{
			"active_version": version,
			"training_mode":  item.Mode,
			"status":         "trained",
		}).Error
	})
}

func (d *AgentDao) UpsertEvaluation(item models.AgentEvaluation) error {
	return dbcommons.GetDb().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "app_key"}, {Name: "evaluation_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"unique_name", "version", "overall_score", "dimensions_json", "summary_md", "agent_created_at"}),
	}).Create(agentEvaluationToDao(item)).Error
}

func (d *AgentDao) FindEvaluation(appkey, evaluationId string) (*models.AgentEvaluation, error) {
	var item AgentEvaluationDao
	err := dbcommons.GetDb().Where("app_key=? and evaluation_id=?", appkey, evaluationId).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return agentEvaluationFromDao(item), nil
}

func (d *AgentDao) CreateMessage(item models.AgentMessage) error {
	return dbcommons.GetDb().Create(agentMessageToDao(item)).Error
}

func (d *AgentDao) FindMessageByIM(appkey, imMsgId, role string) (*models.AgentMessage, error) {
	var item AgentMessageDao
	err := dbcommons.GetDb().Where("app_key=? and im_msg_id=? and role=?", appkey, imMsgId, role).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return agentMessageFromDao(item), nil
}

func (d *AgentDao) QryMessages(appkey, uniqueName, customerId string, startId, limit int64) ([]*models.AgentMessage, error) {
	db := dbcommons.GetDb().Where("app_key=? and unique_name=? and customer_id=?", appkey, uniqueName, customerId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AgentMessageDao
	err := db.Order("id desc").Limit(normalizeLimit(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentMessage, 0, len(items))
	for _, item := range items {
		ret = append(ret, agentMessageFromDao(item))
	}
	return ret, nil
}

func (d *AgentDao) QryMessagesByIM(appkey, imMsgId string) ([]*models.AgentMessage, error) {
	var items []AgentMessageDao
	err := dbcommons.GetDb().Where("app_key=? and im_msg_id=?", appkey, imMsgId).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentMessage, 0, len(items))
	for _, item := range items {
		ret = append(ret, agentMessageFromDao(item))
	}
	return ret, nil
}

func normalizeLimit(limit int64) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return int(limit)
}

func milliToTimePtr(ms int64) *time.Time {
	if ms <= 0 {
		return nil
	}
	t := time.UnixMilli(ms)
	return &t
}

func orTimePtr(a *time.Time, b *time.Time) *time.Time {
	if a != nil {
		return a
	}
	return b
}

func timePtrToMilli(t *time.Time) int64 {
	if t == nil || t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func jsonStringToData(s string) json.RawMessage {
	if s == "" {
		return nil
	}
	return json.RawMessage(s)
}

func jsonDataToString(v json.RawMessage) string {
	if len(v) == 0 {
		return ""
	}
	return string(v)
}

func agentTwinToDao(item models.AgentTwin) *AgentTwinDao {
	syncStatus := item.SyncStatus
	if syncStatus == "" {
		syncStatus = string(models.SyncStatusPending)
	}
	return &AgentTwinDao{
		AppKey:         item.AppKey,
		UniqueName:     item.UniqueName,
		BotId:          item.BotId,
		DisplayName:    item.DisplayName,
		AvatarURL:      item.AvatarURL,
		Greeting:       item.Greeting,
		Prompts:        item.Prompts,
		OwnerId:        item.OwnerId,
		Status:         item.Status,
		ActiveVersion:  item.ActiveVersion,
		TrainingMode:   item.TrainingMode,
		MaterialsCount: item.MaterialsCount,
		SyncStatus:     syncStatus,
		SyncError:      item.SyncError,
		LastSyncedAt:   milliToTimePtr(item.LastSyncedAt),
	}
}

func agentTwinFromDao(item AgentTwinDao) *models.AgentTwin {
	return &models.AgentTwin{
		ID:             item.ID,
		AppKey:         item.AppKey,
		UniqueName:     item.UniqueName,
		BotId:          item.BotId,
		DisplayName:    item.DisplayName,
		AvatarURL:      item.AvatarURL,
		Greeting:       item.Greeting,
		Prompts:        item.Prompts,
		OwnerId:        item.OwnerId,
		Status:         item.Status,
		ActiveVersion:  item.ActiveVersion,
		TrainingMode:   item.TrainingMode,
		MaterialsCount: item.MaterialsCount,
		SyncStatus:     item.SyncStatus,
		SyncError:      item.SyncError,
		LastSyncedAt:   timePtrToMilli(item.LastSyncedAt),
		UpdatedTime:    item.UpdatedTime.UnixMilli(),
		CreatedTime:    item.CreatedTime.UnixMilli(),
	}
}

func agentMaterialToDao(item models.AgentMaterial) *AgentMaterialDao {
	syncStatus := item.SyncStatus
	if syncStatus == "" {
		syncStatus = string(models.SyncStatusPending)
	}
	now := time.Now()
	return &AgentMaterialDao{
		AppKey:       item.AppKey,
		UniqueName:   item.UniqueName,
		MaterialId:   item.MaterialId,
		Type:         item.Type,
		Title:        item.Title,
		Source:       item.Source,
		Content:      item.Content,
		URL:          item.URL,
		FilePath:     item.FilePath,
		SizeBytes:    item.SizeBytes,
		SyncStatus:   syncStatus,
		SyncError:    item.SyncError,
		LastSyncedAt: milliToTimePtr(item.LastSyncedAt),
		CreatedTime:  now,
		UpdatedTime:  now,
	}
}

func agentMaterialFromDao(item AgentMaterialDao) *models.AgentMaterial {
	return &models.AgentMaterial{
		ID:           item.ID,
		AppKey:       item.AppKey,
		UniqueName:   item.UniqueName,
		MaterialId:   item.MaterialId,
		Type:         item.Type,
		Title:        item.Title,
		Source:       item.Source,
		Content:      item.Content,
		URL:          item.URL,
		FilePath:     item.FilePath,
		SizeBytes:    item.SizeBytes,
		SyncStatus:   item.SyncStatus,
		SyncError:    item.SyncError,
		LastSyncedAt: timePtrToMilli(item.LastSyncedAt),
		UpdatedTime:  item.UpdatedTime.UnixMilli(),
		CreatedTime:  item.CreatedTime.UnixMilli(),
	}
}

func agentJobToDao(item models.AgentJob) *AgentJobDao {
	now := time.Now()
	return &AgentJobDao{
		AppKey:         item.AppKey,
		JobId:          item.JobId,
		AgentJobId:     item.AgentJobId,
		UniqueName:     item.UniqueName,
		Type:           item.Type,
		Status:         item.Status,
		Progress:       item.Progress,
		ResultJSON:     jsonStringToData(item.ResultJSON),
		ErrorCode:      item.ErrorCode,
		ErrorMessage:   item.ErrorMessage,
		AgentCreatedAt: milliToTimePtr(item.AgentCreatedAt),
		StartedAt:      milliToTimePtr(item.StartedAt),
		FinishedAt:     milliToTimePtr(item.FinishedAt),
		CreatedTime:    now,
		UpdatedTime:    now,
	}
}

func agentJobFromDao(item AgentJobDao) *models.AgentJob {
	return &models.AgentJob{
		ID:             item.ID,
		AppKey:         item.AppKey,
		JobId:          item.JobId,
		AgentJobId:     item.AgentJobId,
		UniqueName:     item.UniqueName,
		Type:           item.Type,
		Status:         item.Status,
		Progress:       item.Progress,
		ResultJSON:     jsonDataToString(item.ResultJSON),
		ErrorCode:      item.ErrorCode,
		ErrorMessage:   item.ErrorMessage,
		AgentCreatedAt: timePtrToMilli(item.AgentCreatedAt),
		StartedAt:      timePtrToMilli(item.StartedAt),
		FinishedAt:     timePtrToMilli(item.FinishedAt),
		UpdatedTime:    item.UpdatedTime.UnixMilli(),
		CreatedTime:    item.CreatedTime.UnixMilli(),
	}
}

func agentVersionToDao(item models.AgentVersion) *AgentVersionDao {
	return &AgentVersionDao{
		AppKey:         item.AppKey,
		UniqueName:     item.UniqueName,
		Version:        item.Version,
		Mode:           item.Mode,
		Active:         item.Active,
		TrainingJobId:  item.TrainingJobId,
		MaterialsCount: item.MaterialsCount,
		AgentCreatedAt: milliToTimePtr(item.AgentCreatedAt),
	}
}

func agentVersionFromDao(item AgentVersionDao) *models.AgentVersion {
	return &models.AgentVersion{
		ID:             item.ID,
		AppKey:         item.AppKey,
		UniqueName:     item.UniqueName,
		Version:        item.Version,
		Mode:           item.Mode,
		Active:         item.Active,
		TrainingJobId:  item.TrainingJobId,
		MaterialsCount: item.MaterialsCount,
		AgentCreatedAt: timePtrToMilli(item.AgentCreatedAt),
		UpdatedTime:    item.UpdatedTime.UnixMilli(),
		CreatedTime:    item.CreatedTime.UnixMilli(),
	}
}

func agentEvaluationToDao(item models.AgentEvaluation) *AgentEvaluationDao {
	return &AgentEvaluationDao{
		AppKey:         item.AppKey,
		EvaluationId:   item.EvaluationId,
		UniqueName:     item.UniqueName,
		Version:        item.Version,
		OverallScore:   item.OverallScore,
		DimensionsJSON: jsonStringToData(item.DimensionsJSON),
		SummaryMD:      item.SummaryMD,
		AgentCreatedAt: milliToTimePtr(item.AgentCreatedAt),
	}
}

func agentEvaluationFromDao(item AgentEvaluationDao) *models.AgentEvaluation {
	return &models.AgentEvaluation{
		ID:             item.ID,
		AppKey:         item.AppKey,
		EvaluationId:   item.EvaluationId,
		UniqueName:     item.UniqueName,
		Version:        item.Version,
		OverallScore:   item.OverallScore,
		DimensionsJSON: jsonDataToString(item.DimensionsJSON),
		SummaryMD:      item.SummaryMD,
		AgentCreatedAt: timePtrToMilli(item.AgentCreatedAt),
		UpdatedTime:    item.UpdatedTime.UnixMilli(),
		CreatedTime:    item.CreatedTime.UnixMilli(),
	}
}

func agentMessageToDao(item models.AgentMessage) *AgentMessageDao {
	now := time.Now()
	return &AgentMessageDao{
		AppKey:           item.AppKey,
		UniqueName:       item.UniqueName,
		CustomerId:       item.CustomerId,
		IMMsgId:          item.IMMsgId,
		AgentMessageId:   item.AgentMessageId,
		SessionId:        item.SessionId,
		Role:             item.Role,
		Text:             item.Text,
		Fallback:         item.Fallback,
		SuggestionStatus: item.SuggestionStatus,
		PendingSource:    item.PendingSource,
		Source:           item.Source,
		Platform:         item.Platform,
		ConverType:       item.ConverType,
		RawPayload:       jsonStringToData(item.RawPayload),
		MsgTime:          milliToTimePtr(item.MsgTime),
		CreatedTime:      now,
		UpdatedTime:      now,
	}
}

func agentMessageFromDao(item AgentMessageDao) *models.AgentMessage {
	return &models.AgentMessage{
		ID:               item.ID,
		AppKey:           item.AppKey,
		UniqueName:       item.UniqueName,
		CustomerId:       item.CustomerId,
		IMMsgId:          item.IMMsgId,
		AgentMessageId:   item.AgentMessageId,
		SessionId:        item.SessionId,
		Role:             item.Role,
		Text:             item.Text,
		Fallback:         item.Fallback,
		SuggestionStatus: item.SuggestionStatus,
		PendingSource:    item.PendingSource,
		Source:           item.Source,
		Platform:         item.Platform,
		ConverType:       item.ConverType,
		RawPayload:       jsonDataToString(item.RawPayload),
		MsgTime:          timePtrToMilli(item.MsgTime),
		UpdatedTime:      item.UpdatedTime.UnixMilli(),
		CreatedTime:      item.CreatedTime.UnixMilli(),
	}
}

func (d *AgentDao) QryMessagesBySession(appkey, sessionId string, startId, limit int64) ([]*models.AgentMessage, error) {
	db := dbcommons.GetDb().Where("app_key=? and session_id=?", appkey, sessionId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AgentMessageDao
	err := db.Order("id asc").Limit(normalizeLimit(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentMessage, 0, len(items))
	for _, item := range items {
		ret = append(ret, agentMessageFromDao(item))
	}
	return ret, nil
}

func (d *AgentDao) UpdateMessageSuggestionStatus(appkey string, id int64, status string) error {
	return dbcommons.GetDb().Model(&AgentMessageDao{}).
		Where("app_key=? and id=?", appkey, id).
		Update("suggestion_status", status).Error
}

// ========== AgentSession ==========

type AgentSessionDao struct {
	ID             int64           `gorm:"primary_key"`
	AppKey         string          `gorm:"app_key"`
	SessionId      string          `gorm:"session_id"`
	UniqueName     string          `gorm:"unique_name"`
	CustomerId     string          `gorm:"customer_id"`
	Platform       string          `gorm:"platform"`
	PlatformConvId string          `gorm:"platform_conv_id"`
	OperatorId     string          `gorm:"operator_id"`
	Status         int             `gorm:"status"`
	AutoMode       int             `gorm:"auto_mode"`
	MsgCount       int             `gorm:"msg_count"`
	Tags           json.RawMessage `gorm:"tags"`
	Summary        string          `gorm:"summary"`
	FirstMsgAt     *time.Time      `gorm:"first_msg_at"`
	LastMsgAt      *time.Time      `gorm:"last_msg_at"`
	ClosedAt       *time.Time      `gorm:"closed_at"`
	UpdatedTime    time.Time       `gorm:"updated_time"`
	CreatedTime    time.Time       `gorm:"created_time"`
}

func (AgentSessionDao) TableName() string { return "agent_sessions" }

func (d *AgentDao) CreateSession(item models.AgentSession) error {
	now := time.Now()
	return dbcommons.GetDb().Create(&AgentSessionDao{
		AppKey:         item.AppKey,
		SessionId:      item.SessionId,
		UniqueName:     item.UniqueName,
		CustomerId:     item.CustomerId,
		Platform:       item.Platform,
		PlatformConvId: item.PlatformConvId,
		OperatorId:     item.OperatorId,
		Status:         item.Status,
		AutoMode:       item.AutoMode,
		MsgCount:       item.MsgCount,
		Tags:           json.RawMessage(item.Tags),
		Summary:        item.Summary,
		FirstMsgAt:     orTimePtr(milliToTimePtr(item.FirstMsgAt), &now),
		LastMsgAt:      orTimePtr(milliToTimePtr(item.LastMsgAt), &now),
		ClosedAt:       milliToTimePtr(item.ClosedAt),
		CreatedTime:    now,
		UpdatedTime:    now,
	}).Error
}

func (d *AgentDao) UpdateSession(item models.AgentSession) error {
	return dbcommons.GetDb().Model(&AgentSessionDao{}).
		Where("app_key=? and session_id=?", item.AppKey, item.SessionId).
		Updates(map[string]interface{}{
			"operator_id": item.OperatorId,
			"unique_name": item.UniqueName,
			"status":      item.Status,
			"auto_mode":   item.AutoMode,
			"msg_count":   item.MsgCount,
			"summary":     item.Summary,
			"last_msg_at": milliToTimePtr(item.LastMsgAt),
			"closed_at":   milliToTimePtr(item.ClosedAt),
		}).Error
}

// UpdateSessionLastMsgAt only updates the last message timestamp, leaving
// all other fields (unique_name, auto_mode, etc.) untouched. This prevents
// the GORM Updates(map) zero-value pitfall where an empty unique_name would
// silently clear the agent binding.
func (d *AgentDao) UpdateSessionLastMsgAt(appkey, sessionId string, lastMsgAt int64) error {
	return dbcommons.GetDb().Model(&AgentSessionDao{}).
		Where("app_key=? and session_id=?", appkey, sessionId).
		Update("last_msg_at", milliToTimePtr(lastMsgAt)).Error
}

func (d *AgentDao) FindSession(appkey, sessionId string) (*models.AgentSession, error) {
	var item AgentSessionDao
	err := dbcommons.GetDb().Where("app_key=? and session_id=?", appkey, sessionId).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return &models.AgentSession{
		ID:             item.ID,
		AppKey:         item.AppKey,
		SessionId:      item.SessionId,
		UniqueName:     item.UniqueName,
		CustomerId:     item.CustomerId,
		Platform:       item.Platform,
		PlatformConvId: item.PlatformConvId,
		OperatorId:     item.OperatorId,
		Status:         item.Status,
		AutoMode:       item.AutoMode,
		MsgCount:       item.MsgCount,
		Tags:           string(item.Tags),
		Summary:        item.Summary,
		FirstMsgAt:     timePtrToMilli(item.FirstMsgAt),
		LastMsgAt:      timePtrToMilli(item.LastMsgAt),
		ClosedAt:       timePtrToMilli(item.ClosedAt),
		UpdatedTime:    item.UpdatedTime.UnixMilli(),
		CreatedTime:    item.CreatedTime.UnixMilli(),
	}, nil
}

func (d *AgentDao) FindSessionByCustomerAgent(appkey, customerId, uniqueName string, status int) (*models.AgentSession, error) {
	var item AgentSessionDao
	err := dbcommons.GetDb().
		Where("app_key=? and customer_id=? and unique_name=? and status=?", appkey, customerId, uniqueName, status).
		Order("id desc").
		Take(&item).Error
	if err != nil {
		return nil, err
	}
	return &models.AgentSession{
		ID:             item.ID,
		AppKey:         item.AppKey,
		SessionId:      item.SessionId,
		UniqueName:     item.UniqueName,
		CustomerId:     item.CustomerId,
		Platform:       item.Platform,
		PlatformConvId: item.PlatformConvId,
		OperatorId:     item.OperatorId,
		Status:         item.Status,
		AutoMode:       item.AutoMode,
		MsgCount:       item.MsgCount,
		Tags:           string(item.Tags),
		Summary:        item.Summary,
		FirstMsgAt:     timePtrToMilli(item.FirstMsgAt),
		LastMsgAt:      timePtrToMilli(item.LastMsgAt),
		ClosedAt:       timePtrToMilli(item.ClosedAt),
		UpdatedTime:    item.UpdatedTime.UnixMilli(),
		CreatedTime:    item.CreatedTime.UnixMilli(),
	}, nil
}

// FindSessionByConv 按 IM 会话维度查找 session（platform_conv_id = receiver/conversationId）
func (d *AgentDao) FindSessionByConv(appkey, platformConvId, uniqueName string) (*models.AgentSession, error) {
	var item AgentSessionDao
	err := dbcommons.GetDb().
		Where("app_key=? and platform_conv_id=? and unique_name=? and status<>2", appkey, platformConvId, uniqueName).
		Order("id desc").
		Take(&item).Error
	if err != nil {
		return nil, err
	}
	return &models.AgentSession{
		ID:             item.ID,
		AppKey:         item.AppKey,
		SessionId:      item.SessionId,
		UniqueName:     item.UniqueName,
		CustomerId:     item.CustomerId,
		Platform:       item.Platform,
		PlatformConvId: item.PlatformConvId,
		OperatorId:     item.OperatorId,
		Status:         item.Status,
		AutoMode:       item.AutoMode,
		MsgCount:       item.MsgCount,
		Tags:           string(item.Tags),
		Summary:        item.Summary,
		FirstMsgAt:     timePtrToMilli(item.FirstMsgAt),
		LastMsgAt:      timePtrToMilli(item.LastMsgAt),
		ClosedAt:       timePtrToMilli(item.ClosedAt),
		UpdatedTime:    item.UpdatedTime.UnixMilli(),
		CreatedTime:    item.CreatedTime.UnixMilli(),
	}, nil
}

// FindSessionByConvId 按 conv_id 查找任意 agent 的 session（不限制 unique_name，用于前端回显）
func (d *AgentDao) FindSessionByConvId(appkey, platformConvId string) (*models.AgentSession, error) {
	var item AgentSessionDao
	err := dbcommons.GetDb().
		Where("app_key=? and platform_conv_id=? and status<>2", appkey, platformConvId).
		Order("id desc").
		Take(&item).Error
	if err != nil {
		return nil, err
	}
	return &models.AgentSession{
		ID:             item.ID,
		AppKey:         item.AppKey,
		SessionId:      item.SessionId,
		UniqueName:     item.UniqueName,
		CustomerId:     item.CustomerId,
		Platform:       item.Platform,
		PlatformConvId: item.PlatformConvId,
		OperatorId:     item.OperatorId,
		Status:         item.Status,
		AutoMode:       item.AutoMode,
		MsgCount:       item.MsgCount,
		Tags:           string(item.Tags),
		Summary:        item.Summary,
		FirstMsgAt:     timePtrToMilli(item.FirstMsgAt),
		LastMsgAt:      timePtrToMilli(item.LastMsgAt),
		ClosedAt:       timePtrToMilli(item.ClosedAt),
		UpdatedTime:    item.UpdatedTime.UnixMilli(),
		CreatedTime:    item.CreatedTime.UnixMilli(),
	}, nil
}

func (d *AgentDao) QrySessionsByAgent(appkey, uniqueName string, startId, limit int64) ([]*models.AgentSession, error) {
	db := dbcommons.GetDb().Where("app_key=? and unique_name=?", appkey, uniqueName)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AgentSessionDao
	err := db.Order("id desc").Limit(normalizeLimit(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentSession, 0, len(items))
	for _, item := range items {
		ret = append(ret, &models.AgentSession{
			ID:             item.ID,
			AppKey:         item.AppKey,
			SessionId:      item.SessionId,
			UniqueName:     item.UniqueName,
			CustomerId:     item.CustomerId,
			Platform:       item.Platform,
			PlatformConvId: item.PlatformConvId,
			OperatorId:     item.OperatorId,
			Status:         item.Status,
			AutoMode:       item.AutoMode,
			MsgCount:       item.MsgCount,
			Tags:           string(item.Tags),
			Summary:        item.Summary,
			FirstMsgAt:     timePtrToMilli(item.FirstMsgAt),
			LastMsgAt:      timePtrToMilli(item.LastMsgAt),
			ClosedAt:       timePtrToMilli(item.ClosedAt),
			UpdatedTime:    item.UpdatedTime.UnixMilli(),
			CreatedTime:    item.CreatedTime.UnixMilli(),
		})
	}
	return ret, nil
}

func (d *AgentDao) QrySessionsByOperator(appkey, operatorId string, status int, startId, limit int64) ([]*models.AgentSession, error) {
	db := dbcommons.GetDb().Where("app_key=? and operator_id=?", appkey, operatorId)
	if status >= 0 {
		db = db.Where("status=?", status)
	}
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AgentSessionDao
	err := db.Order("id desc").Limit(normalizeLimit(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentSession, 0, len(items))
	for _, item := range items {
		ret = append(ret, &models.AgentSession{
			ID:             item.ID,
			AppKey:         item.AppKey,
			SessionId:      item.SessionId,
			UniqueName:     item.UniqueName,
			CustomerId:     item.CustomerId,
			Platform:       item.Platform,
			PlatformConvId: item.PlatformConvId,
			OperatorId:     item.OperatorId,
			Status:         item.Status,
			AutoMode:       item.AutoMode,
			MsgCount:       item.MsgCount,
			Tags:           string(item.Tags),
			Summary:        item.Summary,
			FirstMsgAt:     timePtrToMilli(item.FirstMsgAt),
			LastMsgAt:      timePtrToMilli(item.LastMsgAt),
			ClosedAt:       timePtrToMilli(item.ClosedAt),
			UpdatedTime:    item.UpdatedTime.UnixMilli(),
			CreatedTime:    item.CreatedTime.UnixMilli(),
		})
	}
	return ret, nil
}

// ========== AgentFeedback ==========

type AgentFeedbackDao struct {
	ID             int64     `gorm:"primary_key"`
	AppKey         string    `gorm:"app_key"`
	FeedbackId     string    `gorm:"feedback_id"`
	SessionId      string    `gorm:"session_id"`
	UniqueName     string    `gorm:"unique_name"`
	AgentMsgId     string    `gorm:"agent_msg_id"`
	AgentReplyText string    `gorm:"agent_reply_text"`
	Action         string    `gorm:"action"`
	FinalReplyText string    `gorm:"final_reply_text"`
	EditDiff       string    `gorm:"edit_diff"`
	RejectReason   string    `gorm:"reject_reason"`
	OperatorId     string    `gorm:"operator_id"`
	CreatedTime    time.Time `gorm:"created_time"`
}

func (AgentFeedbackDao) TableName() string { return "agent_feedbacks" }

func (d *AgentDao) CreateFeedback(item models.AgentFeedback) error {
	return dbcommons.GetDb().Create(&AgentFeedbackDao{
		AppKey:         item.AppKey,
		FeedbackId:     item.FeedbackId,
		SessionId:      item.SessionId,
		UniqueName:     item.UniqueName,
		AgentMsgId:     item.AgentMsgId,
		AgentReplyText: item.AgentReplyText,
		Action:         item.Action,
		FinalReplyText: item.FinalReplyText,
		EditDiff:       item.EditDiff,
		RejectReason:   item.RejectReason,
		OperatorId:     item.OperatorId,
	}).Error
}

func (d *AgentDao) QryFeedbacksByAgent(appkey, uniqueName string, startId, limit int64) ([]*models.AgentFeedback, error) {
	db := dbcommons.GetDb().Where("app_key=? and unique_name=?", appkey, uniqueName)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AgentFeedbackDao
	err := db.Order("id desc").Limit(normalizeLimit(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentFeedback, 0, len(items))
	for _, item := range items {
		ret = append(ret, &models.AgentFeedback{
			ID:             item.ID,
			AppKey:         item.AppKey,
			FeedbackId:     item.FeedbackId,
			SessionId:      item.SessionId,
			UniqueName:     item.UniqueName,
			AgentMsgId:     item.AgentMsgId,
			AgentReplyText: item.AgentReplyText,
			Action:         item.Action,
			FinalReplyText: item.FinalReplyText,
			EditDiff:       item.EditDiff,
			RejectReason:   item.RejectReason,
			OperatorId:     item.OperatorId,
			CreatedTime:    item.CreatedTime.UnixMilli(),
		})
	}
	return ret, nil
}

func (d *AgentDao) QryFeedbacksBySession(appkey, sessionId string, startId, limit int64) ([]*models.AgentFeedback, error) {
	db := dbcommons.GetDb().Where("app_key=? and session_id=?", appkey, sessionId)
	if startId > 0 {
		db = db.Where("id<?", startId)
	}
	var items []AgentFeedbackDao
	err := db.Order("id desc").Limit(normalizeLimit(limit)).Find(&items).Error
	if err != nil {
		return nil, err
	}
	ret := make([]*models.AgentFeedback, 0, len(items))
	for _, item := range items {
		ret = append(ret, &models.AgentFeedback{
			ID:             item.ID,
			AppKey:         item.AppKey,
			FeedbackId:     item.FeedbackId,
			SessionId:      item.SessionId,
			UniqueName:     item.UniqueName,
			AgentMsgId:     item.AgentMsgId,
			AgentReplyText: item.AgentReplyText,
			Action:         item.Action,
			FinalReplyText: item.FinalReplyText,
			EditDiff:       item.EditDiff,
			RejectReason:   item.RejectReason,
			OperatorId:     item.OperatorId,
			CreatedTime:    item.CreatedTime.UnixMilli(),
		})
	}
	return ret, nil
}

var _ models.AgentStorage = (*AgentDao)(nil)

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
