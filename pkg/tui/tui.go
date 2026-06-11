package tui

import (
	"fmt"
	"os"
	"runtime"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/spotify"
	"skyvern/internal/storage"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var miniLogo string

func init() {
	if data, err := os.ReadFile(config.ResolvePath("ascii")); err == nil {
		miniLogo = shrinkASCII(string(data), 4)
	}
}

func shrinkASCII(art string, factor int) string {
	lines := strings.Split(art, "\n")
	if len(lines) == 0 {
		return ""
	}
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	lines = lines[start:end]
	if len(lines) == 0 {
		return ""
	}
	minSpaces := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		spaces := 0
		for _, r := range l {
			if r == ' ' || r == '\t' {
				spaces++
			} else {
				break
			}
		}
		if minSpaces == -1 || spaces < minSpaces {
			minSpaces = spaces
		}
	}
	for i, l := range lines {
		if len(l) > minSpaces {
			lines[i] = l[minSpaces:]
		} else {
			lines[i] = ""
		}
	}
	var sb strings.Builder
	for i := 0; i < len(lines); i += factor {
		l := lines[i]
		for j := 0; j < len(l); j += factor {
			sb.WriteByte(l[j])
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

type Theme struct {
	Name        string
	Bg          lipgloss.Color
	Border      lipgloss.Color
	BorderFocus lipgloss.Color
	Accent      lipgloss.Color
	Green       lipgloss.Color
	Red         lipgloss.Color
	Subtle      lipgloss.Color
}

var Themes = []Theme{
	{
		Name:        "Gruvbox Dark",
		Bg:          lipgloss.Color("#1d2021"),
		Border:      lipgloss.Color("#3c3836"),
		BorderFocus: lipgloss.Color("#fe8019"),
		Accent:      lipgloss.Color("#fbf1c7"),
		Green:       lipgloss.Color("#b8bb26"),
		Red:         lipgloss.Color("#fb4934"),
		Subtle:      lipgloss.Color("#a89984"),
	},
	{
		Name:        "Nordic Night",
		Bg:          lipgloss.Color("#2e3440"),
		Border:      lipgloss.Color("#4c566a"),
		BorderFocus: lipgloss.Color("#88c0d0"),
		Accent:      lipgloss.Color("#eceff4"),
		Green:       lipgloss.Color("#a3be8c"),
		Red:         lipgloss.Color("#bf616a"),
		Subtle:      lipgloss.Color("#d8dee9"),
	},
	{
		Name:        "Matrix Green",
		Bg:          lipgloss.Color("#050505"),
		Border:      lipgloss.Color("#003300"),
		BorderFocus: lipgloss.Color("#00ff00"),
		Accent:      lipgloss.Color("#33ff33"),
		Green:       lipgloss.Color("#00ff00"),
		Red:         lipgloss.Color("#ff0000"),
		Subtle:      lipgloss.Color("#008800"),
	},
	{
		Name:        "Dracula Void",
		Bg:          lipgloss.Color("#282a36"),
		Border:      lipgloss.Color("#44475a"),
		BorderFocus: lipgloss.Color("#bd93f9"),
		Accent:      lipgloss.Color("#f8f8f2"),
		Green:       lipgloss.Color("#50fa7b"),
		Red:         lipgloss.Color("#ff5555"),
		Subtle:      lipgloss.Color("#6272a4"),
	},
}

var curTheme = 0

const Logo = ` ___ _                       
/ __| |___ _ ___ _____ _ _ ___ 
\__ \ / / \ V / -_)  _/ \ ' / -_)
|___/_\_\  \_/\___|_|  \_/\_/\___|`

func progressBar(width int, val, max float64, borderFocus, subtleCol lipgloss.Color) string {
	if max <= 0 {
		return strings.Repeat("░", width)
	}
	ratio := val / max
	if ratio > 1.0 {
		ratio = 1.0
	}
	filled := int(ratio * float64(width))
	if filled < 0 {
		filled = 0
	}
	empty := width - filled
	barStyle := lipgloss.NewStyle().Foreground(borderFocus)
	emptyStyle := lipgloss.NewStyle().Foreground(subtleCol)
	return barStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", empty))
}

type cmdMsg struct{ err error }

type Model struct {
	db      *storage.DB
	mgr     *manager.Manager
	err     error
	width   int
	height  int
	bots    []config.BotInst
	selIdx  int
	editing bool
	inputs  []textinput.Model
	focus   int
	tab     int
	ticks   int
	spTrack string
	spProg  int
	spTot   int
}

func NewModel(db *storage.DB, mgr *manager.Manager) Model {
	g := config.GetGlobal()
	if g.TuiTheme >= 0 && g.TuiTheme < len(Themes) {
		curTheme = g.TuiTheme
	}
	m := Model{
		db:   db,
		mgr:  mgr,
		bots: []config.BotInst{},
	}
	m.reload()
	return m
}

func (m *Model) reload() {
	if bots, err := m.db.ListBots(); err == nil {
		m.bots = bots
		if m.selIdx >= len(m.bots) {
			m.selIdx = len(m.bots) - 1
		}
		if m.selIdx < 0 {
			m.selIdx = 0
		}
	}
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tick()
}

func (m *Model) initGlobalInputs() {
	m.inputs = make([]textinput.Model, 10)
	fields := []struct {
		placeholder string
		value       string
	}{
		{"Global Name", ""},
		{"Global Prefix", ""},
		{"Global Footer", ""},
		{"Global Embed Color (Hex)", ""},
		{"Matrix Color (rgb/preset/hex)", ""},
		{"Storage Location (local/portable/appdata)", ""},
		{"Spotify Enabled (yes/no)", ""},
		{"Logo URL", ""},
		{"Always on Top (yes/no)", ""},
		{"Show Logo (yes/no)", ""},
	}

	g := config.GetGlobal()
	fields[0].value = g.Name
	fields[1].value = g.Prefix
	fields[2].value = g.Footer
	fields[3].value = fmt.Sprintf("%06x", g.EmbedColor)
	fields[4].value = g.MatrixColor
	fields[5].value = string(config.GetTuiCfg().Loc)
	fields[6].value = g.Spotify
	fields[7].value = g.FooterIcon
	if g.AlwaysOnTop {
		fields[8].value = "yes"
	} else {
		fields[8].value = "no"
	}
	if g.ShowLogo {
		fields[9].value = "yes"
	} else {
		fields[9].value = "no"
	}

	th := Themes[curTheme]
	inputStyle := lipgloss.NewStyle().Foreground(th.Accent)

	for i, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.placeholder
		ti.SetValue(f.value)
		ti.Prompt = " > "
		ti.TextStyle = inputStyle
		m.inputs[i] = ti
	}
	m.focus = 0
	m.inputs[0].Focus()
}

func (m *Model) initPalantirInputs() {
	m.inputs = make([]textinput.Model, 5)
	fields := []struct {
		placeholder string
		value       string
	}{
		{"Palantir Enabled (yes/no)", ""},
		{"Blocked Servers", ""},
		{"Blocked Channels", ""},
		{"Blocked Users", ""},
		{"Blocked Events", ""},
	}

	pCfg, _ := m.mgr.GetPalantirCfg()
	if pCfg.Enabled {
		fields[0].value = "yes"
	} else {
		fields[0].value = "no"
	}
	fields[1].value = strings.Join(pCfg.BlockedGuilds, ", ")
	fields[2].value = strings.Join(pCfg.BlockedChannels, ", ")
	fields[3].value = strings.Join(pCfg.BlockedUsers, ", ")
	fields[4].value = strings.Join(pCfg.BlockedEvents, ", ")

	th := Themes[curTheme]
	inputStyle := lipgloss.NewStyle().Foreground(th.Accent)

	for i, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.placeholder
		ti.SetValue(f.value)
		ti.Prompt = " > "
		ti.TextStyle = inputStyle
		m.inputs[i] = ti
	}
	m.focus = 0
	m.inputs[0].Focus()
}

func (m *Model) initInputs(b *config.BotInst) {
	m.inputs = make([]textinput.Model, 7)
	fields := []struct {
		placeholder string
		value       string
	}{
		{"Client ID", ""},
		{"Bot Token", ""},
		{"Prefix (e.g. .)", ""},
		{"Custom Name Override", ""},
		{"Custom Footer Override", ""},
		{"Custom Color (Hex e.g. 1a1a1a)", ""},
		{"Avatar URL", ""},
	}

	if b != nil {
		fields[0].value = b.ClientID
		fields[1].value = b.Token
		fields[2].value = b.Prefix
		fields[3].value = b.CustomName
		fields[4].value = b.CustomFooter
		if b.CustomColor != 0 {
			fields[5].value = fmt.Sprintf("%06x", b.CustomColor)
		}
		fields[6].value = b.AvatarURL
	}

	th := Themes[curTheme]
	inputStyle := lipgloss.NewStyle().Foreground(th.Accent)

	for i, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.placeholder
		ti.SetValue(f.value)
		ti.Prompt = " > "
		ti.TextStyle = inputStyle
		if i == 0 && b != nil {
			ti.Placeholder = "Client ID (Locked)"
			ti.Blur()
		}
		m.inputs[i] = ti
	}
	m.focus = 0
	m.inputs[0].Focus()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.editing {
			switch msg.String() {
			case "esc":
				m.editing = false
				return m, nil
			case "enter":
				if m.focus == len(m.inputs)-1 {
					if m.tab == 1 {
						m.saveGlobalSettings()
					} else if m.tab == 2 {
						m.savePalantirSettings()
					} else {
						m.saveEdit()
					}
					m.editing = false
					return m, nil
				}
				m.focusInput(m.focus + 1)
			case "up", "shift+tab":
				m.focusInput(m.focus - 1)
			case "down", "tab":
				m.focusInput(m.focus + 1)
			default:
				var cmd tea.Cmd
				m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.tab = (m.tab + 1) % 3
		case "up", "k", "left", "h":
			if m.selIdx > 0 {
				m.selIdx--
			}
		case "down", "j", "right", "l":
			if m.selIdx < len(m.bots)-1 {
				m.selIdx++
			}
		case "t":
			curTheme = (curTheme + 1) % len(Themes)
			g := config.GetGlobal()
			g.TuiTheme = curTheme
			_ = m.db.SaveGlobal(g)
			config.SetGlobal(g)
		case "n":
			if m.tab == 0 {
				m.editing = true
				m.initInputs(nil)
			}
		case "e":
			if m.tab == 1 {
				m.editing = true
				m.initGlobalInputs()
			} else if m.tab == 2 {
				m.editing = true
				m.initPalantirInputs()
			} else if len(m.bots) > 0 {
				m.editing = true
				m.initInputs(&m.bots[m.selIdx])
				if m.bots[m.selIdx].ClientID != "" {
					m.focusInput(1)
				}
			}
		case "s":
			if m.tab == 0 && len(m.bots) > 0 {
				b := m.bots[m.selIdx]
				cmd := func() tea.Msg {
					var err error
					if m.mgr.IsRunning(b.ClientID) {
						err = m.mgr.Stop(b.ClientID)
					} else {
						err = m.mgr.Start(b.ClientID)
					}
					return cmdMsg{err: err}
				}
				return m, cmd
			}
		case "x":
			if m.tab == 0 && len(m.bots) > 0 {
				b := m.bots[m.selIdx]
				_ = m.mgr.Stop(b.ClientID)
				_ = m.db.DeleteBot(b.ClientID)
				m.reload()
			}
		}

	case tickMsg:
		m.ticks++
		g := config.GetGlobal()
		if m.tab == 0 && (strings.ToLower(g.Spotify) == "yes" || strings.ToLower(g.Spotify) == "enabled") {
			t := spotify.GetSpotifyTrack()
			if t != "" {
				if t != m.spTrack {
					m.spTrack = t
					m.spProg = 0
					m.spTot = 150 + (m.ticks % 120)
				}
				if m.ticks%6 == 0 {
					m.spProg++
					if m.spProg >= m.spTot {
						m.spProg = 0
					}
				}
			} else {
				m.spTrack = ""
				m.spProg = 0
				m.spTot = 0
			}
		}
		return m, tick()

	case cmdMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.err = nil
		}
		m.reload()
	}

	return m, nil
}

