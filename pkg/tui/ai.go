package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"skyvern/internal/storage"
)

func (m Model) renderAISidebar(contentHeight, sidebarWidth int, th Theme) string {
	titleStyle := lipgloss.NewStyle().Foreground(th.Accent).Bold(true).Underline(true)
	titleFocusStyle := lipgloss.NewStyle().Foreground(th.BorderFocus).Bold(true).Underline(true)
	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(1)

	sbInnerWidth, sbInnerHeight := calcInnerLimits(sidebarWidth, contentHeight)

	var sbLines []string
	if miniLogo != "" && sbInnerHeight > 18 {
		sbLines = append(sbLines, lipgloss.NewStyle().Foreground(th.BorderFocus).Render(miniLogo))
		sbLines = append(sbLines, "")
	}

	remainingHeight := sbInnerHeight - len(sbLines) - 4
	if remainingHeight < 4 {
		remainingHeight = 4
	}
	provVisible := remainingHeight / 2
	promptVisible := remainingHeight - provVisible

	if m.aiSubTab == 0 {
		sbLines = append(sbLines, titleFocusStyle.Render("► AI PROVIDERS"))
	} else {
		sbLines = append(sbLines, titleStyle.Render("  AI PROVIDERS"))
	}
	sbLines = append(sbLines, "")

	pStart, pEnd := calcVisibleRange(len(m.aiProvs), m.aiSelIdx, provVisible)
	for i := pStart; i < pEnd; i++ {
		p := m.aiProvs[i]
		lbl := p.ID
		line := fmt.Sprintf("  %-15s", lbl)
		if m.aiSubTab == 0 && i == m.aiSelIdx {
			sbLines = append(sbLines, lipgloss.NewStyle().Foreground(th.Accent).Background(th.BorderFocus).Render(" "+line))
		} else {
			sbLines = append(sbLines, " "+line)
		}
	}

	sbLines = append(sbLines, "")

	if m.aiSubTab == 1 {
		sbLines = append(sbLines, titleFocusStyle.Render("► AI PROMPTS"))
	} else {
		sbLines = append(sbLines, titleStyle.Render("  AI PROMPTS"))
	}
	sbLines = append(sbLines, "")

	prStart, prEnd := calcVisibleRange(len(m.aiPrompts), m.aiPromptIdx, promptVisible)
	for i := prStart; i < prEnd; i++ {
		p := m.aiPrompts[i]
		lbl := p.Name
		line := fmt.Sprintf("  %-15s", lbl)
		if m.aiSubTab == 1 && i == m.aiPromptIdx {
			sbLines = append(sbLines, lipgloss.NewStyle().Foreground(th.Accent).Background(th.BorderFocus).Render(" "+line))
		} else {
			sbLines = append(sbLines, " "+line)
		}
	}

	return boxStyle.Width(sbInnerWidth).Height(sbInnerHeight).Render(strings.Join(sbLines, "\n"))
}

func (m Model) renderAIPanel(mainWidth, contentHeight int, th Theme) string {
	titleStyle := lipgloss.NewStyle().Foreground(th.Accent).Bold(true).Underline(true)
	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(1)
	mainInnerWidth := calcMainInnerWidth(mainWidth)

	if m.aiSubTab == 1 {
		var lines []string
		lines = append(lines, titleStyle.Render("AI PROMPT CONFIGURATION"))
		lines = append(lines, "")

		if len(m.aiPrompts) == 0 {
			lines = append(lines, "  No custom prompts configured.", "", "  Press [N] to add a new system prompt.")
		} else {
			p := m.aiPrompts[m.aiPromptIdx]
			lines = append(lines, fmt.Sprintf("  Prompt ID/Name:   %s", p.Name))
			lines = append(lines, fmt.Sprintf("  Temperature:      %.2f", p.Temperature))
			lines = append(lines, fmt.Sprintf("  Max Tokens Limit: %d", p.MaxTokens))
			lines = append(lines, "")
			lines = append(lines, "  System Message:")
			
			sysLines := strings.Split(p.SystemMsg, "\n")
			for i, sl := range sysLines {
				if i > 5 {
					lines = append(lines, "    ...")
					break
				}
				if len(sl) > 55 {
					sl = sl[:52] + "..."
				}
				lines = append(lines, "    "+sl)
			}
			
			if p.UserTemplate != "" {
				lines = append(lines, "", "  User Template:")
				utLines := strings.Split(p.UserTemplate, "\n")
				for i, ul := range utLines {
					if i > 3 {
						lines = append(lines, "    ...")
						break
					}
					if len(ul) > 55 {
						ul = ul[:52] + "..."
					}
					lines = append(lines, "    "+ul)
				}
			}
		}
		
		lines = append(lines, "", "  [N] Add Prompt | [E] Edit | [X] Delete | [p] Toggle Box")
		return boxStyle.Width(mainInnerWidth).Render(strings.Join(lines, "\n"))
	}

	var aiLines []string
	aiLines = append(aiLines, titleStyle.Render("AI PROVIDER CONFIGURATION"))
	aiLines = append(aiLines, "")

	if len(m.aiProvs) == 0 {
		aiLines = append(aiLines, "  No AI providers configured.", "", "  Press [N] to add a new AI provider configuration.")
	} else {
		p := m.aiProvs[m.aiSelIdx]
		aiLines = append(aiLines, fmt.Sprintf("  Provider ID:       %s", p.ID))
		aiLines = append(aiLines, fmt.Sprintf("  Provider Type:     %s", p.Type))
		aiLines = append(aiLines, fmt.Sprintf("  Provider Name:     %s", p.Name))
		keyMasked := "(not set)"
		if p.APIKey != "" {
			if len(p.APIKey) > 8 {
				keyMasked = p.APIKey[:4] + "..." + p.APIKey[len(p.APIKey)-4:]
			} else {
				keyMasked = "****"
			}
		}
		aiLines = append(aiLines, fmt.Sprintf("  API Key:           %s", keyMasked))
		baseU := p.BaseURL
		if baseU == "" {
			baseU = "(default endpoint)"
		}
		aiLines = append(aiLines, fmt.Sprintf("  Base URL Override: %s", baseU))
		modelU := p.DefaultModel
		if modelU == "" {
			modelU = "(default for type)"
		}
		aiLines = append(aiLines, fmt.Sprintf("  Default Model:     %s", modelU))

		fallbackStr := p.FallbackID
		if fallbackStr == "" {
			fallbackStr = "(none)"
		}
		aiLines = append(aiLines, fmt.Sprintf("  Fallback Chain ID: %s", fallbackStr))

		maxTStr := "Unlimited"
		if p.MaxTokens > 0 {
			maxTStr = fmt.Sprintf("%d", p.MaxTokens)
		}
		aiLines = append(aiLines, fmt.Sprintf("  Max Tokens Limit:  %s", maxTStr))
		aiLines = append(aiLines, fmt.Sprintf("  Total Tokens Used: %d", p.UsedTokens))

		maxRStr := "Unlimited"
		if p.MaxRequests > 0 {
			maxRStr = fmt.Sprintf("%d", p.MaxRequests)
		}
		aiLines = append(aiLines, fmt.Sprintf("  Max Requests Limit: %s", maxRStr))
		aiLines = append(aiLines, fmt.Sprintf("  Total Requests:    %d", p.UsedRequests))

		aiLines = append(aiLines, "", "  [N] Add Provider | [E] Edit | [X] Delete | [p] Toggle Box")
	}

	return boxStyle.Width(mainInnerWidth).Render(strings.Join(aiLines, "\n"))
}

