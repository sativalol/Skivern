package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func execOpenAI(url, model, key, pType, sysMsg string, msgs []Message, temp *float64, maxTokens int, jsonMode bool) (string, int64, error) {
	reqBody := OpenAIChatRequest{
		Model:       model,
		Temperature: temp,
		MaxTokens:   maxTokens,
	}

	if jsonMode {
		reqBody.ResponseFormat = &OpenAIResponseFormat{Type: "json_object"}
	}

	if sysMsg != "" {
		reqBody.Messages = append(reqBody.Messages, OpenAIChatMessage{
			Role:    "system",
			Content: sysMsg,
		})
	}

	for _, m := range msgs {
		reqBody.Messages = append(reqBody.Messages, OpenAIChatMessage{
			Role:    m.Role,
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
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	if pType == "openrouter" {
		req.Header.Set("HTTP-Referer", "https://github.com/sativalol/Skyvern")
		req.Header.Set("X-Title", "Skyvern")
	}

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
			return "", 0, fmt.Errorf("api error %d: %s", res.StatusCode, apiErr.Error.Message)
		}
		return "", 0, fmt.Errorf("api error %d: %s", res.StatusCode, string(body))
	}

	var apiRes OpenAIChatResponse
	if err := json.Unmarshal(body, &apiRes); err != nil {
		return "", 0, err
	}

	if apiRes.Error != nil && apiRes.Error.Message != "" {
		return "", 0, fmt.Errorf("api error: %s", apiRes.Error.Message)
	}

	if len(apiRes.Choices) == 0 {
		return "", 0, fmt.Errorf("empty api choices")
	}

	var tokens int64
	if apiRes.Usage != nil {
		tokens = int64(apiRes.Usage.TotalTokens)
	}

	return apiRes.Choices[0].Message.Content, tokens, nil
}
