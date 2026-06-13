package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxWelcomeChan = regexp.MustCompile(`^<#(\d+)>$`)

func init() {
	manager.RegisterHelp("welcome", []manager.HelpPage{
		{
			Command:     "Welcome Setup",
			Syntax:      ".welcome add <channel> <message>",
			Description: "Add a welcome message to a channel. Supports JSON formatting.",
		},
		{
			Command:     "Welcome Disable",
			Syntax:      ".welcome remove <channel>",
			Description: "Remove welcome message configuration from a channel.",
		},
		{
			Command:     "Welcome List",
			Syntax:      ".welcome list",
			Description: "View all welcome messages configured in this server.",
		},
		{
			Command:     "Welcome View",
			Syntax:      ".welcome view <channel>",
			Description: "View the welcome message configured for a specific channel.",
		},
		{
			Command:     "Welcome Variables",
			Syntax:      ".welcome variables",
			Description: "View all placeholders and variables for welcome messages.",
		},
	})
}

var Welcome = &manager.Command{
	Trigger:     "welcome",
	Name:        "welcome",
	Description: "Manage server welcome messages",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("welcome")
		}

		sub := strings.ToLower(ctx.Args[0])

		if sub == "variables" || sub == "placeholders" {
			return ctx.Reply("**Welcome placeholders:**\n" +
				"> `{user}` / `{user.mention}` - Mentions the joining user\n" +
				"> `{user.name}` - Username of the user\n" +
				"> `{user.id}` - User ID\n" +
				"> `{user.avatar}` - User's avatar image URL\n" +
				"> `{guild.name}` - Server name\n" +
				"> `{guild.count}` - Server member count\n" +
				"> `{guild.boosts}` - Server boost count\n" +
				"> `{guild.icon}` - Server icon image URL\n" +
				"> `{user.created}` - Account age in days\n\n" +
				"**Embed Formatting:** You can pass a raw JSON block starting with `{` and ending with `}` to send formatted rich embeds (e.g. `{\"title\": \"Welcome!\", \"description\": \"Hello {user.mention}!\"}`).")
		}

		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission.")
		}

		gid := ctx.GuildID()

		switch sub {
		case "add":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.welcome add <channel> <message>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxWelcomeChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}
			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Invalid text channel.")
			}

			msg := strings.Join(ctx.Args[2:], " ")
			_ = ctx.DB.SaveWelcomeMsg(gid, cid, msg)
			return ctx.Reply(fmt.Sprintf("[+] Welcome message configured for <#%s>.", cid))

		case "remove", "delete":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.welcome remove <channel>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxWelcomeChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}

			_ = ctx.DB.DeleteWelcomeMsg(gid, cid)
			return ctx.Reply(fmt.Sprintf("[+] Welcome message removed from <#%s>.", cid))

		case "view":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.welcome view <channel>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxWelcomeChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}

			msg, err := ctx.DB.GetWelcomeMsg(gid, cid)
			if err != nil || msg == "" {
				return ctx.Reply(fmt.Sprintf("[*] No welcome message configured for <#%s>.", cid))
			}
			return ctx.Reply(fmt.Sprintf("Welcome message for <#%s>:\n```\n%s\n```", cid, msg))

		case "list":
			msgs, err := ctx.DB.ListWelcomeMsgs(gid)
			if err != nil || len(msgs) == 0 {
				return ctx.Reply("[*] No welcome messages configured for this server.")
			}
			var sb strings.Builder
			sb.WriteString("Configured Welcome Messages:\n\n")
			for cid, msg := range msgs {
				preview := msg
				if len(preview) > 60 {
					preview = preview[:57] + "..."
				}
				sb.WriteString(fmt.Sprintf("- <#%s>: `%s`\n", cid, preview))
			}
			return ctx.Reply(sb.String())

		default:
			return ctx.SendHelp("welcome")
		}
	},
}
