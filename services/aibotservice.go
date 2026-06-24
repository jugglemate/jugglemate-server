package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/agentclient"
	"github.com/juggleim/jugglemate-server/commons/agentconfig"
	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/commons/oss"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
)

var (
	ossClient     *oss.Client
	ossClientErr  error
	ossClientOnce sync.Once
)

// getOssClient returns the shared OSS upload client (lazy init).
func getOssClient() (*oss.Client, error) {
	ossClientOnce.Do(func() {
		ossClient, ossClientErr = oss.New(oss.Config{
			Endpoint:  configures.Config.Oss.Endpoint,
			AccessKey: configures.Config.Oss.AccessKey,
			SecretKey: configures.Config.Oss.SecretKey,
			Bucket:    configures.Config.Oss.Bucket,
		})
	})
	return ossClient, ossClientErr
}

func newAgentClient() *agentclient.Client {
	return agentclient.New(agentclient.Config{
		BaseURL:    strings.TrimRight(agentconfig.BaseURL(), "/"),
		Timeout:    agentconfig.Timeout(),
		TwinsToken: agentconfig.TwinsToken(),
	})
}

func CreateAiBot(ctx context.Context, bot *apiModels.AiBotInfo) (errs.IMErrorCode, *apiModels.AiBotInfo) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	botId := tools.GenerateUUIDShort11()
	uniqueName := normalizeUniqueName(bot.UniqueName, botId)
	displayName := normalizeDisplayName(bot)
	avatarURL := normalizeAvatarURL(bot)

	log.Printf("[CreateAiBot] params: appkey=%s userId=%s uniqueName=%s displayName=%s greeting=%q prompts_len=%d",
		appkey, userId, uniqueName, displayName, bot.Greeting, len(bot.Prompts))

	storage := storages.NewAgentStorage()
	_, err := storage.HasTwinTombstone(appkey, uniqueName)
	if err != nil {
		log.Printf("[CreateAiBot] HasTwinTombstone failed: err=%v", err)
		return errs.IMErrorCode_APP_AIBOT_AddBotFailed, nil
	}
	if _, err := storage.FindTwin(appkey, uniqueName); err == nil {
		log.Printf("[CreateAiBot] FindTwin found existing agent: appkey=%s uniqueName=%s", appkey, uniqueName)
		return errs.IMErrorCode_APP_AIBOT_AddBotFailed, nil
	}

	// Step 1: call agent API first — if agent service is down, don't create ghost records
	agentCli := newAgentClient()
	createReq := map[string]any{
		"unique_name":  uniqueName,
		"display_name": displayName,
		"avatar_url":   avatarURL,
	}
	_, syncCode, syncErr := agentCli.CreateTwin(ctx, userId, createReq)
	if syncErr != nil || (syncCode != 200 && syncCode != 201) {
		syncErrMsg := fmt.Sprintf("agent CreateTwin failed: code=%d err=%v", syncCode, syncErr)
		log.Printf("[CreateAiBot] %s", syncErrMsg)
		return errs.IMErrorCode_APP_AIBOT_AddBotFailed, nil
	}

	// Step 2: agent API succeeded, persist to local DB
	err = storage.CreateTwin(storageModels.AgentTwin{
		AppKey:         appkey,
		UniqueName:     uniqueName,
		BotId:          botId,
		DisplayName:    displayName,
		AvatarURL:      avatarURL,
		Greeting:       bot.Greeting,
		Prompts:        bot.Prompts,
		OwnerId:        userId,
		Status:         "untrained",
		SyncStatus:     string(storageModels.SyncStatusSynced),
		MaterialsCount: 0,
	})
	if err != nil {
		log.Printf("[CreateAiBot] CreateTwin failed: err=%v", err)
		return errs.IMErrorCode_APP_AIBOT_AddBotFailed, nil
	}
	registerBotToIM(appkey, botId, displayName, avatarURL)

	return errs.IMErrorCode_SUCCESS, &apiModels.AiBotInfo{
		BotId:       botId,
		UniqueName:  uniqueName,
		Nickname:    displayName,
		DisplayName: displayName,
		Avatar:      avatarURL,
		AvatarURL:   avatarURL,
		Prompts:     bot.Prompts,
		Greeting:    bot.Greeting,
		Status:      "untrained",
		SyncStatus:  string(storageModels.SyncStatusSynced),
		OwnerId:     userId,
	}
}

