package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxBoostChan = regexp.MustCompile(`^<#(\d+)>$`)

func init() {
	manager.RegisterHelp("boosts", []manager.HelpPage{
		{
			Command:     "Boost Message Add",
			Syntax:      ".boosts add <channel> <message>",
			Description: "Set a custom boost announcement message for a channel.",
		},
		{
			Command:     "Boost Message Remove",
			Syntax:      ".boosts remove <channel>",
			Description: "Disable boost announcement messages in a channel.",
		},
		{
			Command:     "Boost Message List",
			Syntax:      ".boosts list",
			Description: "List all channels where boost messages are enabled.",
		},
		{
			Command:     "Boost Message View",
			Syntax:      ".boosts view <channel>",
			Description: "View the custom boost announcement message set for a channel.",
		},
		{
			Command:     "Boost Message Variables",
			Syntax:      ".boosts variables",
			Description: "View placeholders and variables for boost messages.",
		},
	})
}

var BoostConfig = &manager.Command{
	Trigger:     "boosts",
	Aliases:     []string{"boostconfig", "boostmsg", "boostlog"},
	Name:        "boosts",
	Description: "Manage server boost messages",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("boosts")
		}

		sub := strings.ToLower(ctx.Args[0])

		if sub == "variables" || sub == "placeholders" {
			return ctx.Reply("**Boost message placeholders:**\n" +
				"> `{user}` / `{user.mention}` - Mentions the boosting user\n" +
				"> `{user.name}` - Username of the user\n" +
				"> `{user.id}` - User ID\n" +
				"> `{guild.name}` - Server name\n" +
				"> `{guild.boosts}` - Current server boost count\n\n" +
				"**Embed Formatting:** You can pass a raw JSON block starting with `{` and ending with `}` to send formatted rich embeds.")
		}

		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission.")
		}

		gid := ctx.GuildID()

		switch sub {
		case "add":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.boosts add <channel> <message>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxBoostChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}
			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Invalid text channel.")
			}

			msg := strings.Join(ctx.Args[2:], " ")
			_ = ctx.DB.SaveBoostMsg(gid, cid, msg)
			return ctx.Reply(fmt.Sprintf("[+] Boost announcement configured for <#%s>.", cid))

		case "remove", "delete":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.boosts remove <channel>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxBoostChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}

			_ = ctx.DB.DeleteBoostMsg(gid, cid)
			return ctx.Reply(fmt.Sprintf("[+] Boost announcement disabled for <#%s>.", cid))

		case "view":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.boosts view <channel>`")
			}
			chanArg := ctx.Args[1]
			cid := chanArg
			if m := rxBoostChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			}

			msg, err := ctx.DB.GetBoostMsg(gid, cid)
			if err != nil || msg == "" {
				return ctx.Reply(fmt.Sprintf("[*] No boost announcement configured for <#%s>.", cid))
			}
			return ctx.Reply(fmt.Sprintf("Boost message for <#%s>:\n```\n%s\n```", cid, msg))

		case "list":
			msgs, err := ctx.DB.ListBoostMsgs(gid)
			if err != nil || len(msgs) == 0 {
				return ctx.Reply("[*] No boost announcements configured for this server.")
			}
			var sb strings.Builder
			sb.WriteString("Configured Boost Messages:\n\n")
			for cid, msg := range msgs {
				preview := msg
				if len(preview) > 60 {
					preview = preview[:57] + "..."
				}
				sb.WriteString(fmt.Sprintf("- <#%s>: `%s`\n", cid, preview))
			}
			return ctx.Reply(sb.String())

		default:
			return ctx.SendHelp("boosts")
		}
	},
}