func (m *Model) saveAISettings() {
	pid := strings.TrimSpace(m.inputs[0].Value())
	if pid == "" && len(m.aiProvs) > 0 && m.inputs[0].Placeholder == "Provider ID (Locked)" {
		pid = m.aiProvs[m.aiSelIdx].ID
	}
	if pid == "" {
		return
	}

	pType := strings.TrimSpace(strings.ToLower(m.inputs[1].Value()))
	if pType == "" {
		pType = "openai"
	}
	pName := strings.TrimSpace(m.inputs[2].Value())
	if pName == "" {
		pName = pid
	}
	apiKey := strings.TrimSpace(m.inputs[3].Value())
	baseURL := strings.TrimSpace(m.inputs[4].Value())
	defModel := strings.TrimSpace(m.inputs[5].Value())
	fallback := strings.TrimSpace(m.inputs[6].Value())

	maxT, _ := strconv.ParseInt(strings.TrimSpace(m.inputs[7].Value()), 10, 64)
	maxR, _ := strconv.ParseInt(strings.TrimSpace(m.inputs[8].Value()), 10, 64)

	var usedT, usedR int64
	if existing, err := m.db.GetAIProvider(pid); err == nil {
		usedT = existing.UsedTokens
		usedR = existing.UsedRequests
	}

	prov := storage.AIProvider{
		ID:           pid,
		Type:         pType,
		Name:         pName,
		APIKey:       apiKey,
		BaseURL:      baseURL,
		DefaultModel: defModel,
		FallbackID:   fallback,
		MaxTokens:    maxT,
		UsedTokens:   usedT,
		MaxRequests:  maxR,
		UsedRequests: usedR,
	}
	_ = m.db.SaveAIProvider(prov)
	m.reload()
}

var aiProviderTypes = []string{
	"openai",
	"anthropic",
	"deepseek",
	"openrouter",
	"groq",
	"mistral",
	"perplexity",
	"gemini",
	"cohere",
	"ollama",
}

var aiModels = map[string][]string{
	"openai":     {"gpt-4o-mini", "gpt-4o", "gpt-3.5-turbo", "o1-mini", "o1-preview"},
	"anthropic":  {"claude-3-5-haiku-20241022", "claude-3-5-sonnet-20241022", "claude-3-opus-20240229"},
	"deepseek":   {"deepseek-chat", "deepseek-reasoner"},
	"openrouter": {"google/gemini-2.5-flash", "google/gemini-2.5-pro", "meta-llama/llama-3.3-70b-instruct", "anthropic/claude-3.5-sonnet"},
	"groq":       {"llama-3.3-70b-versatile", "llama3-70b-8192", "mixtral-8x7b-32768", "gemma2-9b-it"},
	"mistral":    {"mistral-small-latest", "mistral-medium-latest", "mistral-large-latest", "open-mistral-7b"},
	"perplexity": {"sonar-reasoning", "sonar", "llama-3.1-sonar-large-128k-online"},
	"gemini":     {"gemini-2.5-flash", "gemini-2.5-pro", "gemini-1.5-flash", "gemini-1.5-pro"},
	"cohere":     {"command-r-plus", "command-r", "command-light"},
	"ollama":     {"llama3", "mistral", "phi3", "gemma"},
}

func cycleString(cur string, choices []string, next bool) string {
	if len(choices) == 0 {
		return cur
	}
	idx := -1
	for i, c := range choices {
		if strings.ToLower(c) == strings.ToLower(cur) {
			idx = i
			break
		}
	}
	if idx == -1 {
		if next {
			return choices[0]
		}
		return choices[len(choices)-1]
	}
	if next {
		idx = (idx + 1) % len(choices)
	} else {
		idx = (idx - 1 + len(choices)) % len(choices)
	}
	return choices[idx]
}
