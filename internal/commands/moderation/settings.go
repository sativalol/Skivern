package moderation

import (
	"fmt"
	"regexp"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var (
	rxSettingsRole = regexp.MustCompile(`^<@&(\d+)>$`)
	rxSettingsChan = regexp.MustCompile(`^<#(\d+)>$`)
)

func init() {
	manager.RegisterHelp("settings", []manager.HelpPage{
		{
			Command:     "Settings Config",
			Syntax:      ".settings config",
			Description: "View guild settings configuration.",
		},
		{
			Command:     "Settings Baserole",
			Syntax:      ".settings baserole <role>",
			Description: "Set the base role for where custom booster roles will go under.",
		},
		{
			Command:     "Settings Joinlogs",
			Syntax:      ".settings joinlogs <channel>",
			Description: "Set a channel to log member joins/leaves in.",
		},
		{
			Command:     "Settings Jailroles",
			Syntax:      ".settings jailroles <yes|no>",
			Description: "Enable or disable removal of roles for jail.",
		},
		{
			Command:     "Settings Resetcases",
			Syntax:      ".settings resetcases",
			Description: "Reset all guild jail-log cases.",
		},
		{
			Command:     "Settings Jailrole",
			Syntax:      ".settings jailrole <role>",
			Description: "Set default role for the Jail system.",
		},
		{
			Command:     "Settings Autoplay",
			Syntax:      ".settings autoplay <on|off>",
			Description: "Set music autoplay status.",
		},
		{
			Command:     "Settings Jail",
			Syntax:      ".settings jail <channel>",
			Description: "Set jail text channel.",
		},
		{
			Command:     "Settings Jailmsg",
			Syntax:      ".settings jailmsg <message>",
			Description: "Set custom jail message.",
		},
	})
}

var Settings = &manager.Command{
	Trigger:     "settings",
	Aliases:     []string{"setup"},
	Name:        "settings",
	Description: "Guild settings configuration system",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("settings")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		if sub == "resetcases" {
			if !checkPerm(ctx, discordgo.PermissionAdministrator) {
				return ctx.Reply("[!] Administrator permission required to reset cases.")
			}
			if err := ctx.DB.ResetCases(gid); err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to reset cases: %v", err))
			}
			return ctx.Reply("[+] All modlog cases successfully reset.")
		}

		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission.")
		}

		cfg, _ := ctx.DB.GetGuildSettings(gid)

		switch sub {
		case "config", "view":
			emb := config.Build(ctx.Cfg, config.EmbedOpt{
				Title: "Guild Configuration Settings",
				Description: fmt.Sprintf(
					"**Base Booster Role:** <@&%s>\n"+
						"**Join/Leave Logs:** <#%s>\n"+
						"**Jail Role:** <@&%s>\n"+
						"**Jail Channel:** <#%s>\n"+
						"**Strip Roles on Jail:** `%v`\n"+
						"**Autoplay Music:** `%s`\n"+
						"**Custom Jail Msg:** %s",
					nonEmpty(cfg.BaseRoleID, "none"),
					nonEmpty(cfg.JoinLogsChanID, "none"),
					nonEmpty(cfg.JailRoleID, "none"),
					nonEmpty(cfg.JailChanID, "none"),
					cfg.JailRoles,
					nonEmpty(cfg.Autoplay, "off"),
					nonEmpty(cfg.JailMessage, "default"),
				),
			})
			return ctx.Respond(emb)

		case "baserole":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.settings baserole <role>`")
			}
			rid := getRoleID(ctx.Args[1])
			if _, err := ctx.Session.State.Role(gid, rid); err != nil {
				if rlist, err := ctx.Session.GuildRoles(gid); err == nil {
					found := false
					for _, r := range rlist {
						if r.ID == rid {
							found = true
							break
						}
					}
					if !found {
						return ctx.Reply("[!] Invalid role specified.")
					}
				}
			}
			cfg.BaseRoleID = rid
			_ = ctx.DB.SaveGuildSettings(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Booster base role anchored to <@&%s>.", rid))

		case "joinlogs":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.settings joinlogs <channel|off>`")
			}
			val := ctx.Args[1]
			if strings.ToLower(val) == "off" || strings.ToLower(val) == "none" {
				cfg.JoinLogsChanID = ""
				_ = ctx.DB.SaveGuildSettings(gid, cfg)
				return ctx.Reply("[+] Join/Leave logs disabled.")
			}
			cid := getChanID(val)
			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Invalid text channel.")
			}
			cfg.JoinLogsChanID = cid
			_ = ctx.DB.SaveGuildSettings(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Member joins/leaves will be logged in <#%s>.", cid))

		case "jailroles":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.settings jailroles <yes|no>`")
			}
			val := strings.ToLower(ctx.Args[1])
			cfg.JailRoles = (val == "yes" || val == "true" || val == "on")
			_ = ctx.DB.SaveGuildSettings(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Jail roles stripping set to: `%v`.", cfg.JailRoles))

		case "jailrole":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.settings jailrole <role>`")
			}
			rid := getRoleID(ctx.Args[1])
			cfg.JailRoleID = rid
			_ = ctx.DB.SaveGuildSettings(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Default Jail role set to <@&%s>.", rid))

		case "jail":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.settings jail <channel>`")
			}
			cid := getChanID(ctx.Args[1])
			cfg.JailChanID = cid
			_ = ctx.DB.SaveGuildSettings(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Jail actions channel configured: <#%s>.", cid))

		case "jailmsg":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.settings jailmsg <message>`")
			}
			msg := strings.Join(ctx.Args[1:], " ")
			cfg.JailMessage = msg
			_ = ctx.DB.SaveGuildSettings(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Jail announcement message set to:\n%s", msg))

		case "autoplay":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.settings autoplay <on|off>`")
			}
			val := strings.ToLower(ctx.Args[1])
			cfg.Autoplay = val
			_ = ctx.DB.SaveGuildSettings(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Music autoplay set to: `%s`.", val))

		default:
			return ctx.SendHelp("settings")
		}
	},
}

func getRoleID(arg string) string {
	if m := rxSettingsRole.FindStringSubmatch(arg); len(m) > 1 {
		return m[1]
	}
	return arg
}

func getChanID(arg string) string {
	if m := rxSettingsChan.FindStringSubmatch(arg); len(m) > 1 {
		return m[1]
	}
	return arg
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
