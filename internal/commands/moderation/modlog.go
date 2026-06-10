package moderation

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("modlog", []manager.HelpPage{
		{
			Command:     "Modlog Channel",
			Syntax:      ".modlog channel <#channel/ID>",
			Description: "Set the destination channel for moderation action logs.",
		},
		{
			Command:     "Modlog Toggle",
			Syntax:      ".modlog toggle",
			Description: "Toggle logging of manual Discord server actions (e.g. via UI/right-click).",
		},
		{
			Command:     "Modlog Status",
			Syntax:      ".modlog status",
			Description: "View the current moderation logging channel and configuration status.",
		},
	})
}

var rx = regexp.MustCompile(`^<#(\d+)>$`)

var Modlog = &manager.Command{
	Trigger:     "modlog",
	Aliases:     []string{"ml"},
	Name:        "modlog",
	Description: "Configure moderation logging channel and toggle discord events",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageGuild) {
			return ctx.Reply("[!] You need Manage Guild permission to configure logs.")
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("modlog")
		}

		gid := ctx.GuildID()
		cmd := ctx.Args[0]
		cfg, _ := ctx.DB.GetModlog(gid)

		switch cmd {
		case "channel":
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("modlog")
			}
			tgt := ctx.Args[1]
			if m := rx.FindStringSubmatch(tgt); len(m) > 1 {
				tgt = m[1]
			}
			c, err := ctx.Session.Channel(tgt)
			if err != nil || c.GuildID != gid {
				return ctx.Reply("[!] Invalid channel or not in this guild.")
			}
			cfg.ChannelID = tgt
			_ = ctx.DB.SaveModlog(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Moderation logs will now go to <#%s>.", tgt))

		case "toggle":
			if cfg.ChannelID == "" {
				return ctx.Reply("[!] Configure a log channel first using `modlog channel`.")
			}
			cfg.LogDiscord = !cfg.LogDiscord
			_ = ctx.DB.SaveModlog(gid, cfg)
			status := "disabled"
			if cfg.LogDiscord {
				status = "enabled"
			}
			return ctx.Reply(fmt.Sprintf("[+] Logging of manual Discord actions is now %s.", status))

		case "status":
			chStr := "None"
			if cfg.ChannelID != "" {
				chStr = fmt.Sprintf("<#%s>", cfg.ChannelID)
			}
			ev := "Disabled (bot actions only)"
			if cfg.LogDiscord {
				ev = "Enabled (logs UI bans/kicks/timeouts)"
			}
			return ctx.Reply(fmt.Sprintf("Modlog Configuration:\n  Channel: %s\n  Discord Events: %s", chStr, ev))

		default:
			return ctx.Reply("[!] Unknown subcommand. Use: channel, toggle, status.")
		}
	},
}
