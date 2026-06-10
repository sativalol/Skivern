package utility

import (
	"fmt"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("tag", []manager.HelpPage{
		{
			Command:     "Tag Recall",
			Syntax:      ".tag <name>",
			Description: "Recall a custom tag snippet.",
		},
		{
			Command:     "Tag Create",
			Syntax:      ".tag create <name> <content>",
			Description: "Create a new custom tag.",
		},
		{
			Command:     "Tag Delete",
			Syntax:      ".tag delete <name>",
			Description: "Delete a custom tag.",
		},
		{
			Command:     "Tag List",
			Syntax:      ".tag list",
			Description: "List all tags configured for the server.",
		},
	})
}

var Tag = &manager.Command{
	Trigger:     "tag",
	Aliases:     []string{"tg"},
	Name:        "tag",
	Description: "Manage custom tags (snippets)",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("tag")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		switch sub {
		case "create", "add":
			p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
			if err != nil || (p&discordgo.PermissionManageMessages) == 0 {
				return ctx.Reply("[!] You need Manage Messages permission to create tags.")
			}
			if len(ctx.Args) < 3 {
				return ctx.SendHelp("tag")
			}
			tagName := ctx.Args[1]
			tagContent := strings.Join(ctx.Args[2:], " ")

			err = ctx.DB.SaveTag(gid, tagName, tagContent)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save tag: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Tag `%s` created.", tagName))

		case "delete", "remove", "del":
			p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
			if err != nil || (p&discordgo.PermissionManageMessages) == 0 {
				return ctx.Reply("[!] You need Manage Messages permission to delete tags.")
			}
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("tag")
			}
			tagName := ctx.Args[1]

			err = ctx.DB.DeleteTag(gid, tagName)
			if err != nil {
				return ctx.Reply("[!] Tag not found or failed to delete.")
			}
			return ctx.Reply(fmt.Sprintf("[+] Tag `%s` deleted.", tagName))

		case "list":
			tags, err := ctx.DB.ListTags(gid)
			if err != nil || len(tags) == 0 {
				return ctx.Reply("[*] No tags configured for this server.")
			}
			var sb strings.Builder
			sb.WriteString("Server Tags:\n\n")
			for name := range tags {
				sb.WriteString(fmt.Sprintf("- `%s`\n", name))
			}
			return ctx.Reply(sb.String())

		default:
			// Recall tag
			content, err := ctx.DB.GetTag(gid, ctx.Args[0])
			if err != nil || content == "" {
				return ctx.Reply(fmt.Sprintf("[!] Tag `%s` does not exist.", ctx.Args[0]))
			}
			return ctx.Reply(content)
		}
	},
}