func UpdateAiBot(ctx context.Context, bot *apiModels.UpdateAiBotReq) (errs.IMErrorCode, *apiModels.AiBotInfo) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()

	oldBot, err := storage.FindTwinByBotId(appkey, bot.BotId)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if oldBot.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}

	displayName := oldBot.DisplayName
	if bot.DisplayName != nil {
		displayName = *bot.DisplayName
	} else if bot.Nickname != nil {
		displayName = *bot.Nickname
	}
	avatarURL := oldBot.AvatarURL
	if bot.AvatarURL != nil {
		avatarURL = *bot.AvatarURL
	} else if bot.Avatar != nil {
		avatarURL = *bot.Avatar
	}
	greeting := oldBot.Greeting
	if bot.Greeting != nil {
		greeting = *bot.Greeting
	}
	prompts := oldBot.Prompts
	if bot.Prompts != nil {
		prompts = *bot.Prompts
	}

	// Step 1: call agent API first
	agentCli := newAgentClient()
	updateReq := map[string]any{
		"display_name": displayName,
		"avatar_url":   avatarURL,
	}
	_, syncCode, syncErr := agentCli.UpdateTwin(ctx, userId, oldBot.UniqueName, updateReq)
	if syncErr != nil || (syncCode != 200 && syncCode != 201) {
		syncErrMsg := fmt.Sprintf("agent UpdateTwin failed: code=%d err=%v", syncCode, syncErr)
		log.Printf("[UpdateAiBot] %s", syncErrMsg)
		return errs.IMErrorCode_APP_AIBOT_UpdateBotFailed, nil
	}

	// Step 2: agent API succeeded, persist to local DB
	err = storage.UpdateTwin(storageModels.AgentTwin{
		AppKey:         appkey,
		UniqueName:     oldBot.UniqueName,
		BotId:          oldBot.BotId,
		DisplayName:    displayName,
		AvatarURL:      avatarURL,
		Greeting:       greeting,
		Prompts:        prompts,
		Status:         oldBot.Status,
		ActiveVersion:  oldBot.ActiveVersion,
		TrainingMode:   oldBot.TrainingMode,
		MaterialsCount: oldBot.MaterialsCount,
		SyncStatus:     string(storageModels.SyncStatusSynced),
		OwnerId:        oldBot.OwnerId,
	})
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_UpdateBotFailed, nil
	}
	current, err := storage.FindTwin(appkey, oldBot.UniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_UpdateBotFailed, nil
	}
	registerBotToIM(appkey, oldBot.BotId, displayName, avatarURL)

	return errs.IMErrorCode_SUCCESS, twinToAPI(current, current.DisplayName, current.AvatarURL, current.Greeting)
}

func RemoveAiBot(ctx context.Context, botId string) errs.IMErrorCode {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()

	oldBot, err := storage.FindTwinByBotId(appkey, botId)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound
	}
	if oldBot.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission
	}

	// Try to delete from agent service first (best-effort)
	agentCli := newAgentClient()
	delCode, delErr := agentCli.DeleteTwin(ctx, userId, oldBot.UniqueName)
	if delErr != nil || (delCode != 200 && delCode != 204) {
		log.Printf("[RemoveAiBot] agent delete failed: code=%d err=%v, uniqueName=%s", delCode, delErr, oldBot.UniqueName)
	}

	if err := storage.CreateTwinTombstone(appkey, oldBot.UniqueName, oldBot.OwnerId); err != nil {
		return errs.IMErrorCode_APP_AIBOT_DelBotFailed
	}
	if err := storage.DeleteTwin(appkey, oldBot.UniqueName); err != nil {
		return errs.IMErrorCode_APP_AIBOT_DelBotFailed
	}
	return errs.IMErrorCode_SUCCESS
}

func QryMyAiBots(ctx context.Context, limit int64, offset string) (errs.IMErrorCode, *apiModels.AiBotInfos) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()

	startId := decodeOffset(offset)
	if limit <= 0 {
		limit = 20
	}

	ret := &apiModels.AiBotInfos{
		Items: []*apiModels.AiBotInfo{},
	}
	items, err := storage.QryTwinsByOwner(appkey, userId, startId, limit)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
	}

	for _, item := range items {
		ret.Offset, _ = tools.EncodeInt(item.ID)
		ret.Items = append(ret.Items, agentTwinToAPI(item))
	}
	return errs.IMErrorCode_SUCCESS, ret
}

func AddAiMaterial(ctx context.Context, uniqueName string, req *apiModels.AiMaterialInfo) (errs.IMErrorCode, *apiModels.AiMaterialInfo) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()
	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}
	materialId := "mat_" + tools.GenerateUUIDShort11()
	addReq := buildTextOrURLMaterialRequest(materialId, req)
	if addReq == nil {
		return errs.IMErrorCode_APP_ParamError, nil
	}
	source := req.Source
	if source == "" {
		source = req.Type
	}
	sizeBytes := req.SizeBytes
	if sizeBytes <= 0 {
		switch req.Type {
		case "url":
			sizeBytes = int64(len(req.Url))
		default:
			sizeBytes = int64(len(req.Content))
		}
	}
	item := storageModels.AgentMaterial{
		AppKey:       appkey,
		UniqueName:   uniqueName,
		MaterialId:   materialId,
		Type:         req.Type,
		Title:        req.Title,
		Source:       source,
		Content:      req.Content,
		URL:          req.Url,
		FilePath:     req.FilePath,
		SizeBytes:    sizeBytes,
		SyncStatus:   string(storageModels.SyncStatusPending),
		LastSyncedAt: 0,
	}
	if err := storage.CreateMaterial(item); err != nil {
		return errs.IMErrorCode_APP_AIBOT_AddMaterialFailed, nil
	}

	agentCli := newAgentClient()
	_, syncCode, syncErr := agentCli.AddMaterial(ctx, userId, uniqueName, addReq)
	if syncErr != nil || (syncCode != 200 && syncCode != 201) {
		syncErrMsg := fmt.Sprintf("code=%d err=%v", syncCode, syncErr)
		log.Printf("[AddAiMaterial] agent sync failed: %s, material=%s", syncErrMsg, materialId)
		_ = storage.UpdateMaterialSyncError(appkey, materialId, syncErrMsg)
		item.SyncStatus = string(storageModels.SyncStatusFailed)
		item.SyncError = syncErrMsg
	} else {
		_ = storage.UpdateMaterialSyncStatus(appkey, materialId, string(storageModels.SyncStatusSynced))
		item.SyncStatus = string(storageModels.SyncStatusSynced)
		item.SyncError = ""
	}

	twin.MaterialsCount++
	_ = storage.UpdateTwin(*twin)
	return errs.IMErrorCode_SUCCESS, materialToAPI(&item)
}

