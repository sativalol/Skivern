package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"skyvern/internal/config"
	"skyvern/internal/storage"
)

func GetPromptsPath() string {
	return config.ResolvePath("internal/ai/prompts.json")
}

func LoadPrompts() (map[string]storage.AIPrompt, error) {
	p := GetPromptsPath()
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var res map[string]storage.AIPrompt
	err = json.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func SavePrompts(m map[string]storage.AIPrompt) error {
	p := GetPromptsPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

func SyncPrompts(db *storage.DB) {
	jsonPrompts, err := LoadPrompts()
	if err != nil {
		if os.IsNotExist(err) {
			dbPrompts, err := db.ListAIPrompts()
			if err == nil && len(dbPrompts) > 0 {
				m := make(map[string]storage.AIPrompt)
				for _, p := range dbPrompts {
					m[p.Name] = p
				}
				_ = SavePrompts(m)
			} else {
				m := map[string]storage.AIPrompt{
					"default": {
						Name:         "default",
						SystemMsg:    "You are a helpful AI assistant.",
						Temperature:  0.7,
						MaxTokens:    1000,
					},
				}
				_ = SavePrompts(m)
				_ = db.SaveAIPrompt(m["default"])
			}
		}
		return
	}

	healed := false
	for k, val := range jsonPrompts {
		if len(k) > 50 || strings.Contains(k, " ") || k == "system_prompt" {
			var mergedParts []string
			if k != "system_prompt" {
				mergedParts = append(mergedParts, k)
			}
			if val.SystemMsg != "" {
				mergedParts = append(mergedParts, val.SystemMsg)
			}
			if val.UserTemplate != "" {
				mergedParts = append(mergedParts, val.UserTemplate)
			}

			defaultPrompt := storage.AIPrompt{
				Name:         "default",
				SystemMsg:    strings.Join(mergedParts, "\n\n"),
				Temperature:  0.7,
				MaxTokens:    1200,
			}

			jsonPrompts = map[string]storage.AIPrompt{
				"default": defaultPrompt,
			}
			_ = SavePrompts(jsonPrompts)
			healed = true
			break
		}
	}

	dbPrompts, err := db.ListAIPrompts()
	if err == nil {
		if healed {
			for _, dbP := range dbPrompts {
				_ = db.DeleteAIPrompt(dbP.Name)
			}
		} else {
			for _, dbP := range dbPrompts {
				if _, ok := jsonPrompts[dbP.Name]; !ok {
					_ = db.DeleteAIPrompt(dbP.Name)
				}
			}
		}
	}
	for _, p := range jsonPrompts {
		_ = db.SaveAIPrompt(p)
	}
}

