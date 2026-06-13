package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("antiraid", []manager.HelpPage{
		{
			Command:     "Antiraid Toggle",
			Syntax:      ".antiraid <enable|disable>",
			Description: "Turn the antiraid join flood engine on or off.",
		},
		{
			Command:     "Antiraid Settings & Status",
			Syntax:      ".antiraid <settings|status>",
			Description: "View active antiraid thresholds and configurations.",
		},
		{
			Command:     "Antiraid Set Threshold",
			Syntax:      ".antiraid set <option> <value>",
			Description: "Configure threshold limits (e.g. .antiraid set join_limit 15) (options: join_limit, seconds, action).",
		},
	})
}

var Antiraid = &manager.Command{
	Trigger:     "antiraid",
	Aliases:     []string{"ar"},
	Name:        "antiraid",
	Description: "Manage antiraid thresholds and actions",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageGuild) {
			return ctx.Reply("[!] Manage Guild permission required.")
		}
		gid := ctx.GuildID()

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("antiraid")
		}

		sub := strings.ToLower(ctx.Args[0])

		switch sub {
		case "enable", "on":
			cfg, _ := ctx.Mgr.GetAntiraidCfg(gid)
			cfg.Enabled = true
			_ = ctx.Mgr.SaveAntiraidCfg(gid, cfg)
			return ctx.Reply("[+] Antiraid protection enabled.")

		case "disable", "off":
			cfg, _ := ctx.Mgr.GetAntiraidCfg(gid)
			cfg.Enabled = false
			_ = ctx.Mgr.SaveAntiraidCfg(gid, cfg)
			return ctx.Reply("[-] Antiraid protection disabled.")

		case "settings", "status", "config":
			cfg, _ := ctx.Mgr.GetAntiraidCfg(gid)
			status := "Disabled"
			if cfg.Enabled {
				status = "Enabled"
			}
			desc := fmt.Sprintf(
				"**Status:** `%s`\n"+
					"**Action:** `%s`\n"+
					"**Join Limit:** `%d` joins\n"+
					"**Time Window:** `%d` seconds",
				status, cfg.Action, cfg.JoinLimit, cfg.Seconds,
			)
			emb := config.Build(ctx.Cfg, config.EmbedOpt{
				Title:       "⚠️ Antiraid Configuration",
				Description: desc,
			})
			return ctx.Respond(emb)

		case "set":
			if len(ctx.Args) < 3 {
				return ctx.SendHelp("antiraid")
			}
			opt := strings.ToLower(ctx.Args[1])
			val := ctx.Args[2]

			cfg, _ := ctx.Mgr.GetAntiraidCfg(gid)

			if opt == "action" {
				act := strings.ToLower(val)
				if act != "notify" && act != "lockdown" && act != "kick" && act != "ban" {
					return ctx.Reply("[!] Action must be one of: notify, lockdown, kick, ban.")
				}
				cfg.Action = act
				_ = ctx.Mgr.SaveAntiraidCfg(gid, cfg)
				return ctx.Reply(fmt.Sprintf("[+] Antiraid action set to `%s`.", act))
			}

			num, err := strconv.Atoi(val)
			if err != nil || num <= 0 {
				return ctx.Reply("[!] Value must be a positive integer.")
			}

			switch opt {
			case "join_limit", "limit":
				cfg.JoinLimit = num
			case "seconds", "secs", "time":
				cfg.Seconds = num
			default:
				return ctx.Reply("[!] Unknown option. Available options: join_limit, seconds, action.")
			}

			_ = ctx.Mgr.SaveAntiraidCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Antiraid option `%s` set to `%d`.", opt, num))

		default:
			return ctx.SendHelp("antiraid")
		}
	},
}
