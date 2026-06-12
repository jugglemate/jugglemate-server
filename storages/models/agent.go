package models

type SyncStatus string

const (
	SyncStatusPending SyncStatus = "pending"
	SyncStatusSynced  SyncStatus = "synced"
	SyncStatusFailed  SyncStatus = "failed"
)

type AgentTwin struct {
	ID             int64
	AppKey         string
	UniqueName     string
	BotId          string
	DisplayName    string
	AvatarURL      string
	Greeting       string
	Prompts        string
	OwnerId        string
	Status         string
	ActiveVersion  string
	TrainingMode   string
	MaterialsCount int
	SyncStatus     string
	SyncError      string
	LastSyncedAt   int64
	UpdatedTime    int64
	CreatedTime    int64
}

type AgentMaterial struct {
	ID           int64
	AppKey       string
	UniqueName   string
	MaterialId   string
	Type         string
	Title        string
	Source       string
	Content      string
	URL          string
	FilePath     string
	SizeBytes    int64
	SyncStatus   string
	SyncError    string
	LastSyncedAt int64
	UpdatedTime  int64
	CreatedTime  int64
}

type AgentJob struct {
	ID             int64
	AppKey         string
	JobId          string
	UniqueName     string
	Type           string
	Status         string
	Progress       *int
	ResultJSON     string
	ErrorCode      string
	ErrorMessage   string
	AgentCreatedAt int64
	StartedAt      int64
	FinishedAt     int64
	UpdatedTime    int64
	CreatedTime    int64
}

type AgentVersion struct {
	ID             int64
	AppKey         string
	UniqueName     string
	Version        string
	Mode           string
	Active         bool
	TrainingJobId  string
	MaterialsCount int
	AgentCreatedAt int64
	UpdatedTime    int64
	CreatedTime    int64
}

type AgentEvaluation struct {
	ID             int64
	AppKey         string
	EvaluationId   string
	UniqueName     string
	Version        string
	OverallScore   *float64
	DimensionsJSON string
	SummaryMD      string
	AgentCreatedAt int64
	UpdatedTime    int64
	CreatedTime    int64
}

type AgentMessage struct {
	ID             int64
	AppKey         string
	UniqueName     string
	CustomerId     string
	IMMsgId        string
	AgentMessageId string
	SessionId      string
	Role           string
	Text           string
	Fallback       bool
	Source         string
	Platform       string
	ConverType     int
	RawPayload     string
	MsgTime        int64
	UpdatedTime    int64
	CreatedTime    int64
}

type AgentStorage interface {
	CreateTwin(item AgentTwin) error
	UpdateTwin(item AgentTwin) error
	DeleteTwin(appkey, uniqueName string) error
	FindTwin(appkey, uniqueName string) (*AgentTwin, error)
	FindTwinByBotId(appkey, botId string) (*AgentTwin, error)
	FindTwinByAnyKey(appkey, key string) (*AgentTwin, error)
	QryTwinsByOwner(appkey, ownerId string, startId, limit int64) ([]*AgentTwin, error)
	CreateTwinTombstone(appkey, uniqueName, ownerId string) error
	HasTwinTombstone(appkey, uniqueName string) (bool, error)

	CreateMaterial(item AgentMaterial) error
	DeleteMaterial(appkey, materialId string) error
	QryMaterials(appkey, uniqueName string, startId, limit int64) ([]*AgentMaterial, error)

	UpsertJob(item AgentJob) error
	FindJob(appkey, jobId string) (*AgentJob, error)
	QryJobs(appkey, uniqueName, jobType, status string, startId, limit int64) ([]*AgentJob, error)

	UpsertVersion(item AgentVersion) error
	QryVersions(appkey, uniqueName string, startId, limit int64) ([]*AgentVersion, error)
	FindCurrentVersion(appkey, uniqueName string) (*AgentVersion, error)
	SetActiveVersion(appkey, uniqueName, version string) error

	UpsertEvaluation(item AgentEvaluation) error
	FindEvaluation(appkey, evaluationId string) (*AgentEvaluation, error)

	CreateMessage(item AgentMessage) error
	FindMessageByIM(appkey, imMsgId, role string) (*AgentMessage, error)
	QryMessages(appkey, uniqueName, customerId string, startId, limit int64) ([]*AgentMessage, error)
	QryMessagesByIM(appkey, imMsgId string) ([]*AgentMessage, error)
}
