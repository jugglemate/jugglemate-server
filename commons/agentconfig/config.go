package agentconfig

import (
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type configFile struct {
	AgentServer struct {
		BaseURL        string `yaml:"baseURL"`
		TimeoutSeconds int    `yaml:"timeoutSeconds"`
		Authorization  string `yaml:"authorization"`
		Token          string `yaml:"token"`
	} `yaml:"agentServer"`
}

var (
	cfg     configFile
	cfgOnce sync.Once
)

func BaseURL() string {
	readConfigOnce()
	return cfg.AgentServer.BaseURL
}

func Timeout() time.Duration {
	readConfigOnce()
	if cfg.AgentServer.TimeoutSeconds > 0 {
		return time.Duration(cfg.AgentServer.TimeoutSeconds) * time.Second
	}
	return 5 * time.Second
}

func Authorization() string {
	readConfigOnce()
	return cfg.AgentServer.Authorization
}

func TwinsToken() string {
	readConfigOnce()
	return cfg.AgentServer.Token
}

func readConfigOnce() {
	cfgOnce.Do(func() {
		configPath := os.Getenv("CONFIG_PATH")
		if configPath == "" {
			configPath = "conf/config.yml"
		}
		bs, err := os.ReadFile(configPath)
		if err != nil {
			return
		}
		_ = yaml.Unmarshal(bs, &cfg)
	})
}
