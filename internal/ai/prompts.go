package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"skyvern/internal/config"
)

type PromptsConfig struct {
	SystemPrompt string `json:"system_prompt"`
}

func GetPromptsPath() string {
	return config.ResolvePath("internal/ai/prompts.json")
}

func LoadPrompts() (PromptsConfig, error) {
	var cfg PromptsConfig
	p := GetPromptsPath()
	b, err := os.ReadFile(p)
	if err != nil {
		return PromptsConfig{SystemPrompt: "You are a helpful AI assistant."}, nil
	}
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		return PromptsConfig{SystemPrompt: "You are a helpful AI assistant."}, nil
	}
	return cfg, nil
}

func SavePrompts(cfg PromptsConfig) error {
	p := GetPromptsPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}
