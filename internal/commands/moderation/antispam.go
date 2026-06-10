package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("antispam", []manager.HelpPage{
		{
			Command:     "Anti-Spam Configuration",
			Syntax:      ".antispam | .antispam enable | .antispam disable | .antispam limit <count> <seconds> | .antispam action <timeout|kick|ban> | .antispam timeout <duration> | .antispam bypass <yes|no> | .antispam whitelist <add|remove|list> <@target>",
			Description: "Configure automated anti-spam protection.",
		},
	})
}

var Antispam = &manager.Command{
	Trigger:     "antispam",
	Name:        "antispam",
	Description: "Configure anti-spam filter settings",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageGuild) {
			return ctx.Reply("[!] Manage Guild permission required.")
		}

		cfg, _ := ctx.Mgr.GetAntispamCfg(ctx.GuildID())

		if len(ctx.Args) == 0 {
			var wList []string
			for _, id := range cfg.Whitelist {
				wList = append(wList, fmt.Sprintf("<@&%s> / <@%s> / <#%s>", id, id, id))
			}
			wlStr := strings.Join(wList, ", ")
			if wlStr == "" {
				wlStr = "*None*"
			}
			status := "Disabled"
			if cfg.Enabled {
				status = "Enabled"
			}
			timeoutDur := time.Duration(cfg.TimeoutSecs) * time.Second
			return ctx.Reply(fmt.Sprintf("[*] **Anti-Spam Settings**:\n- Status: `%s`\n- Limit: `%d` messages in `%d` seconds\n- Action: `%s`\n- Timeout Duration: `%s`\n- Perm Bypass: `%t`\n- Whitelist: %s", status, cfg.Limit, cfg.Seconds, cfg.Action, timeoutDur.String(), cfg.BypassPerms, wlStr))
		}

		sub := strings.ToLower(ctx.Args[0])
		switch sub {
		case "enable":
			cfg.Enabled = true
			_ = ctx.Mgr.SaveAntispamCfg(ctx.GuildID(), cfg)
			return ctx.Reply("[+] Anti-Spam protection has been enabled.")
		case "disable":
			cfg.Enabled = false
			_ = ctx.Mgr.SaveAntispamCfg(ctx.GuildID(), cfg)
			return ctx.Reply("[-] Anti-Spam protection has been disabled.")
		case "limit":
			if len(ctx.Args) < 3 {
				return ctx.Reply("[!] Usage: .antispam limit <count> <seconds>")
			}
			count, err1 := strconv.Atoi(ctx.Args[1])
			secs, err2 := strconv.Atoi(ctx.Args[2])
			if err1 != nil || err2 != nil || count <= 0 || secs <= 0 {
				return ctx.Reply("[!] Invalid limit parameters.")
			}
			cfg.Limit = count
			cfg.Seconds = secs
			_ = ctx.Mgr.SaveAntispamCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[+] Limit updated to `%d` messages in `%d` seconds.", count, secs))
		case "action":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .antispam action <timeout|kick|ban>")
			}
			act := strings.ToLower(ctx.Args[1])
			if act != "timeout" && act != "kick" && act != "ban" {
				return ctx.Reply("[!] Action must be timeout, kick, or ban.")
			}
			cfg.Action = act
			_ = ctx.Mgr.SaveAntispamCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[+] Violation action updated to `%s`.", act))
		case "timeout":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .antispam timeout <duration>")
			}
			durStr := ctx.Args[1]
			dur, err := time.ParseDuration(durStr)
			if err != nil || dur <= 0 {
				return ctx.Reply("[!] Invalid duration format. Example: 10m, 5s, 1h")
			}
			cfg.TimeoutSecs = int(dur.Seconds())
			_ = ctx.Mgr.SaveAntispamCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[+] Anti-spam timeout duration updated to `%s`.", durStr))
		case "bypass":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .antispam bypass <yes|no>")
			}
			val := strings.ToLower(ctx.Args[1])
			if val != "yes" && val != "no" && val != "true" && val != "false" {
				return ctx.Reply("[!] Bypass must be yes or no.")
			}
			cfg.BypassPerms = (val == "yes" || val == "true")
			_ = ctx.Mgr.SaveAntispamCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[+] Permission bypass updated to `%t`.", cfg.BypassPerms))
		case "whitelist":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .antispam whitelist <add|remove|list> [target]")
			}
			op := strings.ToLower(ctx.Args[1])
			if op == "list" {
				var wl []string
				for _, id := range cfg.Whitelist {
					wl = append(wl, fmt.Sprintf("<@&%s> / <@%s> / <#%s>", id, id, id))
				}
				if len(wl) == 0 {
					return ctx.Reply("[*] Whitelist is empty.")
				}
				return ctx.Reply(fmt.Sprintf("[*] Whitelisted entities:\n%s", strings.Join(wl, "\n")))
			}
			if len(ctx.Args) < 3 {
				return ctx.Reply("[!] Target required.")
			}
			target := ctx.Args[2]
			targetID := ""
			if m := rxMember.FindStringSubmatch(target); len(m) > 1 {
				targetID = m[1]
			} else if m := rxChannel.FindStringSubmatch(target); len(m) > 1 {
				targetID = m[1]
			} else if strings.HasPrefix(target, "<@&") && strings.HasSuffix(target, ">") {
				targetID = strings.TrimSuffix(strings.TrimPrefix(target, "<@&"), ">")
			} else {
				targetID = target
			}

			if op == "add" {
				for _, id := range cfg.Whitelist {
					if id == targetID {
						return ctx.Reply("[!] Target is already whitelisted.")
					}
				}
				cfg.Whitelist = append(cfg.Whitelist, targetID)
				_ = ctx.Mgr.SaveAntispamCfg(ctx.GuildID(), cfg)
				return ctx.Reply(fmt.Sprintf("[+] Whitelisted `%s`.", targetID))
			} else if op == "remove" {
				idx := -1
				for i, id := range cfg.Whitelist {
					if id == targetID {
						idx = i
						break
					}
				}
				if idx == -1 {
					return ctx.Reply("[!] Target is not whitelisted.")
				}
				cfg.Whitelist = append(cfg.Whitelist[:idx], cfg.Whitelist[idx+1:]...)
				_ = ctx.Mgr.SaveAntispamCfg(ctx.GuildID(), cfg)
				return ctx.Reply(fmt.Sprintf("[-] Removed `%s` from whitelist.", targetID))
			}
			return ctx.Reply("[!] Operation must be add, remove, or list.")
		default:
			return ctx.SendHelp("antispam")
		}
	},
}
