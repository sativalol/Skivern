package ai

import "time"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenOpts struct {
	SystemMsg   string            `json:"system_msg"`
	Messages    []Message         `json:"messages"`
	UserMsg     string            `json:"user_msg"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens"`
	JSONMode    bool              `json:"json_mode"`
	Variables   map[string]string `json:"variables"`
	FallbackID  string            `json:"fallback_id"`
}

type Result struct {
	Text       string        `json:"text"`
	Model      string        `json:"model"`
	ProviderID string        `json:"provider_id"`
	Duration   time.Duration `json:"duration"`
}

type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponseFormat struct {
	Type string `json:"type"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIChatRequest struct {
	Model          string                `json:"model"`
	Messages       []OpenAIChatMessage   `json:"messages"`
	Temperature    *float64              `json:"temperature,omitempty"`
	MaxTokens      int                   `json:"max_tokens,omitempty"`
	ResponseFormat *OpenAIResponseFormat `json:"response_format,omitempty"`
}

type OpenAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type AnthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []AnthropicMessage `json:"messages"`
	Temperature *float64           `json:"temperature,omitempty"`
}

type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage *AnthropicUsage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}
