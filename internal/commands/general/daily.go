package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxTime = regexp.MustCompile(`^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$`)
var rxChan = regexp.MustCompile(`^<#(\d+)>$`)

var DailyQuestion = &manager.Command{
	Trigger:     "dailyquestion",
	Aliases:     []string{"dq"},
	Name:        "dailyquestion",
	Description: "Configure daily questions",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission.")
		}

		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: .dailyquestion <#channel> <HH:MM> [disable]")
		}

		gid := ctx.GuildID()

		if strings.ToLower(ctx.Args[0]) == "disable" || (len(ctx.Args) > 2 && strings.ToLower(ctx.Args[2]) == "disable") {
			_ = ctx.DB.SaveDailyQuestion(gid, storage.DailyCfg{Enabled: false})
			return ctx.Reply("[+] Daily questions have been disabled for this server.")
		}

		if len(ctx.Args) < 2 {
			return ctx.Reply("Usage: .dailyquestion <#channel> <HH:MM> [disable]")
		}

		chanArg := ctx.Args[0]
		timeArg := ctx.Args[1]

		cid := ""
		if m := rxChan.FindStringSubmatch(chanArg); len(m) > 1 {
			cid = m[1]
		} else {
			cid = chanArg
		}

		ch, err := ctx.Session.Channel(cid)
		if err != nil || ch.GuildID != gid {
			return ctx.Reply("[!] Could not resolve text channel.")
		}

		if !rxTime.MatchString(timeArg) {
			return ctx.Reply("[!] Invalid time format. Must be HH:MM (24-hour format, e.g. 09:30 or 18:00).")
		}

		if len(timeArg) == 4 {
			timeArg = "0" + timeArg
		}

		err = ctx.DB.SaveDailyQuestion(gid, storage.DailyCfg{
			ChannelID: cid,
			Time:      timeArg,
			Enabled:   true,
		})
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to save configuration: %v", err))
		}

		return ctx.Reply(fmt.Sprintf("[+] Daily questions configured for <#%s> at %s daily.", cid, timeArg))
	},
}

var DailyQuote = &manager.Command{
	Trigger:     "dailyquote",
	Aliases:     []string{"dquote"},
	Name:        "dailyquote",
	Description: "Configure daily quotes",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission.")
		}

		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: .dailyquote <#channel> <HH:MM> [disable]")
		}

		gid := ctx.GuildID()

		if strings.ToLower(ctx.Args[0]) == "disable" || (len(ctx.Args) > 2 && strings.ToLower(ctx.Args[2]) == "disable") {
			_ = ctx.DB.SaveDailyQuote(gid, storage.DailyCfg{Enabled: false})
			return ctx.Reply("[+] Daily quotes have been disabled for this server.")
		}

		if len(ctx.Args) < 2 {
			return ctx.Reply("Usage: .dailyquote <#channel> <HH:MM> [disable]")
		}

		chanArg := ctx.Args[0]
		timeArg := ctx.Args[1]

		cid := ""
		if m := rxChan.FindStringSubmatch(chanArg); len(m) > 1 {
			cid = m[1]
		} else {
			cid = chanArg
		}

		ch, err := ctx.Session.Channel(cid)
		if err != nil || ch.GuildID != gid {
			return ctx.Reply("[!] Could not resolve text channel.")
		}

		if !rxTime.MatchString(timeArg) {
			return ctx.Reply("[!] Invalid time format. Must be HH:MM (24-hour format, e.g. 09:30 or 18:00).")
		}

		if len(timeArg) == 4 {
			timeArg = "0" + timeArg
		}

		err = ctx.DB.SaveDailyQuote(gid, storage.DailyCfg{
			ChannelID: cid,
			Time:      timeArg,
			Enabled:   true,
		})
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to save configuration: %v", err))
		}

		return ctx.Reply(fmt.Sprintf("[+] Daily quotes configured for <#%s> at %s daily.", cid, timeArg))
	},
}
