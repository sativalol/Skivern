package moderation

import (
	"fmt"
	"skyvern/internal/manager"

	"github.com/bwmarrin/discordgo"
)

var Prefix = &manager.Command{
	Trigger:     "prefix",
	Name:        "prefix",
	Description: "View or change the server bot prefix",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		if len(ctx.Args) == 0 {
			prefix := ctx.Cfg.Prefix
			return ctx.Reply(fmt.Sprintf("[*] Current prefix for this server is: `%s`", prefix))
		}

		if !checkPerm(ctx, discordgo.PermissionAdministrator) {
			return ctx.Reply("[!] Only administrators can change the prefix.")
		}

		newPrefix := ctx.Args[0]
		if len(newPrefix) > 5 {
			return ctx.Reply("[!] Prefix cannot be longer than 5 characters.")
		}

		if newPrefix == "default" || newPrefix == "reset" {
			_ = ctx.Mgr.DeletePrefix(gid)
			return ctx.Reply("[+] Prefix reset to default.")
		}

		err := ctx.Mgr.SavePrefix(gid, newPrefix)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to save prefix: %v", err))
		}
		return ctx.Reply(fmt.Sprintf("[+] Prefix successfully changed to `%s` for this server.", newPrefix))
	},
}