func StartTraining(ctx context.Context, uniqueName, mode string) (errs.IMErrorCode, *apiModels.AiBotInfo, string) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()
	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil, ""
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil, ""
	}
	if mode == "" {
		mode = "mind"
	}
	switch mode {
	case "mind", "memory", "skill":
		// valid modes
	default:
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil, ""
	}
	jobID := "job_" + tools.GenerateUUIDShort11()
	progress := 0
	if err := storage.UpsertJob(storageModels.AgentJob{
		AppKey:     appkey,
		JobId:      jobID,
		UniqueName: uniqueName,
		Type:       "training",
		Status:     "queued",
		Progress:   &progress,
	}); err != nil {
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil, ""
	}
	version := "v1"
	versions, err := storage.QryVersions(appkey, uniqueName, 0, 100)
	if err != nil {
		log.Printf("QryVersions failed: %v", err)
	}
	for _, v := range versions {
		if strings.HasPrefix(v.Version, "v") {
			if num := extractVersionSeq(v.Version); num >= extractVersionSeq(version) {
				version = fmt.Sprintf("v%d", num+1)
			}
		}
	}
	_ = storage.UpsertVersion(storageModels.AgentVersion{
		AppKey:         appkey,
		UniqueName:     uniqueName,
		Version:        version,
		Mode:           mode,
		Active:         false,
		TrainingJobId:  jobID,
		MaterialsCount: twin.MaterialsCount,
	})
	_ = storage.UpdateTwin(storageModels.AgentTwin{
		AppKey:         appkey,
		UniqueName:     uniqueName,
		BotId:          twin.BotId,
		DisplayName:    twin.DisplayName,
		AvatarURL:      twin.AvatarURL,
		Greeting:       twin.Greeting,
		Prompts:        twin.Prompts,
		OwnerId:        twin.OwnerId,
		Status:         "training",
		ActiveVersion:  twin.ActiveVersion,
		TrainingMode:   mode,
		MaterialsCount: twin.MaterialsCount,
		SyncStatus:     string(storageModels.SyncStatusPending),
	})

	// Call agent service to actually start training
	agentCli := newAgentClient()
	agentJob, code, agentErr := agentCli.StartTraining(ctx, userId, uniqueName, map[string]any{"mode": mode})
	if agentErr != nil || (code != 200 && code != 202) {
		errMsg := fmt.Sprintf("agent StartTraining failed: code=%d err=%v", code, agentErr)
		log.Printf("[StartTraining] %s", errMsg)
		_ = storage.UpdateJobStatus(appkey, jobID, "failed", errMsg)
		current, _ := storage.FindTwin(appkey, uniqueName)
		if current != nil {
			return errs.IMErrorCode_APP_AIBOT_DEFAULT, twinToAPI(current, current.DisplayName, current.AvatarURL, current.Greeting), jobID
		}
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil, jobID
	}

	// Write back agent's real job_id and update status
	_ = storage.UpdateJobAgentID(appkey, jobID, agentJob.JobID)
	_ = storage.UpdateJobStatus(appkey, jobID, agentJob.Status, "")

	// Start background polling to sync job status from agent service
	go backgroundPollJob(appkey, uniqueName, jobID, agentJob.JobID, userId, 10*time.Minute)

	current, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_SUCCESS, twinToAPI(twin, twin.DisplayName, twin.AvatarURL, twin.Greeting), jobID
	}
	return errs.IMErrorCode_SUCCESS, twinToAPI(current, current.DisplayName, current.AvatarURL, current.Greeting), jobID
}

func ListJobs(ctx context.Context, uniqueName, status, jobType string, limit int64, offset string) (errs.IMErrorCode, *apiModels.AiBotInfos) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()
	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}
	items, err := storage.QryJobs(appkey, uniqueName, jobType, status, decodeOffset(offset), limit)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
	}
	ret := &apiModels.AiBotInfos{Items: []*apiModels.AiBotInfo{}}
	for _, item := range items {
		ret.Offset, _ = tools.EncodeInt(item.ID)
		ret.Items = append(ret.Items, &apiModels.AiBotInfo{
			BotId:       item.JobId,
			UniqueName:  item.UniqueName,
			Nickname:    item.Type,
			DisplayName: item.Status,
			Greeting:    item.ResultJSON,
			OwnerId:     twin.OwnerId,
			CreatedTime: item.CreatedTime,
			UpdatedTime: item.UpdatedTime,
		})
	}
	return errs.IMErrorCode_SUCCESS, ret
}

func ListVersions(ctx context.Context, uniqueName string, limit int64, offset string) (errs.IMErrorCode, *apiModels.AiBotInfos) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()
	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}
	items, err := storage.QryVersions(appkey, uniqueName, decodeOffset(offset), limit)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
	}
	ret := &apiModels.AiBotInfos{Items: []*apiModels.AiBotInfo{}}
	for _, item := range items {
		ret.Offset, _ = tools.EncodeInt(item.ID)
		ret.Items = append(ret.Items, &apiModels.AiBotInfo{
			BotId:          item.Version,
			UniqueName:     item.UniqueName,
			Nickname:       item.Mode,
			DisplayName:    item.Mode,
			ActiveVersion:  item.TrainingJobId,
			TrainingMode:   item.Mode,
			MaterialsCount: item.MaterialsCount,
			CreatedTime:    item.CreatedTime,
			UpdatedTime:    item.UpdatedTime,
		})
	}
	return errs.IMErrorCode_SUCCESS, ret
}

