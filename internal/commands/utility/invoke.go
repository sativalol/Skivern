package utility

import (
	"fmt"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("invoke", []manager.HelpPage{
		{
			Command:     "Invoke Save",
			Syntax:      ".invoke <trigger> <template>",
			Description: "Create or update a custom dynamic command template.",
		},
		{
			Command:     "Invoke Remove",
			Syntax:      ".invoke remove <trigger>",
			Description: "Delete a custom dynamic command template.",
		},
		{
			Command:     "Invoke List",
			Syntax:      ".invoke list",
			Description: "List all custom dynamic command templates in the guild.",
		},
	})
}

var Invoke = &manager.Command{
	Trigger:     "invoke",
	Name:        "invoke",
	Description: "Manage custom dynamic commands",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		// Require manage guild permissions to define invokes
		if ctx.Message != nil {
			p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
			if err != nil || (p&discordgo.PermissionManageGuild) == 0 {
				return ctx.Reply("[!] You need Manage Guild permission to manage custom invokes.")
			}
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("invoke")
		}

		gid := ctx.GuildID()
		sub := strings.ToLower(ctx.Args[0])

		switch sub {
		case "list":
			// fetch all custom invokes configured for the guild
			invokes, err := ctx.DB.ListInvokes(gid)
			if err != nil || len(invokes) == 0 {
				return ctx.Reply("[+] No custom invokes configured in this guild.")
			}
			var sb strings.Builder
			sb.WriteString("Custom Invokes:\n\n")
			for k, v := range invokes {
				sb.WriteString(fmt.Sprintf("`%s` -> `%s`\n", k, v))
			}
			return ctx.Reply(sb.String())

		case "remove":
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("invoke")
			}
			trigger := strings.ToLower(ctx.Args[1])
			_ = ctx.DB.DeleteInvoke(gid, trigger)
			return ctx.Reply(fmt.Sprintf("[+] Custom invoke `%s` removed.", trigger))

		default:
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("invoke")
			}
			trigger := strings.ToLower(ctx.Args[0])
			template := strings.Join(ctx.Args[1:], " ")
			_ = ctx.DB.SaveInvoke(gid, trigger, template)
			return ctx.Reply(fmt.Sprintf("[+] Custom invoke `%s` saved! Try running it.", trigger))
		}
	},
}
