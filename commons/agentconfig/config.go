package agentconfig

import (
	"log"
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
	var result time.Duration
	if cfg.AgentServer.TimeoutSeconds > 0 {
		result = time.Duration(cfg.AgentServer.TimeoutSeconds) * time.Second
	} else {
		result = 5 * time.Second
	}
	log.Printf("[agentconfig] Timeout() returns %v", result)
	return result
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
		log.Printf("[agentconfig] reading config from: %s", configPath)
		bs, err := os.ReadFile(configPath)
		if err != nil {
			log.Printf("[agentconfig] failed to read config: %v", err)
			return
		}
		if err := yaml.Unmarshal(bs, &cfg); err != nil {
			log.Printf("[agentconfig] failed to unmarshal config: %v", err)
			return
		}
		tokenPreview := cfg.AgentServer.Token
		if len(tokenPreview) > 8 {
			tokenPreview = tokenPreview[:8] + "..."
		}
		log.Printf("[agentconfig] loaded agentServer: baseURL=%s timeoutSeconds=%d token=%s",
			cfg.AgentServer.BaseURL, cfg.AgentServer.TimeoutSeconds, tokenPreview)
	})
}
