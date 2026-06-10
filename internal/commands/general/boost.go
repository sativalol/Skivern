package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxBoostChan = regexp.MustCompile(`^<#(\d+)>$`)

var BoostConfig = &manager.Command{
	Trigger:     "boostconfig",
	Aliases:     []string{"boostmsg", "boostlog"},
	Name:        "boostconfig",
	Description: "Configure the server boost message and logging channel",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission.")
		}

		if len(ctx.Args) < 2 {
			return ctx.Reply("Usage:\n`.boostconfig channel <#channel>`\n`.boostconfig message <customFormat>`")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		cfg, _ := ctx.DB.GetBoostCfg(gid)

		switch sub {
		case "channel", "chan":
			chanArg := ctx.Args[1]
			cid := ""
			if m := rxBoostChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			} else {
				cid = chanArg
			}

			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Could not resolve text channel.")
			}

			cfg.ChannelID = cid
			_ = ctx.DB.SaveBoostCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Boost messages will be sent to <#%s>.", cid))

		case "message", "msg":
			msgFormat := strings.Join(ctx.Args[1:], " ")
			cfg.Message = msgFormat
			_ = ctx.DB.SaveBoostCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Boost message configured:\n%s", msgFormat))

		default:
			return ctx.Reply("Usage:\n`.boostconfig channel <#channel>`\n`.boostconfig message <customFormat>`")
		}
	},
}
