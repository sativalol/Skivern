package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxHallChan = regexp.MustCompile(`^<#(\d+)>$`)

var Hall = &manager.Command{
	Trigger:     "hall",
	Name:        "hall",
	Description: "Configure Hall of Fame and Hall of Shame channels",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission.")
		}

		if len(ctx.Args) < 3 {
			return ctx.Reply("Usage:\n" +
				"`.hall fame channel <#channel> [threshold]`\n" +
				"`.hall shame channel <#channel> [threshold]`")
		}

		hType := strings.ToLower(ctx.Args[0]) // fame or shame
		sub := strings.ToLower(ctx.Args[1])   // channel

		if sub != "channel" && sub != "chan" {
			return ctx.Reply("[!] Invalid syntax. Usage: `.hall <fame/shame> channel <#channel> [threshold]`")
		}

		chanArg := ctx.Args[2]
		cid := ""
		if m := rxHallChan.FindStringSubmatch(chanArg); len(m) > 1 {
			cid = m[1]
		} else {
			cid = chanArg
		}

		gid := ctx.GuildID()
		ch, err := ctx.Session.Channel(cid)
		if err != nil || ch.GuildID != gid {
			return ctx.Reply("[!] Could not resolve text channel.")
		}

		threshold := 3 // default
		if len(ctx.Args) > 3 {
			if num, err := strconv.Atoi(ctx.Args[3]); err == nil && num > 0 {
				threshold = num
			} else {
				return ctx.Reply("[!] Invalid threshold value. Must be a positive number.")
			}
		}

		cfg, _ := ctx.DB.GetHallCfg(gid)

		switch hType {
		case "fame":
			cfg.FameChannelID = cid
			cfg.FameThreshold = threshold
			_ = ctx.DB.SaveHallCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Hall of Fame channel configured to <#%s> with threshold of %d reactions.", cid, threshold))

		case "shame":
			cfg.ShameChannelID = cid
			cfg.ShameThreshold = threshold
			_ = ctx.DB.SaveHallCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Hall of Shame channel configured to <#%s> with threshold of %d reactions.", cid, threshold))

		default:
			return ctx.Reply("[!] Invalid type. Use `fame` or `shame`.")
		}
	},
}
