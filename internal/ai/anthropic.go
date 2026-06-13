package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func execAnthropic(url, model, key, sysMsg string, msgs []Message, temp *float64, maxTokens int) (string, int64, error) {
	reqBody := AnthropicRequest{
		Model:       model,
		MaxTokens:   maxTokens,
		System:      sysMsg,
		Temperature: temp,
	}

	for _, m := range msgs {
		role := m.Role
		if role == "system" {
			continue
		}
		reqBody.Messages = append(reqBody.Messages, AnthropicMessage{
			Role:    role,
			Content: m.Content,
		})
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")

	res, err := execWithRetry(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", 0, err
	}

	if res.StatusCode != 200 {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
			return "", 0, fmt.Errorf("anthropic error %d: %s", res.StatusCode, apiErr.Error.Message)
		}
		return "", 0, fmt.Errorf("anthropic error %d: %s", res.StatusCode, string(body))
	}

	var apiRes AnthropicResponse
	if err := json.Unmarshal(body, &apiRes); err != nil {
		return "", 0, err
	}

	if apiRes.Error != nil && apiRes.Error.Message != "" {
		return "", 0, fmt.Errorf("anthropic error: %s", apiRes.Error.Message)
	}

	if len(apiRes.Content) == 0 {
		return "", 0, fmt.Errorf("empty anthropic content")
	}

	var tokens int64
	if apiRes.Usage != nil {
		tokens = int64(apiRes.Usage.InputTokens + apiRes.Usage.OutputTokens)
	}

	return apiRes.Content[0].Text, tokens, nil
}
