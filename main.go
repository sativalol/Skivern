package main

import (
	"fmt"
	"os"
	"skyvern/internal/commands"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/plugins"
	_ "skyvern/internal/plugins/fun"
	"skyvern/internal/storage"
	"skyvern/pkg/tui"
	"strings"
)

func main() {
	if b, err := os.ReadFile(config.ResolvePath("ascii")); err == nil {
		fmt.Print(shrink(string(b), 2))
	} else {
		fmt.Println(tui.Logo)
	}
	fmt.Println("  Skyvern | Version 1.0.0")
	fmt.Println("  Loading configurations...")

	db, err := storage.Open(config.ResolvePath("bots.db"))
	if err != nil {
		fmt.Printf("db init: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if g, err := db.GetGlobal(); err == nil {
		config.SetGlobal(g)
	}

	mgr := manager.New(db, commands.Registry)
	defer mgr.Close()
	commands.Init(mgr)

	for _, p := range plugins.Loaded() {
		if err := p.Init(db, mgr); err != nil {
			fmt.Printf("plugin %s init failed: %v\n", p.Name(), err)
			continue
		}
		mgr.AddCommands(p.Commands())
	}

	if list, err := db.ListBots(); err == nil {
		for _, b := range list {
			if b.IsEnabled {
				_ = mgr.Start(b.ClientID)
			}
		}
	}

	if err := tui.Run(db, mgr); err != nil {
		fmt.Printf("tui error: %v\n", err)
		os.Exit(1)
	}
}

func shrink(art string, factor int) string {
	lns := strings.Split(art, "\n")
	if len(lns) == 0 {
		return ""
	}

	start := 0
	for start < len(lns) && strings.TrimSpace(lns[start]) == "" {
		start++
	}
	end := len(lns)
	for end > start && strings.TrimSpace(lns[end-1]) == "" {
		end--
	}
	lns = lns[start:end]

	if len(lns) == 0 {
		return ""
	}

	min := -1
	for _, l := range lns {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := 0
		for _, r := range l {
			if r == ' ' || r == '\t' {
				n++
			} else {
				break
			}
		}
		if min == -1 || n < min {
			min = n
		}
	}

	for i, l := range lns {
		if len(l) > min {
			lns[i] = l[min:]
		} else {
			lns[i] = ""
		}
	}

	var sb strings.Builder
	for i := 0; i < len(lns); i += factor {
		l := lns[i]
		for j := 0; j < len(l); j += factor {
			sb.WriteByte(l[j])
		}
		sb.WriteByte('\n')
	}

	return sb.String()
}
