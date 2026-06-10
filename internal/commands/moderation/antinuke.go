package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strings"
)

func init() {
	manager.RegisterHelp("antinuke", []manager.HelpPage{
		{
			Command:     "Antinuke Bypass Grant",
			Syntax:      ".antinuke grant <user>",
			Description: "Allow a user to bypass owner-only restrictions (owner only).",
		},
		{
			Command:     "Antinuke Bypass Remove",
			Syntax:      ".antinuke remove <user>",
			Description: "Revoke antinuke bypass permissions from a user (owner only).",
		},
		{
			Command:     "Antinuke Bypass List",
			Syntax:      ".antinuke list",
			Description: "View list of all users currently bypassed.",
		},
	})
}

var Antinuke = &manager.Command{
	Trigger:     "antinuke",
	Aliases:     []string{"an"},
	Name:        "antinuke",
	Description: "Manage antinuke bypass lists (owner only)",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("antinuke")
		}

		sub := strings.ToLower(ctx.Args[0])

		switch sub {
		case "list":
			list, err := ctx.DB.ListBypasses(gid)
			if err != nil || len(list) == 0 {
				return ctx.Reply("[+] No users in antinuke bypass list.")
			}
			var sb strings.Builder
			sb.WriteString("Antinuke Bypassed Users:\n\n")
			for _, uid := range list {
				sb.WriteString(fmt.Sprintf("- <@%s> (`%s`)\n", uid, uid))
			}
			return ctx.Reply(sb.String())

		case "grant":
			if !isOwner(ctx) {
				return ctx.Reply("[!] Only the server owner can grant antinuke bypass.")
			}
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("antinuke")
			}
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			err = ctx.DB.AddBypass(gid, m.User.ID)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save bypass: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Granted antinuke bypass to **%s**.", m.User.Username))

		case "remove":
			if !isOwner(ctx) {
				return ctx.Reply("[!] Only the server owner can revoke antinuke bypass.")
			}
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("antinuke")
			}
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			err = ctx.DB.DeleteBypass(gid, m.User.ID)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to remove bypass: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Revoked antinuke bypass from **%s**.", m.User.Username))

		default:
			return ctx.SendHelp("antinuke")
		}
	},
}
