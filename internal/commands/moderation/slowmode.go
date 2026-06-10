package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"time"

	"github.com/bwmarrin/discordgo"
)

var Slowmode = &manager.Command{
	Trigger:     "slowmode",
	Aliases:     []string{"slow", "sm"},
	Name:        "slowmode",
	Description: "Set slowmode (message cooldown) for a channel",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageChannels) {
			return ctx.Reply("[!] You need Manage Channels permission.")
		}

		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: .slowmode <time> (e.g. 5s, 1m, 0 to disable)")
		}

		timeStr := ctx.Args[0]
		if timeStr == "0" || timeStr == "off" || timeStr == "disable" {
			val := 0
			_, err := ctx.Session.ChannelEdit(ctx.ChanID(), &discordgo.ChannelEdit{
				RateLimitPerUser: &val,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to disable slowmode: %v", err))
			}
			return ctx.Reply("[+] Slowmode has been disabled.")
		}

		lastChar := timeStr[len(timeStr)-1]
		if lastChar >= '0' && lastChar <= '9' {
			timeStr += "s"
		}

		dur, err := time.ParseDuration(timeStr)
		if err != nil {
			return ctx.Reply("[!] Invalid duration format. Examples: 10s, 5m, 1h.")
		}

		secs := int(dur.Seconds())
		if secs < 0 || secs > 21600 {
			return ctx.Reply("[!] Slowmode must be between 0 seconds and 6 hours.")
		}

		_, err = ctx.Session.ChannelEdit(ctx.ChanID(), &discordgo.ChannelEdit{
			RateLimitPerUser: &secs,
		})
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to set slowmode: %v", err))
		}

		return ctx.Reply(fmt.Sprintf("[+] Set channel slowmode to **%s** (%d seconds).", dur.String(), secs))
	},
}
