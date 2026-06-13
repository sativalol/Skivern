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
	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(1)

	sbInnerWidth, sbInnerHeight := calcInnerLimits(sidebarWidth, contentHeight)

	var sbLines []string
	if miniLogo != "" && sbInnerHeight > 15 {
		sbLines = append(sbLines, lipgloss.NewStyle().Foreground(th.BorderFocus).Render(miniLogo))
		sbLines = append(sbLines, "")
	}
	sbLines = append(sbLines, titleStyle.Render("AI PROVIDERS"))
	sbLines = append(sbLines, "")

	maxVisible := calcMaxVisible(sbInnerHeight)
	startIdx, endIdx := calcVisibleRange(len(m.aiProvs), m.aiSelIdx, maxVisible)
	for i := startIdx; i < endIdx; i++ {
		p := m.aiProvs[i]
		lbl := p.ID
		line := fmt.Sprintf("  %-15s", lbl)
		if i == m.aiSelIdx {
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

	var aiLines []string
	aiLines = append(aiLines, titleStyle.Render("AI CONFIGURATION"))
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

		aiLines = append(aiLines, "", "  Press [N] to add a new provider, [E] to edit, or [X] to delete.")
	}

	mainInnerWidth := calcMainInnerWidth(mainWidth)
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
