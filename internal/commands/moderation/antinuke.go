package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strconv"
	"strings"
)

func init() {
	manager.RegisterHelp("antinuke", []manager.HelpPage{
		{
			Command:     "Antinuke Toggle",
			Syntax:      ".antinuke <enable|disable>",
			Description: "Turn the antinuke protection engine on or off (owner only).",
		},
		{
			Command:     "Antinuke Settings & Status",
			Syntax:      ".antinuke <settings|status>",
			Description: "View active antinuke thresholds and configurations.",
		},
		{
			Command:     "Antinuke Set Threshold",
			Syntax:      ".antinuke set <limit_name> <value>",
			Description: "Configure threshold values and time windows (e.g. .antinuke set chan_limit 3) (owner only).",
		},
		{
			Command:     "Antinuke Bypass Grant",
			Syntax:      ".antinuke grant <user>",
			Description: "Allow a user to bypass owner-only restrictions (owner only).",
		},
		{
			Command:     "Antinuke Bypass Remove",
			Syntax:      ".antinuke remove <user>",
			Description: "Revoke antinuke bypass permissions from a user (owner only).",
		},
		{
			Command:     "Antinuke Bypass List",
			Syntax:      ".antinuke list",
			Description: "View list of all users currently bypassed.",
		},
	})
}

var Antinuke = &manager.Command{
	Trigger:     "antinuke",
	Aliases:     []string{"an"},
	Name:        "antinuke",
	Description: "Manage antinuke thresholds and bypass lists (owner only)",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("antinuke")
		}

		sub := strings.ToLower(ctx.Args[0])

		switch sub {
		case "enable", "on":
			if !isOwner(ctx) {
				return ctx.Reply("[!] Only the server owner can modify antinuke settings.")
			}
			cfg, _ := ctx.DB.GetAntinukeCfg(gid)
			cfg.Enabled = true
			_ = ctx.DB.SaveAntinukeCfg(gid, cfg)
			return ctx.Reply("[+] Antinuke protection enabled.")

		case "disable", "off":
			if !isOwner(ctx) {
				return ctx.Reply("[!] Only the server owner can modify antinuke settings.")
			}
			cfg, _ := ctx.DB.GetAntinukeCfg(gid)
			cfg.Enabled = false
			_ = ctx.DB.SaveAntinukeCfg(gid, cfg)
			return ctx.Reply("[-] Antinuke protection disabled.")

		case "settings", "status", "config":
			cfg, _ := ctx.DB.GetAntinukeCfg(gid)
			status := "Disabled"
			if cfg.Enabled {
				status = "Enabled"
			}
			desc := fmt.Sprintf(
				"**Status:** `%s`\n"+
					"**Action:** `%s`\n\n"+
					"**Limits (Actions / TimeWindow):**\n"+
					"- Channel Create/Delete: `%d` actions per `%d`s\n"+
					"- Role Create/Delete: `%d` actions per `%d`s\n"+
					"- Member Bans: `%d` actions per `%d`s\n"+
					"- Member Kicks: `%d` actions per `%d`s\n"+
					"- Bot Additions: `%d` actions per `%d`s",
				status, cfg.Action,
				cfg.ChanLimit, cfg.ChanSecs,
				cfg.RoleLimit, cfg.RoleSecs,
				cfg.BanLimit, cfg.BanSecs,
				cfg.KickLimit, cfg.KickSecs,
				cfg.BotLimit, cfg.BotSecs,
			)
			emb := config.Build(ctx.Cfg, config.EmbedOpt{
				Title:       "🛡️ Antinuke Configuration",
				Description: desc,
			})
			return ctx.Respond(emb)

		case "set":
			if !isOwner(ctx) {
				return ctx.Reply("[!] Only the server owner can modify antinuke settings.")
			}
			if len(ctx.Args) < 3 {
				return ctx.SendHelp("antinuke")
			}
			opt := strings.ToLower(ctx.Args[1])
			val := ctx.Args[2]

			cfg, _ := ctx.DB.GetAntinukeCfg(gid)

			if opt == "action" {
				act := strings.ToLower(val)
				if act != "strip" && act != "ban" && act != "kick" {
					return ctx.Reply("[!] Action must be one of: strip, ban, kick.")
				}
				cfg.Action = act
				_ = ctx.DB.SaveAntinukeCfg(gid, cfg)
				return ctx.Reply(fmt.Sprintf("[+] Antinuke action set to `%s`.", act))
			}

			num, err := strconv.Atoi(val)
			if err != nil || num <= 0 {
				return ctx.Reply("[!] Value must be a positive integer.")
			}

			switch opt {
			case "chan_limit", "channel_limit":
				cfg.ChanLimit = num
			case "chan_secs", "chan_time", "channel_time":
				cfg.ChanSecs = num
			case "role_limit":
				cfg.RoleLimit = num
			case "role_secs", "role_time":
				cfg.RoleSecs = num
			case "ban_limit":
				cfg.BanLimit = num
			case "ban_secs", "ban_time":
				cfg.BanSecs = num
			case "kick_limit":
				cfg.KickLimit = num
			case "kick_secs", "kick_time":
				cfg.KickSecs = num
			case "bot_limit":
				cfg.BotLimit = num
			case "bot_secs", "bot_time":
				cfg.BotSecs = num
			default:
				return ctx.Reply("[!] Unknown option. Available options: chan_limit, chan_secs, role_limit, role_secs, ban_limit, ban_secs, kick_limit, kick_secs, bot_limit, bot_secs, action.")
			}

			_ = ctx.DB.SaveAntinukeCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Antinuke option `%s` set to `%d`.", opt, num))

		case "list":
			list, err := ctx.DB.ListBypasses(gid)
			if err != nil || len(list) == 0 {
				return ctx.Reply("[+] No users in antinuke bypass list.")
			}
			var sb strings.Builder
			sb.WriteString("Antinuke Bypassed Users:\n\n")
			for _, uid := range list {
				sb.WriteString(fmt.Sprintf("- <@%s> (`%s`)\n", uid, uid))
			}
			return ctx.Reply(sb.String())

		case "grant":
			if !isOwner(ctx) {
				return ctx.Reply("[!] Only the server owner can grant antinuke bypass.")
			}
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("antinuke")
			}
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			err = ctx.DB.AddBypass(gid, m.User.ID)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save bypass: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Granted antinuke bypass to **%s**.", m.User.Username))

		case "remove":
			if !isOwner(ctx) {
				return ctx.Reply("[!] Only the server owner can revoke antinuke bypass.")
			}
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("antinuke")
			}
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			err = ctx.DB.DeleteBypass(gid, m.User.ID)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to remove bypass: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Revoked antinuke bypass from **%s**.", m.User.Username))

		default:
			return ctx.SendHelp("antinuke")
		}
	},
}