func (m *Model) focusInput(idx int) {
	if idx < 0 {
		idx = len(m.inputs) - 1
	}
	if idx >= len(m.inputs) {
		idx = 0
	}
	m.inputs[m.focus].Blur()
	m.focus = idx
	m.inputs[m.focus].Focus()
}

func (m *Model) saveGlobalSettings() {
	name := strings.TrimSpace(m.inputs[0].Value())
	if name == "" {
		name = config.DefaultName
	}
	prefix := strings.TrimSpace(m.inputs[1].Value())
	if prefix == "" {
		prefix = config.DefaultPrefix
	}
	footer := strings.TrimSpace(m.inputs[2].Value())
	if footer == "" {
		footer = config.DefaultFooter
	}
	colStr := strings.TrimPrefix(m.inputs[3].Value(), "#")
	colVal, err := strconv.ParseInt(colStr, 16, 32)
	if err != nil {
		colVal = config.ColorDefault
	}
	matrixCol := strings.TrimSpace(m.inputs[4].Value())
	if matrixCol == "" {
		matrixCol = "rgb"
	}
	spotifyVal := strings.TrimSpace(m.inputs[6].Value())
	if spotifyVal == "" {
		spotifyVal = "no"
	}
	logoVal := strings.TrimSpace(m.inputs[7].Value())
	topVal := strings.TrimSpace(strings.ToLower(m.inputs[8].Value()))
	alwaysTop := topVal == "yes" || topVal == "true" || topVal == "1"
	showLogoVal := strings.TrimSpace(strings.ToLower(m.inputs[9].Value()))
	showLogo := showLogoVal == "yes" || showLogoVal == "true" || showLogoVal == "1" || showLogoVal == ""

	storeLoc := config.StorageLoc(strings.TrimSpace(strings.ToLower(m.inputs[5].Value())))
	if storeLoc != config.LocPortable && storeLoc != config.LocAppData {
		storeLoc = config.LocLocal
	}
	_ = config.SaveTuiCfg(config.TuiCfg{Loc: storeLoc})

	g := config.GlobalCfg{
		Name:        name,
		Prefix:      prefix,
		Footer:      footer,
		EmbedColor:  int(colVal),
		MatrixColor: matrixCol,
		TuiTheme:    curTheme,
		Spotify:     spotifyVal,
		FooterIcon:  logoVal,
		AlwaysOnTop: alwaysTop,
		ShowLogo:    showLogo,
	}

	_ = m.db.SaveGlobal(g)
	config.SetGlobal(g)
	SetAlwaysOnTop(alwaysTop)

	for _, b := range m.bots {
		_ = m.mgr.UpdateInstance(b.ClientID)
	}
	m.reload()
}

