package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"skyvern/internal/commands"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/lavalink"
	"skyvern/internal/plugins"
	_ "skyvern/internal/plugins/fun"
	"skyvern/internal/storage"
	"strings"
	"skyvern/pkg/tui"
	"syscall"
	"time"
)

func main() {
	signal.Ignore(syscall.SIGHUP)
	f := setupLogger()
	defer f.Close()
	defer func() {
		if r := recover(); r != nil {
			out := fmt.Sprintf("\n[PANIC] %v\n\n%s\n", r, debug.Stack())
			_, _ = fmt.Fprint(f, out)
			f.Close()
			_ = os.Rename(config.ResolvePath("skyvern.log"), config.ResolvePath(fmt.Sprintf("crash_%s.log", time.Now().Format("2006-01-02_15-04-05"))))
			panic(r)
		}
	}()

	if b, err := os.ReadFile(config.ResolvePath("ascii")); err == nil {
		fmt.Print(tui.Shrink(string(b), 2))
	} else {
		fmt.Println(tui.Logo)
	}
	fmt.Println("  Skyvern | Version 0.1.0-alpha")
	fmt.Println("  Loading cfgs...")

	db, err := storage.Open(config.ResolvePath("bots.db"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "db init: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if g, err := db.GetGlobal(); err == nil {
		config.SetGlobal(g)
	}

	g := config.GetGlobal()
	isLocal := g.LavalinkHost == "" || strings.Contains(g.LavalinkHost, "localhost") || strings.Contains(g.LavalinkHost, "127.0.0.1")
	if g.AutoStartLavalink && isLocal {
		lavalink.StartServer(config.ResolvePath)
		fmt.Print("  Waiting for local Lavalink server to start...")
		start := time.Now()
		client := &http.Client{Timeout: 500 * time.Millisecond}
		for time.Since(start) < 25*time.Second {
			req, err := http.NewRequest("GET", "http://localhost:2333/version", nil)
			if err == nil {
				req.Header.Set("Authorization", g.LavalinkPass)
				resp, err := client.Do(req)
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						break
					}
				}
			}
			fmt.Print(".")
			time.Sleep(1 * time.Second)
		}
		fmt.Println(" Done!")
	}
	defer lavalink.StopServer()

	mgr := manager.New(db, commands.Registry)
	defer mgr.Close()
	commands.Init(mgr)

	for _, p := range plugins.Loaded() {
		if err := p.Init(db, mgr); err != nil {
			fmt.Fprintf(os.Stderr, "plugin %s init failed: %v\n", p.Name(), err)
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
		fmt.Fprintf(os.Stderr, "tui exited: %v\n", err)
	}

	if !mgr.HasRunningBots() {
		return
	}

	fmt.Println("\ntui closed but bots are running in background. ctrl+c to exit")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func setupLogger() *os.File {
	f, err := os.OpenFile(config.ResolvePath("skyvern.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return os.Stderr
	}
	_, _ = fmt.Fprintf(f, "started %s\n\n", time.Now().Format(time.RFC3339))
	os.Stderr = f
	return f
}


