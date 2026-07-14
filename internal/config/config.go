package config

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"gh-mirror/pkg/models"
	"gh-mirror/pkg/platform"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration loaded from config.yaml.
type Config struct {
	Platforms    map[string]PlatformConfig `yaml:"platforms"`
	Source       string                   `yaml:"source"`
	Destinations []string                 `yaml:"destinations"`
	Sync         SyncConfig               `yaml:"sync"`
}

// PlatformConfig holds authentication and endpoint settings for a single platform.
type PlatformConfig struct {
	Token  string `yaml:"token"`
	APIURL string `yaml:"api_url"`
	URL    string `yaml:"url"`
	Owner  string `yaml:"owner"`
}

// SyncConfig controls sync behavior.
type SyncConfig struct {
	TimeoutMinutes int `yaml:"timeout_minutes"`
}

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandEnvValue replaces ${VAR} patterns with the corresponding environment variable values.
func expandEnvValue(value string) string {
	return envVarPattern.ReplaceAllStringFunc(value, func(match string) string {
		envName := match[2 : len(match)-1]
		envVal := os.Getenv(envName)
		if envVal == "" {
			slog.Warn("environment variable is empty or not set", "var", envName)
		}
		return envVal
	})
}

// Load reads and validates a configuration file from path.
// It first parses the YAML, then expands ${VAR} environment variables
// in platform token, URL, and API URL fields.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	for k, v := range cfg.Platforms {
		v.Token = expandEnvValue(v.Token)
		v.APIURL = expandEnvValue(v.APIURL)
		v.URL = expandEnvValue(v.URL)
		cfg.Platforms[k] = v
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Source == "" {
		return &ConfigError{Field: "source", Message: "required"}
	}

	sourceID := models.PlatformID(c.Source)
	if _, err := platform.Create(sourceID); err != nil {
		return &ConfigError{Field: "source", Message: fmt.Sprintf("unsupported platform: %s", c.Source)}
	}

	if c.Platforms == nil {
		c.Platforms = make(map[string]PlatformConfig)
	}

	if _, hasSourceConfig := c.Platforms[c.Source]; !hasSourceConfig {
		return &ConfigError{Field: fmt.Sprintf("platforms.%s", c.Source), Message: "platform configuration required"}
	}

	for _, dest := range c.Destinations {
		destID := models.PlatformID(dest)
		if _, err := platform.Create(destID); err != nil {
			return &ConfigError{Field: "destinations", Message: fmt.Sprintf("unsupported platform: %s", dest)}
		}
		if _, hasDestConfig := c.Platforms[dest]; !hasDestConfig {
			return &ConfigError{Field: fmt.Sprintf("platforms.%s", dest), Message: "platform configuration required"}
		}
	}

	for _, dest := range c.Destinations {
		if dest == c.Source {
			return &ConfigError{Field: "destinations", Message: fmt.Sprintf("destination cannot be same as source: %s", c.Source)}
		}
	}

	if c.Sync.TimeoutMinutes == 0 {
		c.Sync.TimeoutMinutes = 30
	}

	return nil
}

// ConfigError is a structured configuration validation error.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config: %s %s", e.Field, e.Message)
}
