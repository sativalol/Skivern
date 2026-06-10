package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxBumpChan = regexp.MustCompile(`^<#(\d+)>$`)

var BumpReminder = &manager.Command{
	Trigger:     "bumpreminder",
	Aliases:     []string{"breminder", "bump"},
	Name:        "bumpreminder",
	Description: "Configure bump reminders",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission to configure bump reminders.")
		}

		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage:\n" +
				"`.bumpreminder channel <#channel>`\n" +
				"`.bumpreminder message <text>`\n" +
				"`.bumpreminder enable`\n" +
				"`.bumpreminder disable`")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		cfg, err := ctx.DB.GetBumpCfg(gid)
		if err != nil {
			cfg = storage.BumpCfg{
				Enabled: false,
				Message: "It's time to bump the server! Use `/bump`.",
			}
		}

		switch sub {
		case "channel", "chan":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.bumpreminder channel <#channel>`")
			}
			chanArg := ctx.Args[1]
			cid := ""
			if m := rxBumpChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			} else {
				cid = chanArg
			}

			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Could not resolve text channel.")
			}

			cfg.ChannelID = cid
			_ = ctx.DB.SaveBumpCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Bump reminders will be sent to <#%s>.", cid))

		case "message", "msg":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.bumpreminder message <text>`")
			}
			cfg.Message = strings.Join(ctx.Args[1:], " ")
			_ = ctx.DB.SaveBumpCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Set bump reminder message to:\n%s", cfg.Message))

		case "enable":
			if cfg.ChannelID == "" {
				return ctx.Reply("[!] Please configure a channel first using `.bumpreminder channel <#channel>`.")
			}
			cfg.Enabled = true
			_ = ctx.DB.SaveBumpCfg(gid, cfg)
			return ctx.Reply("[+] Bump reminders enabled.")

		case "disable":
			cfg.Enabled = false
			_ = ctx.DB.SaveBumpCfg(gid, cfg)
			return ctx.Reply("[+] Bump reminders disabled.")

		default:
			return ctx.Reply("Unknown subcommand. Use channel, message, enable, or disable.")
		}
	},
}
