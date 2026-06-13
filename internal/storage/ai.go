package storage

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

type AIProvider struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	APIKey       string `json:"api_key"`
	BaseURL      string `json:"base_url"`
	DefaultModel string `json:"default_model"`
	FallbackID   string `json:"fallback_id"`
	MaxTokens    int64  `json:"max_tokens"`
	UsedTokens   int64  `json:"used_tokens"`
	MaxRequests  int64  `json:"max_requests"`
	UsedRequests int64  `json:"used_requests"`
}

type AIPrompt struct {
	Name         string  `json:"name"`
	SystemMsg    string  `json:"system_msg"`
	UserTemplate string  `json:"user_template"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
}

func (d *DB) SaveAIProvider(prov AIProvider) error {
	if prov.ID == "" {
		return fmt.Errorf("provider ID required")
	}
	encKey, err := encrypt(prov.APIKey)
	if err != nil {
		return err
	}
	prov.APIKey = encKey
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktAIProviders), []byte(prov.ID), prov)
	})
}

func (d *DB) GetAIProvider(id string) (AIProvider, error) {
	var prov AIProvider
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktAIProviders).Get([]byte(id))
		if v == nil {
			return fmt.Errorf("provider not found: %s", id)
		}
		return json.Unmarshal(v, &prov)
	})
	if err == nil {
		if decKey, decErr := decrypt(prov.APIKey); decErr == nil {
			prov.APIKey = decKey
		}
	}
	return prov, err
}

func (d *DB) DeleteAIProvider(id string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAIProviders).Delete([]byte(id))
	})
}

func (d *DB) ListAIProviders() ([]AIProvider, error) {
	var list []AIProvider
	err := d.b.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAIProviders).ForEach(func(k, v []byte) error {
			var p AIProvider
			if err := json.Unmarshal(v, &p); err == nil {
				if decKey, decErr := decrypt(p.APIKey); decErr == nil {
					p.APIKey = decKey
				}
				list = append(list, p)
			}
			return nil
		})
	})
	return list, err
}

func (d *DB) SaveAIPrompt(prompt AIPrompt) error {
	if prompt.Name == "" {
		return fmt.Errorf("prompt name required")
	}
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktAIPrompts), []byte(prompt.Name), prompt)
	})
}

func (d *DB) GetAIPrompt(name string) (AIPrompt, error) {
	var prompt AIPrompt
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktAIPrompts).Get([]byte(name))
		if v == nil {
			return fmt.Errorf("prompt not found: %s", name)
		}
		return json.Unmarshal(v, &prompt)
	})
	return prompt, err
}

func (d *DB) DeleteAIPrompt(name string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAIPrompts).Delete([]byte(name))
	})
}

func (d *DB) ListAIPrompts() ([]AIPrompt, error) {
	var list []AIPrompt
	err := d.b.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAIPrompts).ForEach(func(k, v []byte) error {
			var p AIPrompt
			if err := json.Unmarshal(v, &p); err == nil {
				list = append(list, p)
			}
			return nil
		})
	})
	return list, err
}
