package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"skyvern/internal/config"
)

func GetPromptsPath() string {
	return config.ResolvePath("internal/ai/prompts.json")
}

func LoadPrompts() (map[string]string, error) {
	p := GetPromptsPath()
	b, err := os.ReadFile(p)
	if err != nil {
		return map[string]string{
			"default": "You are a helpful AI assistant named Skyvern.",
		}, nil
	}
	var res map[string]string
	err = json.Unmarshal(b, &res)
	if err != nil {
		return map[string]string{
			"default": "You are a helpful AI assistant named Skyvern.",
		}, nil
	}
	return res, nil
}

func SavePrompts(m map[string]string) error {
	p := GetPromptsPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}
