package moderation

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("filter", []manager.HelpPage{
		{
			Command:     "Content Filter",
			Syntax:      ".filter | .filter enable | .filter disable | .filter add <word> | .filter remove <word> | .filter allow <word> | .filter unallow <word> | .filter regex add <pattern> | .filter regex remove <pattern> | .filter bypass <yes|no> | .filter whitelist <add|remove|list> <target> | .filter list",
			Description: "Configure word and regex message content filters.",
		},
	})
}

var Filter = &manager.Command{
	Trigger:     "filter",
	Name:        "filter",
	Description: "Configure message content filters",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageGuild) {
			return ctx.Reply("[!] Manage Guild permission required.")
		}

		cfg, _ := ctx.Mgr.GetFilterCfg(ctx.GuildID())

		if len(ctx.Args) == 0 {
			status := "Disabled"
			if cfg.Enabled {
				status = "Enabled"
			}
			var wList []string
			for _, id := range cfg.Whitelist {
				wList = append(wList, fmt.Sprintf("<@&%s> / <@%s> / <#%s>", id, id, id))
			}
			wlStr := strings.Join(wList, ", ")
			if wlStr == "" {
				wlStr = "*None*"
			}
			return ctx.Reply(fmt.Sprintf("[*] **Filter Settings**:\n- Status: `%s`\n- Blocked Words: `%d` active\n- Allowed Words: `%d` active\n- Custom Regexes: `%d` active\n- Perm Bypass: `%t`\n- Whitelist: %s", status, len(cfg.BlockedWords), len(cfg.AllowedWords), len(cfg.Regexes), cfg.BypassPerms, wlStr))
		}

		sub := strings.ToLower(ctx.Args[0])
		switch sub {
		case "enable":
			cfg.Enabled = true
			_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
			return ctx.Reply("[+] Content filter enabled.")
		case "disable":
			cfg.Enabled = false
			_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
			return ctx.Reply("[-] Content filter disabled.")
		case "bypass":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .filter bypass <yes|no>")
			}
			val := strings.ToLower(ctx.Args[1])
			if val != "yes" && val != "no" && val != "true" && val != "false" {
				return ctx.Reply("[!] Bypass must be yes or no.")
			}
			cfg.BypassPerms = (val == "yes" || val == "true")
			_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[+] Permission bypass updated to `%t`.", cfg.BypassPerms))
		case "add":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .filter add <word>")
			}
			word := strings.Join(ctx.Args[1:], " ")
			for _, w := range cfg.BlockedWords {
				if strings.EqualFold(w, word) {
					return ctx.Reply("[!] Word is already in the blocked list.")
				}
			}
			cfg.BlockedWords = append(cfg.BlockedWords, word)
			_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[+] Added `%s` to blocked words.", word))
		case "remove":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .filter remove <word>")
			}
			word := strings.Join(ctx.Args[1:], " ")
			idx := -1
			for i, w := range cfg.BlockedWords {
				if strings.EqualFold(w, word) {
					idx = i
					break
				}
			}
			if idx == -1 {
				return ctx.Reply("[!] Word not found in blocked list.")
			}
			cfg.BlockedWords = append(cfg.BlockedWords[:idx], cfg.BlockedWords[idx+1:]...)
			_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[-] Removed `%s` from blocked words.", word))
		case "allow":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .filter allow <word>")
			}
			word := strings.Join(ctx.Args[1:], " ")
			for _, w := range cfg.AllowedWords {
				if strings.EqualFold(w, word) {
					return ctx.Reply("[!] Word is already in the allowed list.")
				}
			}
			cfg.AllowedWords = append(cfg.AllowedWords, word)
			_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[+] Added `%s` to allowed exceptions.", word))
		case "unallow":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .filter unallow <word>")
			}
			word := strings.Join(ctx.Args[1:], " ")
			idx := -1
			for i, w := range cfg.AllowedWords {
				if strings.EqualFold(w, word) {
					idx = i
					break
				}
			}
			if idx == -1 {
				return ctx.Reply("[!] Word not found in allowed exceptions.")
			}
			cfg.AllowedWords = append(cfg.AllowedWords[:idx], cfg.AllowedWords[idx+1:]...)
			_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
			return ctx.Reply(fmt.Sprintf("[-] Removed `%s` from allowed exceptions.", word))
		case "regex":
			if len(ctx.Args) < 3 {
				return ctx.Reply("[!] Usage: .filter regex <add|remove> <pattern>")
			}
			op := strings.ToLower(ctx.Args[1])
			pattern := strings.Join(ctx.Args[2:], " ")
			if op == "add" {
				if _, err := regexp.Compile(pattern); err != nil {
					return ctx.Reply(fmt.Sprintf("[!] Invalid regex pattern: `%v`", err))
				}
				for _, p := range cfg.Regexes {
					if p == pattern {
						return ctx.Reply("[!] Regex pattern is already registered.")
					}
				}
				cfg.Regexes = append(cfg.Regexes, pattern)
				_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
				return ctx.Reply(fmt.Sprintf("[+] Registered regex pattern: `%s`", pattern))
			} else if op == "remove" {
				idx := -1
				for i, p := range cfg.Regexes {
					if p == pattern {
						idx = i
						break
					}
				}
				if idx == -1 {
					return ctx.Reply("[!] Regex pattern not found.")
				}
				cfg.Regexes = append(cfg.Regexes[:idx], cfg.Regexes[idx+1:]...)
				_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
				return ctx.Reply(fmt.Sprintf("[-] Removed regex pattern: `%s`", pattern))
			}
			return ctx.Reply("[!] Operation must be add or remove.")
		case "whitelist":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Usage: .filter whitelist <add|remove|list> [target]")
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
				_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
				return ctx.Reply(fmt.Sprintf("[+] Whitelisted `%s` for content filter.", targetID))
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
				_ = ctx.Mgr.SaveFilterCfg(ctx.GuildID(), cfg)
				return ctx.Reply(fmt.Sprintf("[-] Removed `%s` from content filter whitelist.", targetID))
			}
			return ctx.Reply("[!] Operation must be add, remove, or list.")
		case "list":
			bwList := "*None*"
			if len(cfg.BlockedWords) > 0 {
				bwList = fmt.Sprintf("`%s`", strings.Join(cfg.BlockedWords, "`, `"))
			}
			awList := "*None*"
			if len(cfg.AllowedWords) > 0 {
				awList = fmt.Sprintf("`%s`", strings.Join(cfg.AllowedWords, "`, `"))
			}
			rxList := "*None*"
			if len(cfg.Regexes) > 0 {
				rxList = fmt.Sprintf("`%s`", strings.Join(cfg.Regexes, "`, `"))
			}
			return ctx.Reply(fmt.Sprintf("[*] **Content Filter Configuration**:\n\n**Blocked Words:**\n%s\n\n**Allowed Words:**\n%s\n\n**Blocked Regexes:**\n%s", bwList, awList, rxList))
		default:
			return ctx.SendHelp("filter")
		}
	},
}