func ActivateVersion(ctx context.Context, uniqueName, version string) (errs.IMErrorCode, *apiModels.AiBotInfo) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()
	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}
	if err := storage.SetActiveVersion(appkey, uniqueName, version); err != nil {
		return errs.IMErrorCode_APP_AIBOT_UpdateBotFailed, nil
	}

	// Also activate on agent service side
	agentCli := newAgentClient()
	_, code, agentErr := agentCli.ActivateVersion(ctx, userId, uniqueName, version)
	if agentErr != nil || (code != 200 && code != 201) {
		log.Printf("[ActivateVersion] agent activate failed: uniqueName=%s version=%s code=%d err=%v",
			uniqueName, version, code, agentErr)
		return errs.IMErrorCode_APP_AIBOT_UpdateBotFailed, nil
	}

	current, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
	}
	return errs.IMErrorCode_SUCCESS, twinToAPI(current, current.DisplayName, current.AvatarURL, current.Greeting)
}

func GetCurrentVersion(ctx context.Context, uniqueName string) (errs.IMErrorCode, *apiModels.AiBotInfo) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()
	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}
	current, err := storage.FindCurrentVersion(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	return errs.IMErrorCode_SUCCESS, &apiModels.AiBotInfo{
		BotId:          current.Version,
		UniqueName:     current.UniqueName,
		Nickname:       current.Mode,
		DisplayName:    current.Mode,
		ActiveVersion:  current.TrainingJobId,
		TrainingMode:   current.Mode,
		MaterialsCount: current.MaterialsCount,
	}
}

func ListEvaluations(ctx context.Context, uniqueName string, limit int64, offset string) (errs.IMErrorCode, *apiModels.AiBotInfos) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()
	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}
	// TODO: 评估功能待实现，当前返回空列表占位
	return errs.IMErrorCode_SUCCESS, &apiModels.AiBotInfos{Items: []*apiModels.AiBotInfo{}}
}

func QryAiMaterials(ctx context.Context, uniqueName string, limit int64, offset string) (errs.IMErrorCode, *apiModels.AiMaterialInfos) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()
	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}
	items, err := storage.QryMaterials(appkey, uniqueName, decodeOffset(offset), limit)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
	}
	ret := &apiModels.AiMaterialInfos{Items: []*apiModels.AiMaterialInfo{}}
	for _, item := range items {
		ret.Offset, _ = tools.EncodeInt(item.ID)
		ret.Items = append(ret.Items, materialToAPI(item))
	}
	return errs.IMErrorCode_SUCCESS, ret
}

func RemoveAiMaterial(ctx context.Context, uniqueName, materialId string) errs.IMErrorCode {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()
	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission
	}
	material, err := storage.FindMaterial(appkey, materialId)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_DelMaterialFailed
	}
	if material.UniqueName != uniqueName {
		return errs.IMErrorCode_APP_AIBOT_DelMaterialFailed
	}

	agentCli := newAgentClient()
	delCode, delErr := agentCli.DeleteMaterial(ctx, userId, uniqueName, materialId)
	if delErr != nil || (delCode != 200 && delCode != 204 && delCode != 404) {
		log.Printf("[RemoveAiMaterial] agent delete failed: code=%d err=%v, uniqueName=%s materialId=%s", delCode, delErr, uniqueName, materialId)
		return errs.IMErrorCode_APP_AIBOT_DelMaterialFailed
	}

	if err := storage.DeleteMaterial(appkey, materialId); err != nil {
		return errs.IMErrorCode_APP_AIBOT_DelMaterialFailed
	}
	if twin.MaterialsCount > 0 {
		twin.MaterialsCount--
		_ = storage.UpdateTwin(*twin)
	}
	return errs.IMErrorCode_SUCCESS
}

// SyncTwin retries syncing a twin to the agent service.
func SyncTwin(ctx context.Context, uniqueName string) (errs.IMErrorCode, *apiModels.AiBotInfo) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()

	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}

	agentCli := newAgentClient()
	createReq := map[string]any{
		"unique_name":  twin.UniqueName,
		"display_name": twin.DisplayName,
		"avatar_url":   twin.AvatarURL,
	}
	_, syncCode, syncErr := agentCli.CreateTwin(ctx, userId, createReq)
	if syncErr != nil || (syncCode != 200 && syncCode != 201) {
		// Try update as fallback (twin may already exist on agent side)
		updateReq := map[string]any{
			"display_name": twin.DisplayName,
			"avatar_url":   twin.AvatarURL,
		}
		_, syncCode, syncErr = agentCli.UpdateTwin(ctx, userId, uniqueName, updateReq)
	}
	if syncErr != nil || (syncCode != 200 && syncCode != 201) {
		syncErrMsg := fmt.Sprintf("code=%d err=%v", syncCode, syncErr)
		log.Printf("[SyncTwin] sync failed: %s, uniqueName=%s", syncErrMsg, uniqueName)
		twin.SyncStatus = string(storageModels.SyncStatusFailed)
		twin.SyncError = syncErrMsg
		_ = storage.UpdateTwin(*twin)
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, agentTwinToAPI(twin)
	}
	_ = storage.UpdateTwinSyncStatus(appkey, uniqueName, string(storageModels.SyncStatusSynced))
	twin.SyncStatus = string(storageModels.SyncStatusSynced)
	twin.SyncError = ""
	return errs.IMErrorCode_SUCCESS, agentTwinToAPI(twin)
}

