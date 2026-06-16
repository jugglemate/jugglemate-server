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

	apiModels "github.com/juggleim/jugglechat-server-ai/apis/models"
	"github.com/juggleim/jugglechat-server-ai/commons/configures"
	"github.com/juggleim/jugglechat-server-ai/commons/ctxs"
	"github.com/juggleim/jugglechat-server-ai/commons/errs"
	"github.com/juggleim/jugglechat-server-ai/commons/imsdk"
	"github.com/juggleim/jugglechat-server-ai/commons/oss"
	"github.com/juggleim/jugglechat-server-ai/commons/tools"
	"github.com/juggleim/jugglechat-server-ai/storages"
	storageModels "github.com/juggleim/jugglechat-server-ai/storages/models"

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
		SyncStatus:     string(storageModels.SyncStatusPending),
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
		SyncStatus:     string(storageModels.SyncStatusPending),
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
	source := req.Source
	if source == "" {
		source = req.Type
	}
	sizeBytes := req.SizeBytes
	if sizeBytes <= 0 {
		sizeBytes = int64(len(req.Content))
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
	twin.MaterialsCount++
	_ = storage.UpdateTwin(*twin)
	return errs.IMErrorCode_SUCCESS, materialToAPI(&item)
}

func StartTraining(ctx context.Context, uniqueName, mode string) (errs.IMErrorCode, *apiModels.AiBotInfo) {
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
	if mode == "" {
		mode = "mind"
	}
	switch mode {
	case "mind", "memory", "skill":
		// valid modes
	default:
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
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
		return errs.IMErrorCode_APP_AIBOT_DEFAULT, nil
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
	current, err := storage.FindTwin(appkey, uniqueName)
	if err != nil {
		return errs.IMErrorCode_SUCCESS, twinToAPI(twin, twin.DisplayName, twin.AvatarURL, twin.Greeting)
	}
	return errs.IMErrorCode_SUCCESS, twinToAPI(current, current.DisplayName, current.AvatarURL, current.Greeting)
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
	if err := storage.DeleteMaterial(appkey, materialId); err != nil {
		return errs.IMErrorCode_APP_AIBOT_DelMaterialFailed
	}
	if twin.MaterialsCount > 0 {
		twin.MaterialsCount--
		_ = storage.UpdateTwin(*twin)
	}
	return errs.IMErrorCode_SUCCESS
}

func normalizeUniqueName(uniqueName, botId string) string {
	if uniqueName != "" {
		return uniqueName
	}
	return strings.ToLower(botId)
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