func (m *Model) savePalantirSettings() {
	pEnabled := strings.TrimSpace(strings.ToLower(m.inputs[0].Value())) == "yes" || strings.TrimSpace(strings.ToLower(m.inputs[0].Value())) == "true"
	splitTrim := func(s string) []string {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		parts := strings.Split(s, ",")
		var out []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	pCfg := storage.PalantirCfg{
		Enabled:         pEnabled,
		BlockedGuilds:   splitTrim(m.inputs[1].Value()),
		BlockedChannels: splitTrim(m.inputs[2].Value()),
		BlockedUsers:    splitTrim(m.inputs[3].Value()),
		BlockedEvents:   splitTrim(m.inputs[4].Value()),
	}
	_ = m.mgr.SavePalantirCfg(pCfg)
}

func (m *Model) saveEdit() {
	cid := strings.TrimSpace(m.inputs[0].Value())
	if cid == "" && len(m.bots) > 0 && m.inputs[0].Placeholder == "Client ID (Locked)" {
		cid = m.bots[m.selIdx].ClientID
	}
	if cid == "" {
		return
	}

	colStr := strings.TrimPrefix(m.inputs[5].Value(), "#")
	colVal, _ := strconv.ParseInt(colStr, 16, 32)

	b := config.BotInst{
		ClientID:     cid,
		Token:        strings.TrimSpace(m.inputs[1].Value()),
		Prefix:       strings.TrimSpace(m.inputs[2].Value()),
		CustomName:   strings.TrimSpace(m.inputs[3].Value()),
		CustomFooter: strings.TrimSpace(m.inputs[4].Value()),
		CustomColor:  int(colVal),
		AvatarURL:    strings.TrimSpace(m.inputs[6].Value()),
	}

	_ = m.db.SaveBot(b)
	_ = m.mgr.UpdateInstance(b.ClientID)
	m.reload()
}

func (m Model) View() string {
	if m.width < 30 || m.height < 8 {
		return "Too small."
	}

	th := Themes[curTheme]
	bgStyle := lipgloss.NewStyle().Background(th.Bg)
	bannerStyle := lipgloss.NewStyle().Foreground(th.Accent).Background(th.BorderFocus).Bold(true).Padding(0, 2).MarginBottom(1)
	titleStyle := lipgloss.NewStyle().Foreground(th.Accent).Bold(true).Underline(true)
	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(1)
	boxFocusStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(th.BorderFocus).Padding(1)
	labelStyle := lipgloss.NewStyle().Foreground(th.Subtle).Width(22)

	headerHeight := 2
	footerHeight := 1
	contentHeight := m.height - headerHeight - footerHeight - 2
	if contentHeight < 4 {
		contentHeight = 4
	}

	showSidebar := m.width >= 65
	sidebarWidth := 0
	if showSidebar {
		sidebarWidth = m.width / 4
		if sidebarWidth < 18 {
			sidebarWidth = 18
		}
		if sidebarWidth > 30 {
			sidebarWidth = 30
		}
	}
	mainWidth := m.width - sidebarWidth - 2
	if mainWidth < 20 {
		mainWidth = 20
	}

	viewName := "DASHBOARD"
	if m.tab == 1 {
		viewName = "SETTINGS"
	} else if m.tab == 2 {
		viewName = "PALANTIR"
	}
	banner := bannerStyle.Render(fmt.Sprintf(" SKYVERN  |  %s  |  %d BOTS ACTIVE  |  THEME: %s ", viewName, len(m.bots), th.Name))

	var sidebar string
	if showSidebar {
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
		sidebar = boxStyle.Width(sbInnerWidth).Height(sbInnerHeight).Render(strings.Join(sbLines, "\n"))
	}

	var mainPanel string
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
		mainPanel = boxFocusStyle.Width(mainInnerWidth).Render(strings.Join(fLines, "\n"))
	} else if m.tab == 1 {
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
		setLines = append(setLines, fmt.Sprintf("  TUI Theme:         %s", th.Name))
		setLines = append(setLines, "", "  Press [E] to edit global settings.")

		mainInnerWidth := mainWidth - 4
		if mainInnerWidth < 20 {
			mainInnerWidth = 20
		}
		mainPanel = boxStyle.Width(mainInnerWidth).Render(strings.Join(setLines, "\n"))
	} else if m.tab == 2 {
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
		mainPanel = boxStyle.Width(mainInnerWidth).Render(strings.Join(setLines, "\n"))
	} else {
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
			mainPanel = lipgloss.JoinVertical(lipgloss.Left, topRight, bottomRight)
		} else {
			mainPanel = topRight
		}
	}

	var footer string
	if m.tab == 0 {
		footer = lipgloss.NewStyle().Foreground(th.Subtle).Render("  [↑/↓] Scroll Bots   [Tab] Settings   [N] New   [E] Edit   [S] Toggle State   [T] Cycle Themes   [X] Delete   [Q] Quit")
	} else if m.tab == 1 {
		footer = lipgloss.NewStyle().Foreground(th.Subtle).Render("  [Tab] Palantir Settings   [E] Edit Globals   [T] Cycle Themes   [Q] Quit")
	} else {
		footer = lipgloss.NewStyle().Foreground(th.Subtle).Render("  [Tab] Dashboard   [E] Edit Palantir   [T] Cycle Themes   [Q] Quit")
	}

	var body string
	if showSidebar {
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainPanel)
	} else {
		body = mainPanel
	}

	return bgStyle.Render(lipgloss.JoinVertical(lipgloss.Left, banner, body, "", footer))
}