// SyncMaterial retries syncing a single material to the agent service.
func SyncMaterial(ctx context.Context, uniqueName, materialId string) (errs.IMErrorCode, *apiModels.AiMaterialInfo) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()

	item, err := storage.FindMaterial(appkey, materialId)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if item.UniqueName != uniqueName {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil || twin == nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}

	agentCli := newAgentClient()
	syncCode := 0
	var syncErr error
	if item.Type == "file" || item.Type == "image" {
		syncFile, openErr := os.Open(item.FilePath)
		if openErr != nil {
			syncErr = openErr
		} else {
			defer syncFile.Close()
			_, syncCode, syncErr = agentCli.AddMaterialFile(ctx, userId, uniqueName, filepath.Base(item.FilePath), syncFile, item.Title, map[string]string{
				"id": item.MaterialId,
			})
		}
	} else {
		addReq := buildTextOrURLMaterialRequest(item.MaterialId, &apiModels.AiMaterialInfo{
			Type:    item.Type,
			Title:   item.Title,
			Source:  item.Source,
			Content: item.Content,
			Url:     item.URL,
		})
		if addReq == nil {
			syncErr = fmt.Errorf("invalid material type: %s", item.Type)
		} else {
			_, syncCode, syncErr = agentCli.AddMaterial(ctx, userId, uniqueName, addReq)
		}
	}
	if syncErr != nil || (syncCode != 200 && syncCode != 201) {
		syncErrMsg := fmt.Sprintf("code=%d err=%v", syncCode, syncErr)
		log.Printf("[SyncMaterial] sync failed: %s, material=%s", syncErrMsg, materialId)
		item.SyncStatus = string(storageModels.SyncStatusFailed)
		item.SyncError = syncErrMsg
		_ = storage.UpdateMaterialSyncError(appkey, materialId, syncErrMsg)
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, materialToAPI(item)
	}
	_ = storage.UpdateMaterialSyncStatus(appkey, materialId, string(storageModels.SyncStatusSynced))
	item.SyncStatus = string(storageModels.SyncStatusSynced)
	item.SyncError = ""
	return errs.IMErrorCode_SUCCESS, materialToAPI(item)
}

// normalizeUniqueName 生成符合 agent API 规则的 unique_name（[a-z][a-z0-9-]{2,31}）。
func normalizeUniqueName(uniqueName, botId string) string {
	sanitize := func(raw string) string {
		var buf strings.Builder
		for _, r := range strings.ToLower(raw) {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				buf.WriteRune(r)
			}
		}
		return buf.String()
	}

	from := botId
	if uniqueName != "" {
		from = uniqueName
	}
	name := sanitize(from)

	// 确保有效长度
	if len(name) < 3 {
		padding := strings.Repeat("0", 3-len(name))
		name = "a" + name + padding
	}
	if len(name) > 32 {
		name = name[:32]
		name = strings.TrimRight(name, "-")
	}
	// 必须以小写字母开头
	if name[0] < 'a' || name[0] > 'z' {
		name = "a" + name
		if len(name) > 32 {
			name = name[:32]
		}
	}
	// 以 - 结尾时补 0
	name = strings.TrimRight(name, "-")
	if len(name) < 3 {
		name += strings.Repeat("0", 3-len(name))
	}
	return name
}

func normalizeDisplayName(bot *apiModels.AiBotInfo) string {
	if bot.DisplayName != "" {
		return bot.DisplayName
	}
	return bot.Nickname
}

func normalizeAvatarURL(bot *apiModels.AiBotInfo) string {
	if bot.AvatarURL != "" {
		return bot.AvatarURL
	}
	return bot.Avatar
}

func decodeOffset(offset string) int64 {
	if offset == "" {
		return 0
	}
	intVal, err := tools.DecodeInt(offset)
	if err != nil {
		return 0
	}
	return intVal
}

func agentTwinToAPI(item *storageModels.AgentTwin) *apiModels.AiBotInfo {
	return &apiModels.AiBotInfo{
		BotId:          item.BotId,
		UniqueName:     item.UniqueName,
		Nickname:       item.DisplayName,
		DisplayName:    item.DisplayName,
		Avatar:         item.AvatarURL,
		AvatarURL:      item.AvatarURL,
		Prompts:        item.Prompts,
		Greeting:       item.Greeting,
		OwnerId:        item.OwnerId,
		Status:         item.Status,
		ActiveVersion:  item.ActiveVersion,
		TrainingMode:   item.TrainingMode,
		MaterialsCount: item.MaterialsCount,
		SyncStatus:     item.SyncStatus,
		SyncError:      item.SyncError,
		CreatedTime:    item.CreatedTime,
		UpdatedTime:    item.UpdatedTime,
	}
}

func twinToAPI(item *storageModels.AgentTwin, displayName, avatarURL, greeting string) *apiModels.AiBotInfo {
	ret := agentTwinToAPI(item)
	ret.Nickname = displayName
	ret.DisplayName = displayName
	ret.Avatar = avatarURL
	ret.AvatarURL = avatarURL
	ret.Greeting = greeting
	return ret
}

func materialToAPI(item *storageModels.AgentMaterial) *apiModels.AiMaterialInfo {
	return &apiModels.AiMaterialInfo{
		MaterialId: item.MaterialId,
		Type:       item.Type,
		Title:      item.Title,
		Source:     item.Source,
		Content:    item.Content,
		Url:        item.URL,
		FilePath:   item.FilePath,
		FileName:   extractFileName(item.FilePath),
		SizeBytes:  item.SizeBytes,
		SyncStatus: item.SyncStatus,
		SyncError:  item.SyncError,
		CreatedAt:  time.UnixMilli(item.CreatedTime).Format(time.RFC3339),
	}
}

