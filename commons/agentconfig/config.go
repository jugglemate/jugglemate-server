package agentconfig

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/juggleim/jugglechat-server-ai/commons/configures"
	"gopkg.in/yaml.v3"
)

type configFile struct {
	AgentServer struct {
		BaseURL        string `yaml:"baseURL"`
		TimeoutSeconds int    `yaml:"timeoutSeconds"`
		Authorization  string `yaml:"authorization"`
	} `yaml:"agentServer"`
}

var (
	cfg     configFile
	cfgOnce sync.Once
)

func BaseURL() string {
	readConfigOnce()
	if cfg.AgentServer.BaseURL != "" {
		return cfg.AgentServer.BaseURL
	}
	return configures.Config.BotConnector.Domain
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

func readConfigOnce() {
	cfgOnce.Do(func() {
		configPath := os.Getenv("CONFIG_PATH")
		if configPath == "" {
			execDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
			configPath = filepath.Join(execDir, "conf", "config.yml")
		}
		bs, err := os.ReadFile(configPath)
		if err != nil {
			return
		}
		_ = yaml.Unmarshal(bs, &cfg)
	})
}
