package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config app settings
type Config struct {
	GitLabToken   string `json:"gitlab_token"`
	RootGroupPath string `json:"root_group_path"`
	BaseURL       string `json:"gitlab_base_url"`

	PushAccessLevel  int `json:"push_access_level"`
	MergeAccessLevel int `json:"merge_access_level"`
}

// LoadConfig reads и parses JSON-file config
func LoadConfig(path string) (*Config, error) {
	cfgFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config file %q: %w", path, err)
	}
	defer cfgFile.Close()

	var cfg Config
	if err = json.NewDecoder(cfgFile).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if cfg.GitLabToken == "" || cfg.RootGroupPath == "" || cfg.BaseURL == "" {
		return nil, fmt.Errorf("one of config variables is empty")
	}

	if cfg.PushAccessLevel == 0 {
		cfg.PushAccessLevel = 30
	}
	if cfg.MergeAccessLevel == 0 {
		cfg.MergeAccessLevel = 30
	}

	return &cfg, nil
}