func buildTextOrURLMaterialRequest(materialId string, req *apiModels.AiMaterialInfo) map[string]any {
	if req == nil {
		return nil
	}
	base := map[string]any{}
	if materialId != "" {
		base["id"] = materialId
	}
	if req.Title != "" {
		base["title"] = req.Title
	}
	if req.Source != "" {
		base["source"] = req.Source
	}
	switch req.Type {
	case "text":
		if req.Content == "" {
			return nil
		}
		base["type"] = "text"
		base["content"] = req.Content
		return base
	case "url":
		if req.Url == "" {
			return nil
		}
		base["type"] = "url"
		base["url"] = req.Url
		return base
	default:
		return nil
	}
}

func extractFileName(path string) string {
	if path == "" {
		return ""
	}
	idx := strings.LastIndex(path, "/")
	if idx >= 0 {
		return path[idx+1:]
	}
	return path
}

func extractVersionSeq(v string) int {
	if len(v) <= 1 || !strings.HasPrefix(v, "v") {
		return 0
	}
	return tools.ToInt(v[1:])
}

// QueryJobStatus looks up a local job and syncs its status from the agent service.
// When the job is terminal (succeeded/failed), it also updates twin status and syncs versions.
func QueryJobStatus(ctx context.Context, uniqueName, jobId string) (errs.IMErrorCode, *apiModels.AiBotInfo) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()

	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}

	job, err := storage.FindJob(appkey, jobId)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}

	// If we have an agent-side job ID, sync latest status from agent
	if job.AgentJobId != "" {
		agentCli := newAgentClient()
		agentJob, code, agentErr := agentCli.GetJob(ctx, userId, job.AgentJobId)
		if agentErr == nil && (code == 200) && agentJob != nil {
			// Parse time fields from agent response
			resultJSON := tools.ToJson(agentJob.Result)
			agentCreatedAt := parseTimeToMilli(agentJob.CreatedAt)
			startedAt := parseTimeToMilli(agentJob.StartedAt)
			finishedAt := parseTimeToMilli(agentJob.FinishedAt)

			_ = storage.UpsertJob(storageModels.AgentJob{
				AppKey:         appkey,
				JobId:          jobId,
				AgentJobId:     job.AgentJobId,
				UniqueName:     uniqueName,
				Type:           "training",
				Status:         agentJob.Status,
				Progress:       agentJob.Progress,
				ResultJSON:     resultJSON,
				AgentCreatedAt: agentCreatedAt,
				StartedAt:      startedAt,
				FinishedAt:     finishedAt,
			})
			job.Status = agentJob.Status
			job.ResultJSON = resultJSON
			if agentJob.Progress != nil {
				job.Progress = agentJob.Progress
			}

			// On terminal state, update twin and sync versions (same as backgroundPollJob)
			switch agentJob.Status {
			case "succeeded":
				log.Printf("[QueryJobStatus] job succeeded, updating twin and syncing versions: uniqueName=%s jobId=%s", uniqueName, jobId)
				twin.Status = "trained"
				_ = storage.UpdateTwin(*twin)

				versions, vCode, vErr := agentCli.ListVersions(ctx, userId, uniqueName)
				if vErr == nil && vCode == 200 {
					for _, v := range versions {
						_ = storage.UpsertVersion(storageModels.AgentVersion{
							AppKey:         appkey,
							UniqueName:     uniqueName,
							Version:        v.Version,
							Mode:           v.Mode,
							Active:         v.Active,
							TrainingJobId:  v.TrainingJobID,
							MaterialsCount: v.MaterialsCount,
							AgentCreatedAt: parseTimeToMilli(v.CreatedAt),
						})
					}
					log.Printf("[QueryJobStatus] synced %d versions from agent", len(versions))
				} else {
					log.Printf("[QueryJobStatus] ListVersions failed: code=%d err=%v", vCode, vErr)
				}
			case "failed":
				log.Printf("[QueryJobStatus] job failed: uniqueName=%s jobId=%s err=%v", uniqueName, jobId, agentJob.Error)
				twin.Status = "training_failed"
				_ = storage.UpdateTwin(*twin)
			}
		}
	}

	return errs.IMErrorCode_SUCCESS, &apiModels.AiBotInfo{
		BotId:       job.JobId,
		UniqueName:  job.UniqueName,
		Nickname:    job.Type,
		DisplayName: job.Status,
		Greeting:    job.ResultJSON,
		Prompts:     job.ErrorMessage,
		AvatarURL:   job.AgentJobId,
		CreatedTime: job.CreatedTime,
		UpdatedTime: job.UpdatedTime,
	}
}

func registerBotToIM(appkey, botId, nickname, avatar string) {
	sdk := imsdk.GetImSdk(appkey)
	if sdk != nil {
		sdk.RegisterBot(juggleimsdk.BotInfo{
			BotId:    botId,
			Nickname: nickname,
			Portrait: avatar,

			BotConf: &juggleimsdk.BotConf{
				Url: "http://127.0.0.1:8050/botmsgs/msgcallback",
			},
		})
	}
}

