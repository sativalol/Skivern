package tui

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/storage"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

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