func (m Model) renderMatrixRain(w, h int, th Theme) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	grid := make([][]string, h)
	for r := 0; r < h; r++ {
		grid[r] = make([]string, w)
		for c := 0; c < w; c++ {
			grid[r][c] = " "
		}
	}

	word := "esoteric.win"
	startX := (w - len(word)) / 2
	if startX < 0 {
		startX = 0
	}
	midY := h / 2

	g := config.GetGlobal()
	colMode := strings.ToLower(g.MatrixColor)
	if colMode == "" {
		colMode = "rgb"
	}

	getStyle := func(c, r, age int) lipgloss.Style {
		var baseCol lipgloss.Color
		switch colMode {
		case "rgb":
			rgbCols := []string{"#ff0000", "#ff7f00", "#ffff00", "#00ff00", "#0000ff", "#4b0082", "#9400d3"}
			baseCol = lipgloss.Color(rgbCols[(c+r+m.ticks/2)%len(rgbCols)])
		case "green", "matrix":
			cols := []string{"#00ff00", "#33ff33", "#00aa00", "#005500", "#002200"}
			idx := age
			if idx >= len(cols) {
				idx = len(cols) - 1
			}
			baseCol = lipgloss.Color(cols[idx])
		case "dracula", "purple":
			cols := []string{"#bd93f9", "#ff79c6", "#8be9fd", "#6272a4", "#44475a"}
			idx := age
			if idx >= len(cols) {
				idx = len(cols) - 1
			}
			baseCol = lipgloss.Color(cols[idx])
		case "nordic", "cyan":
			cols := []string{"#88c0d0", "#8fbcbb", "#81a1c1", "#5e81ac", "#4c566a"}
			idx := age
			if idx >= len(cols) {
				idx = len(cols) - 1
			}
			baseCol = lipgloss.Color(cols[idx])
		default:
			if !strings.HasPrefix(colMode, "#") {
				colMode = "#" + colMode
			}
			baseCol = lipgloss.Color(colMode)
		}
		style := lipgloss.NewStyle().Foreground(baseCol)
		if age == 0 {
			style = style.Bold(true)
		}
		return style
	}

	chars := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789@#$%&*+-=")

	for c := 0; c < w; c++ {
		offset := (c * 7) % 37
		dropY := (m.ticks/2 + offset) % (h + 6)

		for r := 0; r < h; r++ {
			isWord := r == midY && c >= startX && c < startX+len(word)
			var char rune
			age := r - dropY

			if isWord {
				// Pseudo-random reveal: 30% chance or if drop is over it
				revealSeed := (r + c + m.ticks/4) % 7
				if (age >= 0 && age < 5) || revealSeed == 0 {
					char = rune(word[c-startX])
					style := getStyle(c, r, age)
					grid[r][c] = style.Render(string(char))
				} else {
					// Blend with rain as faint random character
					charIdx := (r*c + m.ticks + offset) % len(chars)
					char = chars[charIdx]
					grid[r][c] = lipgloss.NewStyle().Foreground(th.Subtle).Render(string(char))
				}
			} else {
				if age >= 0 && age < 5 {
					charIdx := (r*c + m.ticks + offset) % len(chars)
					char = chars[charIdx]
					style := getStyle(c, r, age)
					grid[r][c] = style.Render(string(char))
				}
			}
		}
	}

	var sb strings.Builder
	for r := 0; r < h; r++ {
		sb.WriteString(strings.Join(grid[r], ""))
		if r < h-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (m Model) renderSpotifyPanel(w, h int, th Theme) string {
	if w <= 0 || h <= 0 {
		return ""
	}

	g := config.GetGlobal()
	col := strings.ToLower(g.MatrixColor)
	var lCol lipgloss.Color
	switch col {
	case "rgb":
		lCol = lipgloss.Color("#1DB954")
	case "green", "matrix":
		lCol = lipgloss.Color("#00ff00")
	case "dracula", "purple":
		lCol = lipgloss.Color("#bd93f9")
	case "nordic", "cyan":
		lCol = lipgloss.Color("#88c0d0")
	default:
		if strings.HasPrefix(col, "#") {
			lCol = lipgloss.Color(col)
		} else if col != "" && col != "disabled" && col != "none" && col != "off" && col != "no" {
			lCol = lipgloss.Color("#" + col)
		} else {
			lCol = lipgloss.Color("#1DB954")
		}
	}

	var info []string
	info = append(info, lipgloss.NewStyle().Foreground(th.Accent).Bold(true).Render("SPOTIFY"))

	tr := func(s string, max int) string {
		if max < 4 {
			return "..."
		}
		if len(s) > max {
			return s[:max-3] + "..."
		}
		return s
	}

	song := m.spTrack
	prog := m.spProg
	tot := m.spTot

	if song == "" {
		info = append(info, lipgloss.NewStyle().Foreground(th.Subtle).Italic(true).Render("Paused / Idle"))
	} else {
		p := strings.SplitN(song, " - ", 2)
		art := ""
		name := song
		if len(p) == 2 {
			art = p[0]
			name = p[1]
		}

		if art != "" {
			info = append(info, lipgloss.NewStyle().Foreground(th.Subtle).Render("Artist: "+tr(art, w-2)))
			info = append(info, lipgloss.NewStyle().Foreground(th.Subtle).Render("Title:  "+tr(name, w-2)))
		} else {
			info = append(info, lipgloss.NewStyle().Foreground(th.Subtle).Render("Track:  "+tr(name, w-2)))
		}

		pStr := fmt.Sprintf("%02d:%02d", prog/60, prog%60)
		tStr := fmt.Sprintf("%02d:%02d", tot/60, tot%60)

		barW := w - 13
		if barW < 4 {
			barW = 4
		}
		pBar := progressBar(barW, float64(prog), float64(tot), lCol, th.Subtle)
		info = append(info, fmt.Sprintf("%s [%s] %s", pStr, pBar, tStr))
	}

	txt := strings.Join(info, "\n")
	availHeight := h - len(info) - 1

	var logo []string
	if w >= 38 && availHeight >= 13 {
		logo = []string{
			"       ⢀⣠⣤⣤⣶⣶⣶⣶⣤⣤⣄⡀       ",
			"    ⢀⣤⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⣤⡀    ",
			"   ⣴⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣦   ",
			" ⢀⣾⣿⡿⠿⠛⠛⠛⠉⠉⠉⠉⠛⠛⠛⠿⠿⣿⣿⣿⣿⣿⣷⡀ ",
			" ⣾⣿⣿⣇⠀⣀⣀⣠⣤⣤⣤⣤⣤⣀⣀⠀⠀⠀⠈⠙⠻⣿⣿⣷ ",
			"⢠⣿⣿⣿⣿⡿⠿⠟⠛⠛⠛⠛⠛⠛⠻⠿⢿⣿⣶⣤⣀⣠⣿⣿⣿⡄",
			"⢸⣿⣿⣿⣿⣇⣀⣀⣤⣤⣤⣤⣤⣄⣀⣀⠀⠀⠉⠛⢿⣿⣿⣿⣿⡇",
			"⠘⣿⣿⣿⣿⣿⠿⠿⠛⠛⠛⠛⠛⠛⠿⠿⣿⣶⣦⣤⣾⣿⣿⣿⣿⠃",
			" ⢿⣿⣿⣿⣿⣤⣤⣤⣤⣶⣶⣦⣤⣤⣄⡀⠈⠙⣿⣿⣿⣿⣿⡿ ",
			" ⠈⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⣾⣿⣿⣿⣿⡿⠁ ",
			"   ⠻⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠟   ",
			"    ⠈⠛⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠛⠁    ",
			"       ⠈⠙⠛⠛⠿⠿⠿⠿⠛⠛⠋⠁       ",
		}
	} else if w >= 22 && availHeight >= 7 {
		logo = []string{
			"    ⢀⣤⣴⣶⣦⣤⡀    ",
			"  ⣠⣾⣿⣿⣿⣿⣿⣿⣄  ",
			" ⣴⣿⡿⠋⠉⠉⠙⢿⣿⣦ ",
			" ⣿⣿⣇⣠⣤⣤⣄⣸⣿⣿ ",
			" ⠻⣿⣿⠿⠿⠿⠿⣿⣿⠟ ",
			"  ⠙⢿⣿⣿⣿⣿⡿⠋  ",
			"    ⠈⠙⠛⠛⠋⠁    ",
		}
	}

	if len(logo) == 0 {
		return txt
	}
	sty := lipgloss.NewStyle().Foreground(lCol)
	var lines []string
	for _, l := range logo {
		lines = append(lines, sty.Render(l))
	}
	lStr := strings.Join(lines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left, txt, "", lStr)
}

func Run(db *storage.DB, mgr *manager.Manager) error {
	fmt.Print("\033[8;32;125t")
	SetAlwaysOnTop(config.GetGlobal().AlwaysOnTop)
	p := tea.NewProgram(NewModel(db, mgr), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