// backgroundPollJob periodically syncs job status from agent service to local DB.
// On job completion, it also updates the twin status and syncs version data.
func backgroundPollJob(appkey, uniqueName, localJobID, agentJobID, userId string, timeout time.Duration) {
	if agentJobID == "" {
		return
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				log.Printf("[backgroundPollJob] timeout: localJobID=%s agentJobID=%s", localJobID, agentJobID)
				storage := storages.NewAgentStorage()
				_ = storage.UpdateJobStatus(appkey, localJobID, "failed", "polling timeout")
				// Update twin status to failed
				if twin, findErr := storage.FindTwin(appkey, uniqueName); findErr == nil {
					twin.Status = "training_failed"
					_ = storage.UpdateTwin(*twin)
				}
				return
			}

			agentCli := newAgentClient()
			job, code, err := agentCli.GetJob(context.Background(), userId, agentJobID)
			if err != nil || code != 200 || job == nil {
				log.Printf("[backgroundPollJob] GetJob failed: localJobID=%s agentJobID=%s code=%d err=%v", localJobID, agentJobID, code, err)
				continue
			}

			// Parse time fields from agent response
			resultJSON := tools.ToJson(job.Result)
			agentCreatedAt := parseTimeToMilli(job.CreatedAt)
			startedAt := parseTimeToMilli(job.StartedAt)
			finishedAt := parseTimeToMilli(job.FinishedAt)

			storage := storages.NewAgentStorage()
			_ = storage.UpsertJob(storageModels.AgentJob{
				AppKey:         appkey,
				JobId:          localJobID,
				AgentJobId:     agentJobID,
				UniqueName:     uniqueName,
				Type:           "training",
				Status:         job.Status,
				Progress:       job.Progress,
				ResultJSON:     resultJSON,
				AgentCreatedAt: agentCreatedAt,
				StartedAt:      startedAt,
				FinishedAt:     finishedAt,
			})

			if job.Status == "succeeded" {
				log.Printf("[backgroundPollJob] succeeded: localJobID=%s agentJobID=%s", localJobID, agentJobID)

				// Update twin status to trained
				twin, findErr := storage.FindTwin(appkey, uniqueName)
				if findErr == nil {
					twin.Status = "trained"
					_ = storage.UpdateTwin(*twin)
				}

				// Sync versions from agent service
				versions, vCode, vErr := agentCli.ListVersions(context.Background(), userId, uniqueName)
				if vErr == nil && vCode == 200 {
					for _, v := range versions {
						_ = storage.UpsertVersion(storageModels.AgentVersion{
							AppKey:         appkey,
							UniqueName:     uniqueName,
							Version:        v.Version,
							Mode:           v.Mode,
							Active:         v.Active,
							TrainingJobId:  v.TrainingJobID,
							MaterialsCount: v.MaterialsCount,
							AgentCreatedAt: parseTimeToMilli(v.CreatedAt),
						})
					}
					log.Printf("[backgroundPollJob] synced %d versions from agent", len(versions))
				} else {
					log.Printf("[backgroundPollJob] ListVersions failed: code=%d err=%v", vCode, vErr)
				}
				return
			}

			if job.Status == "failed" {
				log.Printf("[backgroundPollJob] failed: localJobID=%s agentJobID=%s err=%v", localJobID, agentJobID, job.Error)
				twin, findErr := storage.FindTwin(appkey, uniqueName)
				if findErr == nil {
					twin.Status = "training_failed"
					_ = storage.UpdateTwin(*twin)
				}
				return
			}
		}
	}
}

// parseTimeToMilli parses an ISO 8601 / RFC3339 time string to Unix milliseconds.
func parseTimeToMilli(s string) int64 {
	if s == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

// BatchQueryJobStatusRequest is the request body for batch querying job statuses.
type BatchQueryJobStatusRequest struct {
	JobIds []string `json:"job_ids"`
}

// BatchQueryJobStatus queries multiple local jobs and syncs non-terminal ones from agent service.
func BatchQueryJobStatus(ctx context.Context, req *BatchQueryJobStatusRequest) (errs.IMErrorCode, []map[string]interface{}) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()

	if len(req.JobIds) == 0 {
		return errs.IMErrorCode_SUCCESS, []map[string]interface{}{}
	}

	jobs, err := storage.FindJobsByIDs(appkey, req.JobIds)
	if err != nil {
		log.Printf("[BatchQueryJobStatus] FindJobsByIDs failed: err=%v", err)
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
	}

	agentCli := newAgentClient()
	results := make([]map[string]interface{}, 0, len(jobs))

	for _, job := range jobs {
		// If job has an agent-side ID and is not terminal, sync from agent
		if job.AgentJobId != "" && job.Status != "succeeded" && job.Status != "failed" {
			agentJob, code, agentErr := agentCli.GetJob(ctx, userId, job.AgentJobId)
			if agentErr == nil && code == 200 && agentJob != nil {
				resultJSON := tools.ToJson(agentJob.Result)
				_ = storage.UpsertJob(storageModels.AgentJob{
					AppKey:     appkey,
					JobId:      job.JobId,
					AgentJobId: job.AgentJobId,
					Type:       "training",
					Status:     agentJob.Status,
					Progress:   agentJob.Progress,
					ResultJSON: resultJSON,
				})
				job.Status = agentJob.Status
				job.ResultJSON = resultJSON
			}
		}

		results = append(results, map[string]interface{}{
			"job_id":      job.JobId,
			"unique_name": job.UniqueName,
			"type":        job.Type,
			"status":      job.Status,
			"progress":    job.Progress,
			"result":      job.ResultJSON,
			"error_msg":   job.ErrorMessage,
		})
	}

	return errs.IMErrorCode_SUCCESS, results
}

