//go:build linux
package spotify

import (
	"bytes"
	"os/exec"
	"strings"
)

func GetSpotifyTrack() string {
	// playerctl handles MPRIS players cleanly
	if p, err := exec.LookPath("playerctl"); err == nil {
		c := exec.Command(p, "-p", "spotify", "metadata", "--format", "{{artist}} - {{title}}")
		var buf bytes.Buffer
		c.Stdout = &buf
		if c.Run() == nil {
			t := strings.TrimSpace(buf.String())
			if t != "" && t != "-" {
				return t
			}
		}

		// check browsers too if they're holding active tabs
		c = exec.Command(p, "-a", "metadata", "--format", "{{playerName}}::{{artist}}::{{title}}")
		buf.Reset()
		c.Stdout = &buf
		if c.Run() == nil {
			lns := strings.Split(buf.String(), "\n")
			for _, l := range lns {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				pParts := strings.Split(l, "::")
				if len(pParts) < 3 {
					continue
				}
				name := strings.ToLower(pParts[0])
				art := strings.TrimSpace(pParts[1])
				t := strings.TrimSpace(pParts[2])
				if strings.Contains(name, "chrome") || strings.Contains(name, "firefox") || strings.Contains(name, "chromium") || strings.Contains(name, "brave") {
					if art != "" && t != "" {
						return art + " - " + t
					}
				}
			}
		}
	}

	// fallback directly to dbus if playerctl isn't installed
	if p, err := exec.LookPath("dbus-send"); err == nil {
		c := exec.Command(p, "--print-reply", "--dest=org.mpris.MediaPlayer2.spotify", "/org/mpris/MediaPlayer2", "org.freedesktop.DBus.Properties.Get", "string:org.mpris.MediaPlayer2.Player", "string:Metadata")
		var buf bytes.Buffer
		c.Stdout = &buf
		if c.Run() == nil {
			out := buf.String()
			art := parseDbus(out, "xesam:artist")
			t := parseDbus(out, "xesam:title")
			if t != "" {
				if art != "" {
					return art + " - " + t
				}
				return t
			}
		}
	}

	return ""
}

func parseDbus(out, key string) string {
	lns := strings.Split(out, "\n")
	for i, l := range lns {
		if !strings.Contains(l, key) {
			continue
		}
		for j := i + 1; j < len(lns) && j < i+4; j++ {
			vl := lns[j]
			if !strings.Contains(vl, "variant") && !strings.Contains(vl, "string") {
				continue
			}
			idx := strings.Index(vl, "string \"")
			if idx == -1 {
				continue
			}
			val := vl[idx+8:]
			if len(val) > 0 && val[len(val)-1] == '"' {
				val = val[:len(val)-1]
			}
			return val
		}
	}
	return ""
}
