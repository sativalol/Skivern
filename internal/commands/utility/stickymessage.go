package utility

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxStickyChan = regexp.MustCompile(`^<#(\d+)>$`)

func init() {
	manager.RegisterHelp("stickymessage", []manager.HelpPage{
		{
			Command:     "StickyMessage Add",
			Syntax:      ".stickymessage add <channel> <message>",
			Description: "Set a sticky message for a channel. Supports JSON embeddings.",
		},
		{
			Command:     "StickyMessage Remove",
			Syntax:      ".stickymessage remove <channel>",
			Description: "Remove the sticky message from a channel.",
		},
		{
			Command:     "StickyMessage View",
			Syntax:      ".stickymessage view <channel>",
			Description: "View the configured sticky message for a channel.",
		},
		{
			Command:     "StickyMessage List",
			Syntax:      ".stickymessage list",
			Description: "List all channels with configured sticky messages.",
		},
	})
}

var StickyMessage = &manager.Command{
	Trigger:     "stickymessage",
	Aliases:     []string{"sticky"},
	Name:        "stickymessage",
	Description: "Manage sticky messages in text channels",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionManageGuild) == 0 {
			return ctx.Reply("[!] You need Manage Guild permission.")
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("stickymessage")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		switch sub {
		case "add":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.stickymessage add <channel> <message>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxStickyChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}
			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Invalid text channel.")
			}

			msg := strings.Join(ctx.Args[2:], " ")
			sm := storage.StickyMessage{
				ChannelID: cid,
				Message:   msg,
			}
			_ = ctx.DB.SaveStickyMessage(gid, cid, sm)
			return ctx.Reply(fmt.Sprintf("[+] Sticky message successfully configured for <#%s>.", cid))

		case "remove", "delete":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.stickymessage remove <channel>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxStickyChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}

			// Clean up previous message if we have it
			if old, err := ctx.DB.GetStickyMessage(gid, cid); err == nil && old.LastMsgID != "" {
				_ = ctx.Session.ChannelMessageDelete(cid, old.LastMsgID)
			}

			_ = ctx.DB.DeleteStickyMessage(gid, cid)
			return ctx.Reply(fmt.Sprintf("[+] Sticky message disabled for <#%s>.", cid))

		case "view":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.stickymessage view <channel>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxStickyChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}

			sm, err := ctx.DB.GetStickyMessage(gid, cid)
			if err != nil || sm.Message == "" {
				return ctx.Reply(fmt.Sprintf("[*] No sticky message configured for <#%s>.", cid))
			}
			return ctx.Reply(fmt.Sprintf("Sticky message for <#%s>:\n```\n%s\n```", cid, sm.Message))

		case "list":
			list, err := ctx.DB.ListStickyMessages(gid)
			if err != nil || len(list) == 0 {
				return ctx.Reply("[*] No sticky messages configured for this server.")
			}
			var sb strings.Builder
			sb.WriteString("Sticky Messages:\n\n")
			for _, sm := range list {
				preview := sm.Message
				if len(preview) > 60 {
					preview = preview[:57] + "..."
				}
				sb.WriteString(fmt.Sprintf("- <#%s>: `%s`\n", sm.ChannelID, preview))
			}
			return ctx.Reply(sb.String())

		default:
			return ctx.SendHelp("stickymessage")
		}
	},
}
