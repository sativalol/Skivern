package tui

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/spotify"
	"skyvern/internal/storage"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
			m.tab = (m.tab + 1) % 4
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
			// using the internal background tracking for simulated progress
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

func (m Model) View() string {
	if m.width < 30 || m.height < 8 {
		return "Too small."
	}

	th := Themes[curTheme]
	bgStyle := lipgloss.NewStyle().Background(th.Bg)
	bannerStyle := lipgloss.NewStyle().Foreground(th.Accent).Background(th.BorderFocus).Bold(true).Padding(0, 2).MarginBottom(1)

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
	} else if m.tab == 3 {
		viewName = "LAVALINK"
	}
	banner := bannerStyle.Render(fmt.Sprintf(" SKYVERN  |  %s  |  %d BOTS ACTIVE  |  THEME: %s ", viewName, len(m.bots), th.Name))

	var sidebar string
	if showSidebar {
		sidebar = m.renderSidebar(contentHeight, sidebarWidth, th)
	}

	mainPanel := m.renderMainPanel(mainWidth, contentHeight, th)

	var footer string
	if m.tab == 0 {
		footer = lipgloss.NewStyle().Foreground(th.Subtle).Render("  [↑/↓] Scroll Bots   [Tab] Settings   [N] New   [E] Edit   [S] Toggle State   [T] Cycle Themes   [X] Delete   [Q] Quit")
	} else if m.tab == 1 {
		footer = lipgloss.NewStyle().Foreground(th.Subtle).Render("  [Tab] Palantir Settings   [E] Edit Globals   [T] Cycle Themes   [Q] Quit")
	} else if m.tab == 2 {
		footer = lipgloss.NewStyle().Foreground(th.Subtle).Render("  [Tab] Lavalink Status   [E] Edit Palantir   [T] Cycle Themes   [Q] Quit")
	} else {
		footer = lipgloss.NewStyle().Foreground(th.Subtle).Render("  [Tab] Dashboard   [T] Cycle Themes   [Q] Quit")
	}

	var body string
	if showSidebar {
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainPanel)
	} else {
		body = mainPanel
	}

	return bgStyle.Render(lipgloss.JoinVertical(lipgloss.Left, banner, body, "", footer))
}

func Run(db *storage.DB, mgr *manager.Manager) error {
	fmt.Print("\033[8;32;125t")
	SetAlwaysOnTop(config.GetGlobal().AlwaysOnTop)
	p := tea.NewProgram(NewModel(db, mgr), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
