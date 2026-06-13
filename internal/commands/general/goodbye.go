package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxGoodbyeChan = regexp.MustCompile(`^<#(\d+)>$`)

func init() {
	manager.RegisterHelp("goodbye", []manager.HelpPage{
		{
			Command:     "Goodbye Setup",
			Syntax:      ".goodbye add <channel> <message>",
			Description: "Add a goodbye message to a channel. Supports JSON formatting.",
		},
		{
			Command:     "Goodbye Disable",
			Syntax:      ".goodbye remove <channel>",
			Description: "Remove goodbye message configuration from a channel.",
		},
		{
			Command:     "Goodbye List",
			Syntax:      ".goodbye list",
			Description: "View all goodbye messages configured in this server.",
		},
		{
			Command:     "Goodbye View",
			Syntax:      ".goodbye view <channel>",
			Description: "View the goodbye message configured for a specific channel.",
		},
		{
			Command:     "Goodbye Variables",
			Syntax:      ".goodbye variables",
			Description: "View all placeholders and variables for goodbye messages.",
		},
	})
}

var Goodbye = &manager.Command{
	Trigger:     "goodbye",
	Name:        "goodbye",
	Description: "Manage server goodbye messages",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("goodbye")
		}

		sub := strings.ToLower(ctx.Args[0])

		if sub == "variables" || sub == "placeholders" {
			return ctx.Reply("**Goodbye placeholders:**\n" +
				"> `{user}` / `{user.mention}` - Mentions the leaving user\n" +
				"> `{user.name}` - Username of the user\n" +
				"> `{user.id}` - User ID\n" +
				"> `{user.avatar}` - User's avatar image URL\n" +
				"> `{guild.name}` - Server name\n" +
				"> `{guild.count}` - Server member count\n" +
				"> `{guild.boosts}` - Server boost count\n" +
				"> `{guild.icon}` - Server icon image URL\n" +
				"> `{user.created}` - Account age in days\n\n" +
				"**Embed Formatting:** You can pass a raw JSON block starting with `{` and ending with `}` to send formatted rich embeds.")
		}

		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission.")
		}

		gid := ctx.GuildID()

		switch sub {
		case "add":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.goodbye add <channel> <message>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxGoodbyeChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}
			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Invalid text channel.")
			}

			msg := strings.Join(ctx.Args[2:], " ")
			_ = ctx.DB.SaveGoodbyeMsg(gid, cid, msg)
			return ctx.Reply(fmt.Sprintf("[+] Goodbye message configured for <#%s>.", cid))

		case "remove", "delete":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.goodbye remove <channel>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxGoodbyeChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}

			_ = ctx.DB.DeleteGoodbyeMsg(gid, cid)
			return ctx.Reply(fmt.Sprintf("[+] Goodbye message removed from <#%s>.", cid))

		case "view":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.goodbye view <channel>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxGoodbyeChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}

			msg, err := ctx.DB.GetGoodbyeMsg(gid, cid)
			if err != nil || msg == "" {
				return ctx.Reply(fmt.Sprintf("[*] No goodbye message configured for <#%s>.", cid))
			}
			return ctx.Reply(fmt.Sprintf("Goodbye message for <#%s>:\n```\n%s\n```", cid, msg))

		case "list":
			msgs, err := ctx.DB.ListGoodbyeMsgs(gid)
			if err != nil || len(msgs) == 0 {
				return ctx.Reply("[*] No goodbye messages configured for this server.")
			}
			var sb strings.Builder
			sb.WriteString("Configured Goodbye Messages:\n\n")
			for cid, msg := range msgs {
				preview := msg
				if len(preview) > 60 {
					preview = preview[:57] + "..."
				}
				sb.WriteString(fmt.Sprintf("- <#%s>: `%s`\n", cid, preview))
			}
			return ctx.Reply(sb.String())

		default:
			return ctx.SendHelp("goodbye")
		}
	},
}
