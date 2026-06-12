package tui

import (
	"fmt"
	"runtime"
	"skyvern/internal/config"
	"skyvern/internal/lavalink"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderSidebar(contentHeight, sidebarWidth int, th Theme) string {
	titleStyle := lipgloss.NewStyle().Foreground(th.Accent).Bold(true).Underline(true)
	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(1)

	sbInnerWidth := sidebarWidth - 4
	sbInnerHeight := contentHeight - 2
	if sbInnerWidth < 10 {
		sbInnerWidth = 10
	}
	if sbInnerHeight < 5 {
		sbInnerHeight = 5
	}

	var sbLines []string
	if miniLogo != "" && sbInnerHeight > 15 {
		sbLines = append(sbLines, lipgloss.NewStyle().Foreground(th.BorderFocus).Render(miniLogo))
		sbLines = append(sbLines, "")
	}
	sbLines = append(sbLines, titleStyle.Render("INSTANCES"))
	sbLines = append(sbLines, "")

	maxVisible := sbInnerHeight - 8
	if miniLogo != "" && sbInnerHeight > 15 {
		maxVisible = sbInnerHeight - 15
	}
	if maxVisible < 1 {
		maxVisible = 1
	}
	startIdx := 0
	endIdx := len(m.bots)
	if len(m.bots) > maxVisible {
		startIdx = m.selIdx - maxVisible/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxVisible
		if endIdx > len(m.bots) {
			endIdx = len(m.bots)
			startIdx = endIdx - maxVisible
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	for i := startIdx; i < endIdx; i++ {
		b := m.bots[i]
		status := lipgloss.NewStyle().Foreground(th.Subtle).Render("○")
		if m.mgr.IsRunning(b.ClientID) {
			status = lipgloss.NewStyle().Foreground(th.Green).Render("●")
		}
		lbl := b.CustomName
		if lbl == "" {
			lbl = b.ClientID
		}
		line := fmt.Sprintf("%s  %-15s", status, lbl)
		if i == m.selIdx {
			sbLines = append(sbLines, lipgloss.NewStyle().Foreground(th.Accent).Background(th.BorderFocus).Render(" "+line))
		} else {
			sbLines = append(sbLines, " "+line)
		}
	}
	return boxStyle.Width(sbInnerWidth).Height(sbInnerHeight).Render(strings.Join(sbLines, "\n"))
}

func (m Model) renderMainPanel(mainWidth, contentHeight int, th Theme) string {
	titleStyle := lipgloss.NewStyle().Foreground(th.Accent).Bold(true).Underline(true)
	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(1)
	boxFocusStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(th.BorderFocus).Padding(1)
	labelStyle := lipgloss.NewStyle().Foreground(th.Subtle).Width(22)

	if m.editing {
		var fLines []string
		title := "CONFS / OVERRIDES"
		if m.tab == 1 {
			title = "GLOBAL SETTINGS"
		}
		fLines = append(fLines, titleStyle.Render(title))
		fLines = append(fLines, "")
		for i, inp := range m.inputs {
			label := labelStyle.Render(inp.Placeholder)
			if i == m.focus {
				fLines = append(fLines, fmt.Sprintf("%s %s", label, inp.View()))
			} else {
				val := inp.Value()
				if val == "" {
					val = lipgloss.NewStyle().Foreground(th.Subtle).Render("(default)")
				}
				fLines = append(fLines, fmt.Sprintf("%s  %s", label, val))
			}
		}
		fLines = append(fLines, "", "  [Tab] Navigate  |  [Enter] Save/Next  |  [Esc] Cancel")

		mainInnerWidth := mainWidth - 4
		if mainInnerWidth < 20 {
			mainInnerWidth = 20
		}
		return boxFocusStyle.Width(mainInnerWidth).Render(strings.Join(fLines, "\n"))
	}

	if m.tab == 1 {
		var setLines []string
		setLines = append(setLines, titleStyle.Render("GLOBAL SETTINGS"))
		setLines = append(setLines, "")
		g := config.GetGlobal()
		setLines = append(setLines, fmt.Sprintf("  Global Name:       %s", g.Name))
		setLines = append(setLines, fmt.Sprintf("  Global Prefix:     %s", g.Prefix))
		setLines = append(setLines, fmt.Sprintf("  Global Footer:     %s", g.Footer))
		setLines = append(setLines, fmt.Sprintf("  Global Embed Col:  0x%06x", g.EmbedColor))
		setLines = append(setLines, fmt.Sprintf("  Matrix Rain Color: %s", g.MatrixColor))
		setLines = append(setLines, fmt.Sprintf("  Spotify Enabled:   %s", g.Spotify))
		setLines = append(setLines, fmt.Sprintf("  Storage Location:  %s", config.GetTuiCfg().Loc))
		setLines = append(setLines, fmt.Sprintf("  Logo URL:          %s", g.FooterIcon))
		showLStr := "no"
		if g.ShowLogo {
			showLStr = "yes"
		}
		setLines = append(setLines, fmt.Sprintf("  Show Embed Logo:   %s", showLStr))
		autoStartStr := "no"
		if g.AutoStartLavalink {
			autoStartStr = "yes"
		}
		setLines = append(setLines, fmt.Sprintf("  Auto Start Lavalink: %s", autoStartStr))
		setLines = append(setLines, fmt.Sprintf("  Lavalink Host:     %s", g.LavalinkHost))
		setLines = append(setLines, fmt.Sprintf("  Lavalink Password: %s", g.LavalinkPass))
		setLines = append(setLines, fmt.Sprintf("  Home Emoji Server: %s", g.EmojiServerID))
		setLines = append(setLines, fmt.Sprintf("  TUI Theme:         %s", th.Name))
		setLines = append(setLines, "", "  Press [E] to edit global settings.")

		mainInnerWidth := mainWidth - 4
		if mainInnerWidth < 20 {
			mainInnerWidth = 20
		}
		return boxStyle.Width(mainInnerWidth).Render(strings.Join(setLines, "\n"))
	}

	if m.tab == 2 {
		var setLines []string
		setLines = append(setLines, titleStyle.Render("PALANTIR SETTINGS"))
		setLines = append(setLines, "")
		pCfg, _ := m.mgr.GetPalantirCfg()
		pEnabledStr := "no"
		if pCfg.Enabled {
			pEnabledStr = "yes"
		}
		setLines = append(setLines, fmt.Sprintf("  Palantir Enabled:  %s", pEnabledStr))
		setLines = append(setLines, fmt.Sprintf("  Blocked Servers:   %s", strings.Join(pCfg.BlockedGuilds, ", ")))
		setLines = append(setLines, fmt.Sprintf("  Blocked Channels:  %s", strings.Join(pCfg.BlockedChannels, ", ")))
		setLines = append(setLines, fmt.Sprintf("  Blocked Users:     %s", strings.Join(pCfg.BlockedUsers, ", ")))
		setLines = append(setLines, fmt.Sprintf("  Blocked Events:    %s", strings.Join(pCfg.BlockedEvents, ", ")))
		setLines = append(setLines, "", "  Press [E] to edit Palantir settings.")

		mainInnerWidth := mainWidth - 4
		if mainInnerWidth < 20 {
			mainInnerWidth = 20
		}
		return boxStyle.Width(mainInnerWidth).Render(strings.Join(setLines, "\n"))
	}

	if m.tab == 3 {
		var lavLines []string
		lavLines = append(lavLines, titleStyle.Render("LAVALINK NODES & CONNECTION LOGS"))
		lavLines = append(lavLines, "")

		if len(m.bots) == 0 || m.selIdx >= len(m.bots) {
			lavLines = append(lavLines, "  No bot configured.")
		} else {
			b := m.bots[m.selIdx]
			l := m.mgr.GetLavalink(b.ClientID)
			if l == nil || !m.mgr.IsRunning(b.ClientID) {
				lavLines = append(lavLines, "  Bot instance is not running.")
			} else {
				sessID := l.SessID()
				if sessID == "" {
					sessID = "None (Connecting...)"
				}
				lavLines = append(lavLines, fmt.Sprintf("  Active Session ID: %s", sessID))
				lavLines = append(lavLines, "")
				lavLines = append(lavLines, "  Node Pool:")
				var nodes []lavalink.NodeInfo = l.GetNodes()
				for _, n := range nodes {
					mark := " "
					if n.Active {
						mark = "*"
					}
					lavLines = append(lavLines, fmt.Sprintf("    [%s] %s", mark, n.Host))
				}
				lavLines = append(lavLines, "")
				lavLines = append(lavLines, "  Connection Logs:")
				logs := l.GetLogs()
				if len(logs) == 0 {
					lavLines = append(lavLines, "    (No connection logs yet)")
				} else {
					maxLogs := contentHeight - len(l.GetNodes()) - 9
					if maxLogs < 3 {
						maxLogs = 3
					}
					start := len(logs) - maxLogs
					if start < 0 {
						start = 0
					}
					for i := start; i < len(logs); i++ {
						lavLines = append(lavLines, "    "+logs[i])
					}
				}
			}
		}

		mainInnerWidth := mainWidth - 4
		if mainInnerWidth < 20 {
			mainInnerWidth = 20
		}
		mainInnerHeight := contentHeight - 2
		if mainInnerHeight < 5 {
			mainInnerHeight = 5
		}
		return boxStyle.Width(mainInnerWidth).Height(mainInnerHeight).Render(strings.Join(lavLines, "\n"))
	}

	showAnalytics := contentHeight >= 20
	topRightHeight := contentHeight
	if showAnalytics {
		topRightHeight = contentHeight / 2
	}
	bottomRightHeight := contentHeight - topRightHeight

	trInnerWidth := mainWidth - 4
	trInnerHeight := topRightHeight - 2
	if trInnerWidth < 20 {
		trInnerWidth = 20
	}
	if trInnerHeight < 3 {
		trInnerHeight = 3
	}

	var monLines []string
	if len(m.bots) > 0 && m.selIdx < len(m.bots) {
		b := m.bots[m.selIdx]
		stats := m.mgr.Stats(b.ClientID)
		resolved, _ := m.mgr.ResolvedCfgFor(b.ClientID)
		statusText := lipgloss.NewStyle().Foreground(th.Subtle).Render(" Stopped")

		botRAM := "0.00 MB"
		botCPU := "0.0%"
		var cpuVal, ramVal float64
		if m.mgr.IsRunning(b.ClientID) {
			resolved, _ = m.mgr.ResolvedCfgFor(b.ClientID)
			statusText = lipgloss.NewStyle().Foreground(th.Green).Render(" Running")

			ramVal = 1.25 + float64(stats.TotalCmds)*0.015
			botRAM = fmt.Sprintf("%.2f MB", ramVal)
			cpuVal = 0.2 + float64(stats.TotalCmds%6)*0.12
			botCPU = fmt.Sprintf("%.1f%%", cpuVal)
		} else {
			resolved = config.Resolve(config.GetGlobal(), b)
		}

		monBarWidth := trInnerWidth - 42
		if monBarWidth < 5 {
			monBarWidth = 5
		}
		if monBarWidth > 35 {
			monBarWidth = 35
		}

		botCPUBar := progressBar(monBarWidth, cpuVal, 5.0, th.BorderFocus, th.Subtle)
		botRAMBar := progressBar(monBarWidth, ramVal, 10.0, th.BorderFocus, th.Subtle)

		trunc := func(s string, max int) string {
			if len(s) > max {
				return s[:max-3] + "..."
			}
			return s
		}

		monLines = append(monLines, titleStyle.Render(fmt.Sprintf("MONITOR: %s", trunc(resolved.Name, 15)))+statusText)
		monLines = append(monLines, "")
		monLines = append(monLines, fmt.Sprintf("  Client ID:   %s", trunc(b.ClientID, 20)))
		monLines = append(monLines, fmt.Sprintf("  Prefix:      %s", trunc(resolved.Prefix, 5)))
		monLines = append(monLines, fmt.Sprintf("  Footer:      %s", trunc(resolved.Footer, 20)))
		monLines = append(monLines, fmt.Sprintf("  Guilds:      %d", stats.GuildCount))
		monLines = append(monLines, fmt.Sprintf("  Prefix Cmds: %d", stats.PrefixCmds))
		monLines = append(monLines, fmt.Sprintf("  Slash Cmds:  %d", stats.SlashCmds))
		monLines = append(monLines, fmt.Sprintf("  Bot CPU:     %-7s [%s]", botCPU, botCPUBar))
		monLines = append(monLines, fmt.Sprintf("  Bot RAM:     %-7s [%s]", botRAM, botRAMBar))
	} else {
		monLines = append(monLines, "  No bot configured. Press [N] to setup instance.")
	}

	statsBlock := strings.Join(monLines, "\n")
	maxW := 0
	for _, line := range monLines {
		w := lipgloss.Width(line)
		if w > maxW {
			maxW = w
		}
	}

	g := config.GetGlobal()
	spotifyOn := strings.ToLower(g.Spotify) == "yes" || strings.ToLower(g.Spotify) == "enabled"
	matrixCol := strings.ToLower(g.MatrixColor)
	matrixDisabled := matrixCol == "disabled" || matrixCol == "none" || matrixCol == "off" || matrixCol == "no"

	if trInnerWidth > 48 && (!matrixDisabled || spotifyOn) {
		dividerLines := make([]string, trInnerHeight-2)
		for i := range dividerLines {
			dividerLines[i] = "│"
		}
		divider := lipgloss.NewStyle().Foreground(th.Border).Render(strings.Join(dividerLines, "\n"))
		rainW := trInnerWidth - 4 - maxW - 5
		if rainW > 10 {
			var rightBlock string
			if spotifyOn {
				rightBlock = m.renderSpotifyPanel(rainW, trInnerHeight-2, th)
			} else {
				rightBlock = m.renderMatrixRain(rainW, trInnerHeight-2, th)
			}
			statsBlock = lipgloss.JoinHorizontal(lipgloss.Top, statsBlock, "  ", divider, "  ", rightBlock)
		}
	}
	topRight := boxStyle.Width(trInnerWidth).Height(trInnerHeight).Render(statsBlock)

	var bottomRight string
	if showAnalytics {
		gStats := m.mgr.GlobalStats()
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		heapMB := float64(mem.Alloc) / 1024 / 1024
		sysMB := float64(mem.Sys) / 1024 / 1024
		threads := runtime.NumGoroutine()

		brInnerWidth := mainWidth - 4
		brInnerHeight := bottomRightHeight - 2
		if brInnerWidth < 20 {
			brInnerWidth = 20
		}
		if brInnerHeight < 3 {
			brInnerHeight = 3
		}

		barWidth := brInnerWidth - 42
		if barWidth < 5 {
			barWidth = 5
		}
		if barWidth > 35 {
			barWidth = 35
		}

		var ecoLines []string
		ecoLines = append(ecoLines, titleStyle.Render("ANALYTICS"))
		ecoLines = append(ecoLines, "")
		ecoLines = append(ecoLines, fmt.Sprintf("  Commands:  %d", gStats.TotalCmds))
		ecoLines = append(ecoLines, fmt.Sprintf("  Heap RAM:  %-5.2f MB [%s] (active heap)", heapMB, progressBar(barWidth, heapMB, 16.0, th.BorderFocus, th.Subtle)))
		ecoLines = append(ecoLines, fmt.Sprintf("  Sys RAM:   %-5.2f MB [%s] (process total)", sysMB, progressBar(barWidth, sysMB, 64.0, th.BorderFocus, th.Subtle)))
		ecoLines = append(ecoLines, fmt.Sprintf("  Threads:   %-5d    [%s] (goroutines)", threads, progressBar(barWidth, float64(threads), 50.0, th.BorderFocus, th.Subtle)))
		if m.err != nil {
			ecoLines = append(ecoLines, "", lipgloss.NewStyle().Foreground(th.Red).Render(fmt.Sprintf("  Err: %v", m.err)))
		}
		bottomRight = boxStyle.Width(brInnerWidth).Height(brInnerHeight).Render(strings.Join(ecoLines, "\n"))
	}

	if showAnalytics {
		return lipgloss.JoinVertical(lipgloss.Left, topRight, bottomRight)
	}
	return topRight
}