// UploadAiMaterial handles file upload for agent training materials.
// The file is saved locally for training and also uploaded to Qiniu CDN for public access.
func UploadAiMaterial(ctx context.Context, uniqueName string, file multipart.File, header *multipart.FileHeader, title, source string) (errs.IMErrorCode, *apiModels.AiMaterialInfo) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	userId := ctxs.GetRequesterIdFromCtx(ctx)
	storage := storages.NewAgentStorage()

	twin, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_APP_AIBOT_BotNotFound, nil
	}
	if twin.OwnerId != userId {
		return errs.IMErrorCode_APP_AIBOT_NoPermission, nil
	}

	// Determine material type from MIME
	contentType := header.Header.Get("Content-Type")
	materialType := "file"
	if strings.HasPrefix(contentType, "image/") {
		materialType = "image"
	}

	// Default source to type if not provided
	if source == "" {
		source = materialType
	}

	materialId := "mat_" + tools.GenerateUUIDShort11()

	// Build storage path: data/materials/{appkey}/{unique_name}/{materialId}_{filename}
	dir := filepath.Join("data", "materials", appkey, uniqueName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[UploadAiMaterial] MkdirAll failed: path=%s err=%v", dir, err)
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
	}
	savedFilename := materialId + "_" + header.Filename
	savePath := filepath.Join(dir, savedFilename)

	dst, err := os.Create(savePath)
	if err != nil {
		log.Printf("[UploadAiMaterial] Create file failed: path=%s err=%v", savePath, err)
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		log.Printf("[UploadAiMaterial] Copy file failed: err=%v", err)
		os.Remove(savePath)
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
	}
	dst.Close()

	// Upload to OSS CDN for public access (read from saved file)
	cdnURL := ""
	uploadFile, err := os.Open(savePath)
	if err != nil {
		log.Printf("[UploadAiMaterial] failed to reopen for oss upload: path=%s err=%v", savePath, err)
	} else {
		defer uploadFile.Close()
		cdnPrefix := fmt.Sprintf("materials/%s/%s/", appkey, uniqueName)
		ossCli, ossErr := getOssClient()
		if ossErr != nil {
			log.Printf("[UploadAiMaterial] oss client init failed: err=%v", ossErr)
		} else {
			result, uploadErr := ossCli.Upload(ctx, uploadFile, cdnPrefix, savedFilename)
			if uploadErr != nil {
				log.Printf("[UploadAiMaterial] oss upload failed (file saved locally): err=%v", uploadErr)
			} else {
				cdnURL = result.URL
			}
		}
	}

	item := storageModels.AgentMaterial{
		AppKey:       appkey,
		UniqueName:   uniqueName,
		MaterialId:   materialId,
		Type:         materialType,
		Title:        title,
		Source:       source,
		Content:      "",
		URL:          cdnURL,
		FilePath:     savePath,
		SizeBytes:    written,
		SyncStatus:   string(storageModels.SyncStatusPending),
		LastSyncedAt: 0,
	}
	if err := storage.CreateMaterial(item); err != nil {
		os.Remove(savePath)
		return errs.IMErrorCode_APP_AIBOT_AddMaterialFailed, nil
	}

	agentCli := newAgentClient()
	syncFile, err := os.Open(savePath)
	if err != nil {
		syncErrMsg := fmt.Sprintf("open saved file failed: %v", err)
		log.Printf("[UploadAiMaterial] %s, material=%s", syncErrMsg, materialId)
		item.SyncStatus = string(storageModels.SyncStatusFailed)
		item.SyncError = syncErrMsg
		_ = storage.UpdateMaterialSyncError(appkey, materialId, syncErrMsg)
		twin.MaterialsCount++
		_ = storage.UpdateTwin(*twin)
		return errs.IMErrorCode_SUCCESS, materialToAPI(&item)
	}
	defer syncFile.Close()

	// Sync material to agent service so it's available for training
	_, syncCode, syncErr := agentCli.AddMaterialFile(ctx, userId, uniqueName, header.Filename, syncFile, title, map[string]string{
		"id": materialId,
	})
	if syncErr != nil || (syncCode != 200 && syncCode != 201) {
		syncErrMsg := fmt.Sprintf("code=%d err=%v", syncCode, syncErr)
		log.Printf("[UploadAiMaterial] agent sync failed: %s, material=%s", syncErrMsg, materialId)
		item.SyncStatus = string(storageModels.SyncStatusFailed)
		item.SyncError = syncErrMsg
		_ = storage.UpdateMaterialSyncError(appkey, materialId, syncErrMsg)
	} else {
		item.SyncStatus = string(storageModels.SyncStatusSynced)
		item.SyncError = ""
		storage.UpdateMaterialSyncStatus(appkey, materialId, item.SyncStatus)
	}

	twin.MaterialsCount++
	_ = storage.UpdateTwin(*twin)

	return errs.IMErrorCode_SUCCESS, materialToAPI(&item)
}

// UploadAvatar handles avatar image upload to OSS CDN and returns the public CDN URL.
func UploadAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader) (errs.IMErrorCode, string) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		log.Printf("[UploadAvatar] invalid content type: %s", contentType)
		return errs.IMErrorCode_APP_ParamError, ""
	}

	// Generate unique filename for CDN key
	ext := filepath.Ext(header.Filename)
	savedFilename := fmt.Sprintf("%d_%s%s", time.Now().UnixMilli(), tools.GenerateUUIDShort11(), ext)

	// Read full file bytes into memory, then upload to OSS
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("[UploadAvatar] ReadAll failed: appkey=%s filename=%s err=%v", appkey, savedFilename, err)
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, ""
	}

	ossCli, ossErr := getOssClient()
	if ossErr != nil {
		log.Printf("[UploadAvatar] oss client init failed: err=%v", ossErr)
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, ""
	}

	result, uploadErr := ossCli.Upload(
		ctx, bytes.NewReader(fileBytes),
		"avatars/"+appkey+"/",
		savedFilename,
	)
	if uploadErr != nil {
		log.Printf("[UploadAvatar] oss upload failed: appkey=%s filename=%s err=%v", appkey, savedFilename, uploadErr)
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, ""
	}

	log.Printf("[UploadAvatar] success: appkey=%s cdnURL=%s", appkey, result.URL)
	return errs.IMErrorCode_SUCCESS, result.URL
}
