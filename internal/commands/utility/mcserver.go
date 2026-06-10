package utility

import (
	"encoding/json"
	"fmt"
	"net/http"
	"skyvern/internal/manager"
	"strings"
)

var MCServer = &manager.Command{
	Trigger:     "mcserver",
	Aliases:     []string{"minecraft", "mc"},
	Name:        "mcserver",
	Description: "Check status of a Minecraft server",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: `.mcserver <ip>` or `.mcserver <ip:port>`")
		}

		serverIP := ctx.Args[0]
		apiURL := fmt.Sprintf("https://api.mcsrvstat.us/3/%s", serverIP)

		resp, err := http.Get(apiURL)
		if err != nil {
			return ctx.Reply("[!] Failed to connect to Minecraft server checker.")
		}
		defer resp.Body.Close()

		var res struct {
			Online bool `json:"online"`
			Ip     string `json:"ip"`
			Port   int    `json:"port"`
			Motd   struct {
				Clean []string `json:"clean"`
			} `json:"motd"`
			Players struct {
				Online int `json:"online"`
				Max    int `json:"max"`
			} `json:"players"`
			Version string `json:"version"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return ctx.Reply("[!] Failed to parse server response.")
		}

		if !res.Online {
			return ctx.Reply(fmt.Sprintf("[*] Server `%s` is offline.", serverIP))
		}

		motd := strings.Join(res.Motd.Clean, "\n")
		if motd == "" {
			motd = "*(No MOTD)*"
		}

		status := fmt.Sprintf("**Minecraft Server Status for %s:%d**\n\n"+
			"**Status:** Online 🟢\n"+
			"**Version:** %s\n"+
			"**Players:** %d / %d\n\n"+
			"**MOTD:**\n```\n%s\n```",
			res.Ip, res.Port, res.Version, res.Players.Online, res.Players.Max, motd)

		return ctx.Reply(status)
	},
}
