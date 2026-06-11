package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("antilink", []manager.HelpPage{
		{
			Command:     "Anti-Link Configuration",
			Syntax:      ".antilink | .antilink enable | .antilink disable | .antilink action <delete|timeout|kick|ban> | .antilink timeout <duration> | .antilink bypass <yes|no> | .antilink invitesonly <yes|no> | .antilink allowed <add|remove|list> <domain> | .antilink blocked <add|remove|list> <domain> | .antilink whitelist <add|remove|list> <target>",
			Description: "Configure automated link blocking with per-domain and per-invite controls.",
		},
	})
}

var Antilink = &manager.Command{
	Trigger:     "antilink",
	Name:        "antilink",
	Description: "Configure anti-link filter settings",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageGuild) {
			return ctx.Reply("[!] Manage Guild permission required.")
		}

		cfg, _ := ctx.Mgr.GetAntilinkCfg(ctx.GuildID())

		if len(ctx.Args) == 0 {
			status := "Disabled"
			if cfg.Enabled {
				status = "Enabled"
			}
			mode := "Block all links"
			if cfg.BlockInvitesOnly {
				mode = "Invites only"
			} else if len(cfg.AllowedDomains) > 0 {
				mode = fmt.Sprintf("Allowlist (%d domains)", len(cfg.AllowedDomains))
			} else if len(cfg.BlockedDomains) > 0 {
				mode = fmt.Sprintf("Blocklist (%d domains)", len(cfg.BlockedDomains))
			}
			var wList []string
			for _, id := range cfg.Whitelist {
				wList = append(wList, fmt.Sprintf("<@&%s> / <@%s> / <#%s>", id, id, id))
			}
			wlStr := strings.Join(wList, ", ")
			if wlStr == "" {
				wlStr = "*None*"
			}
			dur := time.Duration(cfg.TimeoutSecs) * time.Second
			return ctx.Reply(fmt.Sprintf("[*] **Anti-Link Settings**:\n- Status: `%s`\n- Action: `%s`\n- Timeout Duration: `%s`\n- Mode: `%s`\n- Perm Bypass: `%t`\n- Whitelist: %s", status, cfg.Action, dur.String(), mode, cfg.BypassPerms, wlStr))
		}

		sub := strings.ToLower(ctx.Args[0])
		switch sub {
		case "enable":
			cfg.Enabled = true
			_ = ctx.Mgr.SaveAntilinkCfg(ctx.GuildID(), cfg)
			return ctx.Reply("[+] Anti-Link protection enabled.")
		case "disable":
			cfg.Enabled = false
			_ = ctx.Mgr.SaveAntilinkCfg(ctx.GuildID(), cfg)
			return ctx.Reply("[-] Anti-Link protection disabled.")
		case "action":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .antilink action <delete|timeout|kick|ban>")
			}
			act := strings.ToLower(ctx.Args[1])
			if act != "delete" && act != "timeout" && act != "kick" && act != "ban" {
				return ctx.Reply("[!] Action must be delete, timeout, kick, or ban.")
			}
			cfg.Action = act
			_ = ctx.Mgr.SaveAntilinkCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[+] Violation action updated to `%s`.", act))
		case "timeout":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .antilink timeout <duration>")
			}
			dur, err := time.ParseDuration(ctx.Args[1])
			if err != nil || dur <= 0 {
				return ctx.Reply("[!] Invalid duration. Examples: 10m, 1h, 30s")
			}
			cfg.TimeoutSecs = int(dur.Seconds())
			_ = ctx.Mgr.SaveAntilinkCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[+] Timeout duration updated to `%s`.", dur.String()))
		case "bypass":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .antilink bypass <yes|no>")
			}
			val := strings.ToLower(ctx.Args[1])
			if val != "yes" && val != "no" && val != "true" && val != "false" {
				return ctx.Reply("[!] Bypass must be yes or no.")
			}
			cfg.BypassPerms = (val == "yes" || val == "true")
			_ = ctx.Mgr.SaveAntilinkCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[+] Permission bypass updated to `%t`.", cfg.BypassPerms))
		case "invitesonly":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .antilink invitesonly <yes|no>")
			}
			val := strings.ToLower(ctx.Args[1])
			if val != "yes" && val != "no" && val != "true" && val != "false" {
				return ctx.Reply("[!] Value must be yes or no.")
			}
			cfg.BlockInvitesOnly = (val == "yes" || val == "true")
			_ = ctx.Mgr.SaveAntilinkCfg(ctx.GuildID(), cfg)
			if cfg.BlockInvitesOnly {
				return ctx.Reply("[+] Now blocking Discord invite links only.")
			}
			return ctx.Reply("[-] Now applying full link blocking rules.")
		case "allowed":
			return antilinkDomainList(ctx, &cfg, "allowed")
		case "blocked":
			return antilinkDomainList(ctx, &cfg, "blocked")
		case "whitelist":
			return antilinkWhitelist(ctx, &cfg)
		default:
			return ctx.SendHelp("antilink")
		}
	},
}

