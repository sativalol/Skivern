package utility

import (
	"fmt"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("alias", []manager.HelpPage{
		{
			Command:     "Alias Add",
			Syntax:      ".alias add <shortcut> <command>",
			Description: "Create a custom shortcut for a command.",
		},
		{
			Command:     "Alias Remove",
			Syntax:      ".alias remove <shortcut>",
			Description: "Delete a custom shortcut.",
		},
		{
			Command:     "Alias Remove All",
			Syntax:      ".alias removeall <command>",
			Description: "Remove all shortcuts associated with a specific command.",
		},
		{
			Command:     "Alias List",
			Syntax:      ".alias list",
			Description: "List all custom command aliases in the server.",
		},
		{
			Command:     "Alias View",
			Syntax:      ".alias view <shortcut>",
			Description: "View the command expansion mapped to a shortcut.",
		},
		{
			Command:     "Alias Reset",
			Syntax:      ".alias reset",
			Description: "Reset and clear all custom aliases.",
		},
	})
}

var Alias = &manager.Command{
	Trigger:     "alias",
	Name:        "alias",
	Description: "Manage custom command shortcuts (aliases)",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionManageGuild) == 0 {
			return ctx.Reply("[!] You need Manage Server permission.")
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("alias")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		switch sub {
		case "add":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.alias add <shortcut> <command>`")
			}
			shortcut := strings.ToLower(ctx.Args[1])
			target := strings.Join(ctx.Args[2:], " ")

			// Prevent mapping shortcut to itself
			if shortcut == target || strings.HasPrefix(target, shortcut+" ") {
				return ctx.Reply("[!] An alias cannot point to its own shortcut.")
			}

			_ = ctx.DB.SaveAlias(gid, shortcut, target)
			return ctx.Reply(fmt.Sprintf("[+] Custom alias created: `%s` -> `%s`.", shortcut, target))

		case "remove":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.alias remove <shortcut>`")
			}
			shortcut := strings.ToLower(ctx.Args[1])
			_ = ctx.DB.DeleteAlias(gid, shortcut)
			return ctx.Reply(fmt.Sprintf("[+] Alias `%s` removed.", shortcut))

		case "removeall":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.alias removeall <command>`")
			}
			targetCmd := strings.Join(ctx.Args[1:], " ")
			aliases, err := ctx.DB.ListAliases(gid)
			if err != nil || len(aliases) == 0 {
				return ctx.Reply("[*] No aliases found.")
			}
			count := 0
			for sc, cmd := range aliases {
				if cmd == targetCmd || strings.HasPrefix(cmd, targetCmd+" ") {
					_ = ctx.DB.DeleteAlias(gid, sc)
					count++
				}
			}
			return ctx.Reply(fmt.Sprintf("[+] Removed %d aliases associated with command `%s`.", count, targetCmd))

		case "list":
			aliases, err := ctx.DB.ListAliases(gid)
			if err != nil || len(aliases) == 0 {
				return ctx.Reply("[*] No custom aliases configured for this server.")
			}
			var sb strings.Builder
			sb.WriteString("Custom Command Aliases:\n\n")
			for sc, cmd := range aliases {
				sb.WriteString(fmt.Sprintf("- `%s` -> `%s`\n", sc, cmd))
			}
			return ctx.Reply(sb.String())

		case "view":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.alias view <shortcut>`")
			}
			shortcut := strings.ToLower(ctx.Args[1])
			target, err := ctx.DB.GetAlias(gid, shortcut)
			if err != nil || target == "" {
				return ctx.Reply(fmt.Sprintf("[!] No alias configured for `%s`.", shortcut))
			}
			return ctx.Reply(fmt.Sprintf("[*] Alias `%s` expands to: `%s`.", shortcut, target))

		case "reset":
			_ = ctx.DB.DeleteAllAliases(gid)
			return ctx.Reply("[+] All custom aliases cleared.")

		default:
			return ctx.SendHelp("alias")
		}
	},
}
