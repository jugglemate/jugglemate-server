package configures

import (
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Port int `yaml:"port"`

	Log struct {
		LogPath string `yaml:"logPath"`
		LogName string `yaml:"logName"`
	} `yaml:"log"`

	Mysql struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Address  string `yaml:"address"`
		JmateDb  string `yaml:"jmateDb"`
		Debug    bool   `yaml:"debug"`
	} `yaml:"mysql"`

	ImApiDomain   string `yaml:"imApiDomain"`
	ImAdminDomain string `yaml:"imAdminDomain"`
	JmateBaseUrl  string `yaml:"jmateBaseUrl"`

	AiBotCallbackUrl string `yaml:"aiBotCallbackUrl"`

	// OSS 统一上传配置
	Oss OssConfig `yaml:"oss"`

	// AgentAdmin 代理到 agent-server Python 管理 API（/api/v1/*）的配置。
	// 与用于 twins/chat 运行时的 agentServer 区分开。
	AgentAdmin AgentAdminConfig `yaml:"agentAdmin"`
}

// AgentAdminConfig agent-server 管理 API 反向代理配置。
type AgentAdminConfig struct {
	// BaseURL Python agent-server 源地址（不含 /api/v1），例如 http://127.0.0.1:8000。
	BaseURL string `yaml:"baseURL"`
	// InternalSecret Go 代理与 Python 之间的可信内部共享密钥，禁止下发浏览器。
	InternalSecret string `yaml:"internalSecret"`
}

// OssConfig 阿里云 OSS 上传配置
type OssConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
}

var Config AppConfig
var Env string

const (
	EnvDev  = "dev"
	EnvProd = "prod"
)

func InitConfigures() error {
	cfBytes, err := os.ReadFile("conf/config.yml")
	if err == nil {
		var conf AppConfig
		yaml.Unmarshal(cfBytes, &conf)
		Config = conf
		if Config.Port <= 0 {
			Config.Port = 8050
		}
		return nil
	}
	return err
}