func antilinkDomainList(ctx *manager.CommandContext, cfg *storage.AntilinkCfg, listType string) error {
	if len(ctx.Args) < 2 {
		return ctx.Reply(fmt.Sprintf("[!] Usage: .antilink %s <add|remove|list> <domain>", listType))
	}
	op := strings.ToLower(ctx.Args[1])

	var list *[]string
	if listType == "allowed" {
		list = &cfg.AllowedDomains
	} else {
		list = &cfg.BlockedDomains
	}

	if op == "list" {
		if len(*list) == 0 {
			return ctx.Reply(fmt.Sprintf("[*] %s domain list is empty.", strings.ToUpper(listType[:1])+listType[1:]))
		}
		return ctx.Reply(fmt.Sprintf("[*] **%s Domains**:\n`%s`", strings.ToUpper(listType[:1])+listType[1:], strings.Join(*list, "`, `")))
	}
	if len(ctx.Args) < 3 {
		return ctx.Reply("[!] Domain required.")
	}
	domain := strings.ToLower(strings.TrimSpace(ctx.Args[2]))
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.Split(domain, "/")[0]
	if domain == "" {
		return ctx.Reply("[!] Invalid domain.")
	}
	if op == "add" {
		for _, d := range *list {
			if d == domain {
				return ctx.Reply(fmt.Sprintf("[!] Domain `%s` is already in the %s list.", domain, listType))
			}
		}
		*list = append(*list, domain)
		_ = ctx.Mgr.SaveAntilinkCfg(ctx.GuildID(), *cfg)
		return ctx.Reply(fmt.Sprintf("[+] Added `%s` to %s domains.", domain, listType))
	} else if op == "remove" {
		idx := -1
		for i, d := range *list {
			if d == domain {
				idx = i
				break
			}
		}
		if idx == -1 {
			return ctx.Reply(fmt.Sprintf("[!] Domain `%s` not found in %s list.", domain, listType))
		}
		*list = append((*list)[:idx], (*list)[idx+1:]...)
		_ = ctx.Mgr.SaveAntilinkCfg(ctx.GuildID(), *cfg)
		return ctx.Reply(fmt.Sprintf("[-] Removed `%s` from %s domains.", domain, listType))
	}
	return ctx.Reply("[!] Operation must be add, remove, or list.")
}

func antilinkWhitelist(ctx *manager.CommandContext, cfg *storage.AntilinkCfg) error {
	if len(ctx.Args) < 2 {
		return ctx.Reply("[!] Usage: .antilink whitelist <add|remove|list> [target]")
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
		_ = ctx.Mgr.SaveAntilinkCfg(ctx.GuildID(), *cfg)
		return ctx.Reply(fmt.Sprintf("[+] Whitelisted `%s` from anti-link.", targetID))
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
		_ = ctx.Mgr.SaveAntilinkCfg(ctx.GuildID(), *cfg)
		return ctx.Reply(fmt.Sprintf("[-] Removed `%s` from anti-link whitelist.", targetID))
	}
	return ctx.Reply("[!] Operation must be add, remove, or list.")
}
