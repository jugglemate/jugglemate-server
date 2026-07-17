package configures

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AppConfig 定义 JMate 主服务及内置 Go Agent 模块的完整运行配置。
type AppConfig struct {
	Port int `yaml:"port"`

	Log struct {
		LogPath string `yaml:"logPath"`
		LogName string `yaml:"logName"`
	} `yaml:"log"`

	// ImApiDomain 是 Bot 注册、群组创建和群成员变更共用的 IM HTTP API 根地址。
	//
	// TIPS: Bot 的 token 由这里签发，必须与 Agent.IM.WSAddress 属于同一套 IM，否则 Bot
	// 建立长连接时会被拒。IM 的 AppKey 来自请求登录态、AppSecret 取自 apps 表，均不在配置里。
	ImApiDomain  string `yaml:"imApiDomain"`
	JmateBaseUrl string `yaml:"jmateBaseUrl"`

	// OSS 统一上传配置
	Oss OssConfig `yaml:"oss"`

	// Agent Go 版智能体平台模块配置。
	Agent AgentConfig `yaml:"agent"`
}

// AgentConfig Go 版智能体平台模块配置。
//
// PostgreSQL 同时用于客服业务、Agent 业务数据和向量检索，Redis 用于队列、锁及临时状态。
type AgentConfig struct {
	Enabled bool `yaml:"enabled"`

	Postgres  AgentPostgresConfig  `yaml:"postgres"`
	Redis     AgentRedisConfig     `yaml:"redis"`
	Auth      AgentAuthConfig      `yaml:"auth"`
	Security  AgentSecurityConfig  `yaml:"security"`
	Knowledge AgentKnowledgeConfig `yaml:"knowledge"`
	Tools     AgentToolsConfig     `yaml:"tools"`
	Billing   AgentBillingConfig   `yaml:"billing"`
	Profile   AgentProfileConfig   `yaml:"profile"`
	IM        AgentIMConfig        `yaml:"im"`
}

// AgentPostgresConfig Agent PostgreSQL 连接配置。
type AgentPostgresConfig struct {
	DSN                    string `yaml:"dsn"`
	Debug                  bool   `yaml:"debug"`
	MaxIdleConns           int    `yaml:"maxIdleConns"`
	MaxOpenConns           int    `yaml:"maxOpenConns"`
	ConnMaxLifetimeSeconds int    `yaml:"connMaxLifetimeSeconds"`
}

// AgentRedisConfig Agent Redis 连接配置。
type AgentRedisConfig struct {
	Address            string `yaml:"address"`
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	DB                 int    `yaml:"db"`
	DialTimeoutSeconds int    `yaml:"dialTimeoutSeconds"`
}

// AgentAuthConfig Agent 原生 API 与可信内部调用鉴权配置。
//
// 当前生产入口与源服务一致，仅接受 JMate 上游校验后转发的可信内部密钥和身份；
// 旧的账号密码登录与 Bearer JWT 均已下线，对应配置项已移除。
type AgentAuthConfig struct {
	Enabled        bool   `yaml:"enabled"`
	InternalSecret string `yaml:"internalSecret"`
}

// AgentSecurityConfig Agent 密钥加密配置。
type AgentSecurityConfig struct {
	SecretKey  string `yaml:"secretKey"`
	SecretSalt string `yaml:"secretSalt"`
}

// AgentKnowledgeConfig Agent 知识库与向量化配置。
type AgentKnowledgeConfig struct {
	WorkerEnabled      bool   `yaml:"workerEnabled"`
	VectorDimension    int    `yaml:"vectorDimension"`
	VectorBatchSize    int    `yaml:"vectorBatchSize"`
	MaxFileSizeMB      int    `yaml:"maxFileSizeMB"`
	VectorizeStream    string `yaml:"vectorizeStream"`
	ConsumerGroup      string `yaml:"consumerGroup"`
	ConsumerName       string `yaml:"consumerName"`
	TaskTimeoutSeconds int    `yaml:"taskTimeoutSeconds"`
	MaxRetries         int    `yaml:"maxRetries"`
	ObjectStorageRoot  string `yaml:"objectStorageRoot"`
}

// AgentToolsConfig Agent 系统工具初始化配置。
type AgentToolsConfig struct {
	InnerAPIBaseURL string `yaml:"innerAPIBaseURL"`
}

// AgentBillingConfig Agent 直连会话的日 Token 配额配置。
type AgentBillingConfig struct {
	DirectFreeDailyEnabled *bool  `yaml:"directFreeDailyEnabled"`
	DirectFreeDailyTokens  *int   `yaml:"directFreeDailyTokens"`
	DirectQuotaTimezone    string `yaml:"directQuotaTimezone"`
}

// AgentProfileConfig Agent 创建赠送与启动余额校验配置。
type AgentProfileConfig struct {
	FirstAgentRechargeAmount     string `yaml:"firstAgentRechargeAmount"`
	ActivationPointsCheckEnabled *bool  `yaml:"activationPointsCheckEnabled"`
}

