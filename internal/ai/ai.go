package ai

import (
	"fmt"
	"net/http"
	"skyvern/internal/storage"
	"strings"
	"time"
)

var httpCli = &http.Client{
	Timeout: 45 * time.Second,
}

func Generate(db *storage.DB, id string, opts GenOpts) (Result, error) {
	start := time.Now()
	prov, err := db.GetAIProvider(id)

	isLimitHit := false
	if err == nil {
		if (prov.MaxTokens > 0 && prov.UsedTokens >= prov.MaxTokens) || (prov.MaxRequests > 0 && prov.UsedRequests >= prov.MaxRequests) {
			isLimitHit = true
		}
	}

	if err != nil || isLimitHit {
		fallback := resolveFallback(db, id, opts.FallbackID)
		if fallback == "" && err == nil {
			fallback = resolveFallback(db, id, prov.FallbackID)
		}
		if fallback != "" && fallback != id {
			return Generate(db, fallback, opts)
		}
		if isLimitHit {
			return Result{}, fmt.Errorf("provider limit reached: %s", id)
		}
		return Result{}, err
	}

	resText, modelUsed, tokensUsed, err := execGen(prov, opts)
	if err != nil {
		fallback := resolveFallback(db, id, opts.FallbackID)
		if fallback == "" {
			fallback = resolveFallback(db, id, prov.FallbackID)
		}
		if fallback != "" && fallback != id {
			return Generate(db, fallback, opts)
		}
		return Result{}, err
	}

	prov.UsedTokens += tokensUsed
	prov.UsedRequests += 1
	_ = db.SaveAIProvider(prov)

	return Result{
		Text:       resText,
		Model:      modelUsed,
		ProviderID: prov.ID,
		Duration:   time.Since(start),
	}, nil
}

func resolveFallback(db *storage.DB, currentID, fallbackID string) string {
	target := fallbackID
	if target == "random" {
		list, err := db.ListAIProviders()
		if err == nil && len(list) > 1 {
			var pool []string
			for _, item := range list {
				if item.ID == currentID {
					continue
				}
				if item.MaxTokens > 0 && item.UsedTokens >= item.MaxTokens {
					continue
				}
				if item.MaxRequests > 0 && item.UsedRequests >= item.MaxRequests {
					continue
				}
				if item.APIKey == "" && item.Type != "ollama" {
					continue
				}
				pool = append(pool, item.ID)
			}
			if len(pool) > 0 {
				idx := int(time.Now().UnixNano() % int64(len(pool)))
				return pool[idx]
			}
		}
		return ""
	}
	return target
}

func renderTmpl(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

func execGen(p storage.AIProvider, opts GenOpts) (string, string, int64, error) {
	if p.APIKey == "" && p.Type != "ollama" {
		return "", "", 0, fmt.Errorf("api key required for provider: %s", p.ID)
	}

	url := p.BaseURL
	if url == "" {
		switch p.Type {
		case "openai":
			url = "https://api.openai.com/v1/chat/completions"
		case "anthropic":
			url = "https://api.anthropic.com/v1/messages"
		case "deepseek":
			url = "https://api.deepseek.com/chat/completions"
		case "openrouter":
			url = "https://openrouter.ai/api/v1/chat/completions"
		case "groq":
			url = "https://api.groq.com/openai/v1/chat/completions"
		case "mistral":
			url = "https://api.mistral.ai/v1/chat/completions"
		case "perplexity":
			url = "https://api.perplexity.ai/chat/completions"
		case "gemini":
			url = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
		case "cohere":
			url = "https://api.cohere.com/v2/chat"
		case "ollama":
			url = "http://localhost:11434/v1/chat/completions"
		default:
			url = "https://api.openai.com/v1/chat/completions"
		}
	}

	model := p.DefaultModel
	if model == "" {
		switch p.Type {
		case "openai":
			model = "gpt-4o-mini"
		case "anthropic":
			model = "claude-3-5-haiku-20241022"
		case "deepseek":
			model = "deepseek-chat"
		case "openrouter":
			model = "google/gemini-2.5-flash"
		case "groq":
			model = "llama-3.3-70b-versatile"
		case "mistral":
			model = "mistral-small-latest"
		case "perplexity":
			model = "sonar-reasoning"
		case "gemini":
			model = "gemini-2.5-flash"
		case "cohere":
			model = "command-r-plus"
		case "ollama":
			model = "llama3"
		default:
			model = "gpt-4o-mini"
		}
	}

	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 1500
	}
	var tempPtr *float64
	if opts.Temperature > 0 {
		tempPtr = &opts.Temperature
	}

	sysMsg := renderTmpl(opts.SystemMsg, opts.Variables)
	if opts.JSONMode {
		if sysMsg == "" {
			sysMsg = "Output in valid JSON."
		} else if !strings.Contains(strings.ToLower(sysMsg), "json") {
			sysMsg += " (Must output valid JSON)"
		}
	}

	var msgs []Message
	for _, m := range opts.Messages {
		msgs = append(msgs, Message{
			Role:    m.Role,
			Content: renderTmpl(m.Content, opts.Variables),
		})
	}

	if len(msgs) == 0 {
		msgs = append(msgs, Message{
			Role:    "user",
			Content: renderTmpl(opts.UserMsg, opts.Variables),
		})
	}

	if p.Type == "anthropic" {
		txt, tokens, err := execAnthropic(url, model, p.APIKey, sysMsg, msgs, tempPtr, opts.MaxTokens)
		return txt, model, tokens, err
	}

	txt, tokens, err := execOpenAI(url, model, p.APIKey, p.Type, sysMsg, msgs, tempPtr, opts.MaxTokens, opts.JSONMode)
	return txt, model, tokens, err
}

func execWithRetry(req *http.Request) (*http.Response, error) {
	var res *http.Response
	var err error
	delay := 1 * time.Second

	for i := 0; i < 3; i++ {
		if i > 0 && req.GetBody != nil {
			if body, err := req.GetBody(); err == nil {
				req.Body = body
			}
		}

		res, err = httpCli.Do(req)
		if err == nil {
			if res.StatusCode != 429 && res.StatusCode < 500 {
				return res, nil
			}
			res.Body.Close()
		}

		if i < 2 {
			time.Sleep(delay)
			delay *= 2
		}
	}
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("request failed with status %d", res.StatusCode)
}
