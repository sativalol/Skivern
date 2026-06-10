package utility

import (
	"fmt"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var Autoresponder = &manager.Command{
	Trigger:     "autoresponder",
	Aliases:     []string{"ar"},
	Name:        "autoresponder",
	Description: "Manage auto responders",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionAdministrator) == 0 {
			return ctx.Reply("[!] You need Administrator permission to use this command.")
		}

		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage:\n" +
				"`.autoresponder add <trigger> <response>`\n" +
				"`.autoresponder remove <trigger>`\n" +
				"`.autoresponder list`")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		switch sub {
		case "add":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.autoresponder add <trigger> <response>`")
			}
			trigger := strings.ToLower(ctx.Args[1])
			response := strings.Join(ctx.Args[2:], " ")

			err = ctx.DB.SaveAutoresponder(gid, trigger, response)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save autoresponder: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Configured autoresponder trigger `%s`.", trigger))

		case "remove", "del":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.autoresponder remove <trigger>`")
			}
			trigger := strings.ToLower(ctx.Args[1])

			err = ctx.DB.DeleteAutoresponder(gid, trigger)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to delete autoresponder: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Removed autoresponder trigger `%s`.", trigger))

		case "list":
			m, err := ctx.DB.ListAutoresponder(gid)
			if err != nil || len(m) == 0 {
				return ctx.Reply("[*] No autoresponders configured.")
			}
			var sb strings.Builder
			sb.WriteString("Configured Auto Responders:\n\n")
			for trigger, resp := range m {
				sb.WriteString(fmt.Sprintf("- `%s` -> %s\n", trigger, resp))
			}
			return ctx.Reply(sb.String())

		default:
			return ctx.Reply("Unknown subcommand. Use add, remove, or list.")
		}
	},
}
