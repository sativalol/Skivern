package moderation

import (
	"fmt"
	"strings"
	"skyvern/internal/manager"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("prefix", []manager.HelpPage{
		{
			Command:     "Prefix View",
			Syntax:      ".prefix",
			Description: "View the current server prefix.",
		},
		{
			Command:     "Prefix Self",
			Syntax:      ".prefix self <prefix>",
			Description: "Set a personal command prefix across all servers.",
		},
		{
			Command:     "Prefix Set",
			Syntax:      ".prefix set <prefix>",
			Description: "Set the custom command prefix for the server.",
		},
		{
			Command:     "Prefix Remove",
			Syntax:      ".prefix remove",
			Description: "Remove the custom server command prefix.",
		},
	})
}

var Prefix = &manager.Command{
	Trigger:     "prefix",
	Name:        "prefix",
	Description: "View or change server and personal prefixes",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		if len(ctx.Args) == 0 {
			prefix := ctx.Cfg.Prefix
			if gp, err := ctx.DB.GetPrefix(gid); err == nil && gp != "" {
				prefix = gp
			}
			return ctx.Reply(fmt.Sprintf("[*] Current prefix for this server is: `%s`", prefix))
		}

		sub := strings.ToLower(ctx.Args[0])

		switch sub {
		case "self":
			uid := ctx.AuthorID()
			if len(ctx.Args) < 2 {
				// Show current self prefix
				if p, err := ctx.DB.GetUserPrefix(uid); err == nil && p != "" {
					return ctx.Reply(fmt.Sprintf("[*] Your personal prefix is: `%s`", p))
				}
				return ctx.Reply("[*] You do not have a personal prefix configured.")
			}
			newPrefix := ctx.Args[1]
			if newPrefix == "default" || newPrefix == "reset" || newPrefix == "none" || newPrefix == "remove" {
				_ = ctx.DB.DeleteUserPrefix(uid)
				return ctx.Reply("[+] Your personal prefix has been reset to default.")
			}
			if len(newPrefix) > 5 {
				return ctx.Reply("[!] Personal prefix cannot be longer than 5 characters.")
			}
			_ = ctx.DB.SaveUserPrefix(uid, newPrefix)
			return ctx.Reply(fmt.Sprintf("[+] Set your personal prefix to `%s` across all servers.", newPrefix))

		case "set":
			if !checkPerm(ctx, discordgo.PermissionManageServer) {
				return ctx.Reply("[!] Manage Server permission required to set guild prefix.")
			}
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.prefix set <prefix>`")
			}
			newPrefix := ctx.Args[1]
			if len(newPrefix) > 5 {
				return ctx.Reply("[!] Prefix cannot be longer than 5 characters.")
			}
			err := ctx.DB.SavePrefix(gid, newPrefix)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save prefix: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Server command prefix successfully set to `%s`.", newPrefix))

		case "remove", "delete":
			if !checkPerm(ctx, discordgo.PermissionManageServer) {
				return ctx.Reply("[!] Manage Server permission required to remove guild prefix.")
			}
			_ = ctx.DB.DeletePrefix(gid)
			return ctx.Reply("[+] Server prefix reset to default.")

		default:
			// Fallback: treat .prefix <val> as setting the guild prefix for compatibility
			if !checkPerm(ctx, discordgo.PermissionManageServer) {
				return ctx.Reply("[!] Manage Server permission required to set guild prefix.")
			}
			newPrefix := ctx.Args[0]
			if len(newPrefix) > 5 {
				return ctx.Reply("[!] Prefix cannot be longer than 5 characters.")
			}
			if newPrefix == "default" || newPrefix == "reset" {
				_ = ctx.DB.DeletePrefix(gid)
				return ctx.Reply("[+] Server prefix reset to default.")
			}
			_ = ctx.DB.SavePrefix(gid, newPrefix)
			return ctx.Reply(fmt.Sprintf("[+] Server prefix successfully set to `%s`.", newPrefix))
		}
	},
}