// AgentIMConfig IM Bot 注册、WebSocket 连接和默认路由配置。
type AgentIMConfig struct {
	Enabled           *bool  `yaml:"enabled"`
	WSAddress         string `yaml:"wsAddress"`
	ServerAPIInsecure bool   `yaml:"serverAPIInsecure"`
}

// OssConfig 定义阿里云 OSS 上传配置。
type OssConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
}

// Config 保存已加载并补齐默认值的全局应用配置。
var Config AppConfig

// Env 保存当前运行环境标识。
var Env string

const (
	EnvDev  = "dev"  // EnvDev 表示开发环境。
	EnvProd = "prod" // EnvProd 表示生产环境。
)

// InitConfigures 从 conf/config.yml 加载配置并应用安全默认值。
func InitConfigures() error {
	cfBytes, err := os.ReadFile("conf/config.yml")
	if err == nil {
		var conf AppConfig
		if err := yaml.Unmarshal(cfBytes, &conf); err != nil {
			return fmt.Errorf("解析配置文件失败: %w", err)
		}
		applyDefaults(&conf)
		Config = conf
		return nil
	}
	return err
}

// applyDefaults 为可选配置补齐安全的运行默认值。
func applyDefaults(conf *AppConfig) {
	if conf.Port <= 0 {
		conf.Port = 8050
	}
	if conf.Agent.Postgres.MaxIdleConns <= 0 {
		conf.Agent.Postgres.MaxIdleConns = 10
	}
	if conf.Agent.Postgres.MaxOpenConns <= 0 {
		conf.Agent.Postgres.MaxOpenConns = 50
	}
	if conf.Agent.Postgres.ConnMaxLifetimeSeconds <= 0 {
		conf.Agent.Postgres.ConnMaxLifetimeSeconds = 3600
	}
	if conf.Agent.Redis.DialTimeoutSeconds <= 0 {
		conf.Agent.Redis.DialTimeoutSeconds = 5
	}
	if conf.Agent.Knowledge.VectorDimension <= 0 {
		conf.Agent.Knowledge.VectorDimension = 1536
	}
	if conf.Agent.Knowledge.VectorBatchSize <= 0 {
		conf.Agent.Knowledge.VectorBatchSize = 32
	}
	if conf.Agent.Knowledge.MaxFileSizeMB <= 0 {
		conf.Agent.Knowledge.MaxFileSizeMB = 10
	}
	if conf.Agent.Knowledge.VectorizeStream == "" {
		conf.Agent.Knowledge.VectorizeStream = "knowledge_vectorize_stream"
	}
	if conf.Agent.Knowledge.ConsumerGroup == "" {
		conf.Agent.Knowledge.ConsumerGroup = "knowledge_vectorize_group"
	}
	if conf.Agent.Knowledge.ConsumerName == "" {
		conf.Agent.Knowledge.ConsumerName = "knowledge_vectorize_worker_1"
	}
	if conf.Agent.Knowledge.TaskTimeoutSeconds <= 0 {
		conf.Agent.Knowledge.TaskTimeoutSeconds = 600
	}
	if conf.Agent.Knowledge.MaxRetries <= 0 {
		conf.Agent.Knowledge.MaxRetries = 3
	}
	if conf.Agent.Knowledge.ObjectStorageRoot == "" {
		conf.Agent.Knowledge.ObjectStorageRoot = "./data/agent/knowledge"
	}
	if conf.Agent.Tools.InnerAPIBaseURL == "" {
		conf.Agent.Tools.InnerAPIBaseURL = "https://t.JG.pro"
	}
	if conf.Agent.Billing.DirectFreeDailyTokens == nil {
		tokens := 500000
		conf.Agent.Billing.DirectFreeDailyTokens = &tokens
	}
	if conf.Agent.Billing.DirectFreeDailyEnabled == nil {
		enabled := true
		conf.Agent.Billing.DirectFreeDailyEnabled = &enabled
	}
	if conf.Agent.Billing.DirectQuotaTimezone == "" {
		conf.Agent.Billing.DirectQuotaTimezone = "Asia/Shanghai"
	}
	if conf.Agent.Profile.FirstAgentRechargeAmount == "" {
		conf.Agent.Profile.FirstAgentRechargeAmount = "10000"
	}
	if conf.Agent.Profile.ActivationPointsCheckEnabled == nil {
		enabled := true
		conf.Agent.Profile.ActivationPointsCheckEnabled = &enabled
	}
	if conf.Agent.IM.Enabled == nil {
		enabled := true
		conf.Agent.IM.Enabled = &enabled
	}
	if conf.Agent.IM.WSAddress == "" {
		conf.Agent.IM.WSAddress = "wss://127.0.0.1"
	}
}
