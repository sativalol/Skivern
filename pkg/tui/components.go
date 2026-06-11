package tui

import (
	"fmt"
	"os"
	"skyvern/internal/config"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var miniLogo string

func init() {
	if data, err := os.ReadFile(config.ResolvePath("ascii")); err == nil {
		miniLogo = Shrink(string(data), 4)
	}
}

func Shrink(art string, factor int) string {
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
