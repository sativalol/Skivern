package utility

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxImgOnlyChan = regexp.MustCompile(`^<#(\d+)>$`)

func init() {
	manager.RegisterHelp("imgonly", []manager.HelpPage{
		{
			Command:     "Image Only Add",
			Syntax:      ".imgonly add <channel>",
			Description: "Set a text channel to image and link only mode.",
		},
		{
			Command:     "Image Only Remove",
			Syntax:      ".imgonly remove <channel>",
			Description: "Remove image only mode from a channel.",
		},
		{
			Command:     "Image Only List",
			Syntax:      ".imgonly list",
			Description: "View all image only channels configured in this server.",
		},
	})
}

var ImgOnly = &manager.Command{
	Trigger:     "imgonly",
	Aliases:     []string{"gallery"},
	Name:        "imgonly",
	Description: "Configure image-only channels",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionManageGuild) == 0 {
			return ctx.Reply("[!] You need Manage Guild permission.")
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("imgonly")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		switch sub {
		case "add":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.imgonly add <channel>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxImgOnlyChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}
			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Invalid text channel.")
			}

			_ = ctx.DB.SaveImgOnlyChannel(gid, cid)
			return ctx.Reply(fmt.Sprintf("[+] Channel <#%s> configured for image-only gallery mode.", cid))

		case "remove", "delete":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.imgonly remove <channel>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxImgOnlyChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}

			_ = ctx.DB.DeleteImgOnlyChannel(gid, cid)
			return ctx.Reply(fmt.Sprintf("[+] Removed image-only mode from <#%s>.", cid))

		case "list":
			chans, err := ctx.DB.ListImgOnlyChannels(gid)
			if err != nil || len(chans) == 0 {
				return ctx.Reply("[*] No image-only channels configured for this server.")
			}
			var sb strings.Builder
			sb.WriteString("Image-only Channels:\n\n")
			for _, cid := range chans {
				sb.WriteString(fmt.Sprintf("- <#%s>\n", cid))
			}
			return ctx.Reply(sb.String())

		default:
			return ctx.SendHelp("imgonly")
		}
	},
}
