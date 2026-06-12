package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const (
	ColorDefault  = 0x1a1a1a
	ColorBlack    = 0x0d0d0d
	ColorGunmetal = 0x2c2f33
	ColorWhite    = 0xffffff

	DefaultName   = "Skivern"
	DefaultPrefix = "."
	DefaultFooter = "esoteric.win"
)

type GlobalCfg struct {
	Name              string `json:"name"`
	Prefix            string `json:"prefix"`
	Footer            string `json:"footer"`
	EmbedColor        int    `json:"embed_color"`
	AvatarURL         string `json:"avatar_url,omitempty"`
	FooterIcon        string `json:"footer_icon,omitempty"`
	TuiTheme          int    `json:"tui_theme"`
	MatrixColor       string `json:"matrix_color"`
	Spotify           string `json:"spotify"`
	AlwaysOnTop       bool   `json:"always_on_top"`
	ShowLogo          bool   `json:"show_logo"`
	AutoStartLavalink bool   `json:"auto_start_lavalink"`
	LavalinkHost      string `json:"lavalink_host,omitempty"`
	LavalinkPass      string `json:"lavalink_pass,omitempty"`
	EmojiServerID     string `json:"emoji_server_id,omitempty"`
}

type BotInst struct {
	ClientID  string `json:"client_id"`
	Token     string `json:"token"`
	Prefix    string `json:"prefix,omitempty"`
	IsEnabled bool   `json:"is_enabled"`

	CustomName   string `json:"custom_name,omitempty"`
	CustomFooter string `json:"custom_footer,omitempty"`
	CustomColor  int `json:"custom_color,omitempty"`
	AvatarURL    string `json:"avatar_url,omitempty"`
	FooterIcon   string `json:"footer_icon,omitempty"`
}

type ResCfg struct {
	Name       string
	Prefix     string
	Footer     string
	EmbedColor int
	AvatarURL  string
	FooterIcon string
	ShowLogo   bool
	SuccessSym string
	InfoSym    string
	WarnSym    string
	ErrSym     string
}

func Resolve(g GlobalCfg, inst BotInst) ResCfg {
	r := ResCfg{
		Name:       g.Name,
		Prefix:     g.Prefix,
		Footer:     g.Footer,
		EmbedColor: g.EmbedColor,
		AvatarURL:  g.AvatarURL,
		ShowLogo:    g.ShowLogo,
		SuccessSym:  "[+]",
		InfoSym:     "[*]",
		WarnSym:     "[!]",
		ErrSym:      "[-]",
	}
	if inst.CustomName != "" {
		r.Name = inst.CustomName
	}
	if inst.Prefix != "" {
		r.Prefix = inst.Prefix
	}
	if inst.CustomFooter != "" {
		r.Footer = inst.CustomFooter
	}
	if inst.CustomColor != 0 {
		r.EmbedColor = inst.CustomColor
	}
	if inst.AvatarURL != "" {
		r.AvatarURL = inst.AvatarURL
	}
	if inst.FooterIcon != "" {
		r.FooterIcon = inst.FooterIcon
	}
	if r.FooterIcon == "" {
		r.FooterIcon = "https://files.catbox.moe/xxv6qt.webp"
	}
	return r
}

func DefGlobal() GlobalCfg {
	return GlobalCfg{
		Name:              DefaultName,
		Prefix:            DefaultPrefix,
		Footer:            DefaultFooter,
		EmbedColor:        ColorDefault,
		MatrixColor:       "rgb",
		Spotify:           "no",
		AlwaysOnTop:       false,
		FooterIcon:        "https://files.catbox.moe/xxv6qt.webp",
		ShowLogo:          true,
		AutoStartLavalink: true,
		LavalinkHost:      "localhost:2333",
		LavalinkPass:      "youshallnotpass",
		EmojiServerID:     "1411452931915645032",
	}
}

var (
	mu sync.RWMutex
	g  GlobalCfg
)

func SetGlobal(cfg GlobalCfg) {
	mu.Lock()
	g = cfg
	mu.Unlock()
}

func GetGlobal() GlobalCfg {
	mu.RLock()
	defer mu.RUnlock()
	return g
}

type StorageLoc string

const (
	LocLocal    StorageLoc = "local"
	LocPortable StorageLoc = "portable"
	LocAppData  StorageLoc = "appdata"
)

type TuiCfg struct {
	Loc StorageLoc `json:"storage_location"`
}

func GetTuiCfg() TuiCfg {
	b, err := os.ReadFile("tui_config.json")
	if err != nil {
		if exe, err := os.Executable(); err == nil {
			b, err = os.ReadFile(filepath.Join(filepath.Dir(exe), "tui_config.json"))
		}
	}

	var cfg TuiCfg
	if err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if cfg.Loc == "" {
		cfg.Loc = LocLocal
	}
	return cfg
}

func SaveTuiCfg(cfg TuiCfg) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	_ = os.WriteFile("tui_config.json", b, 0600)
	if exe, err := os.Executable(); err == nil {
		_ = os.WriteFile(filepath.Join(filepath.Dir(exe), "tui_config.json"), b, 0600)
	}
	return nil
}

func ResolvePath(name string) string {
	cfg := GetTuiCfg()
	switch cfg.Loc {
	case LocPortable:
		if exe, err := os.Executable(); err == nil {
			return filepath.Join(filepath.Dir(exe), name)
		}
	case LocAppData:
		dir := os.Getenv("APPDATA")
		if dir == "" {
			dir = os.Getenv("HOME")
		}
		if dir != "" {
			p := filepath.Join(dir, "skyvern")
			_ = os.MkdirAll(p, 0755)
			return filepath.Join(p, name)
		}
	}
	return name
}
