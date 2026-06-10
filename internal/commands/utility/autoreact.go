package utility

import (
	"fmt"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var Autoreact = &manager.Command{
	Trigger:     "autoreact",
	Name:        "autoreact",
	Description: "Manage auto reactions triggers and emojis",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionAdministrator) == 0 {
			return ctx.Reply("[!] You need Administrator permission to use this command.")
		}

		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage:\n" +
				"`.autoreact add <trigger> <emoji>`\n" +
				"`.autoreact remove <trigger>`\n" +
				"`.autoreact list`")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		switch sub {
		case "add":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.autoreact add <trigger> <emoji>`")
			}
			trigger := strings.ToLower(ctx.Args[1])
			emoji := ctx.Args[2]

			err = ctx.DB.SaveAutoreact(gid, trigger, emoji)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save autoreact: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Configured autoreact: messages containing `%s` will react with %s.", trigger, emoji))

		case "remove", "del":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.autoreact remove <trigger>`")
			}
			trigger := strings.ToLower(ctx.Args[1])

			err = ctx.DB.DeleteAutoreact(gid, trigger)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to delete autoreact: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Removed autoreact trigger `%s`.", trigger))

		case "list":
			m, err := ctx.DB.ListAutoreact(gid)
			if err != nil || len(m) == 0 {
				return ctx.Reply("[*] No auto reactions configured.")
			}
			var sb strings.Builder
			sb.WriteString("Configured Auto Reactions:\n\n")
			for trigger, emoji := range m {
				sb.WriteString(fmt.Sprintf("- `%s` -> %s\n", trigger, emoji))
			}
			return ctx.Reply(sb.String())

		default:
			return ctx.Reply("Unknown subcommand. Use add, remove, or list.")
		}
	},
}
