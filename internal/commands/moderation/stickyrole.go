package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var Stickyrole = &manager.Command{
	Trigger:     "stickyrole",
	Aliases:     []string{"sticky", "sr"},
	Name:        "stickyrole",
	Description: "Enforce sticky roles on rejoin or assign auto-role on join",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageRoles) {
			return ctx.Reply("[!] You need Manage Roles permission.")
		}

		if len(ctx.Args) < 2 {
			return ctx.Reply("Usage: .stickyrole <user|everyone> <role> [enable/disable]")
		}

		gid := ctx.GuildID()
		targetArg := strings.ToLower(ctx.Args[0])
		roleArg := ctx.Args[1]

		rid := resolveRole(ctx.Session, gid, roleArg)
		if rid == "" {
			return ctx.Reply("[!] Could not resolve role.")
		}

		action := "enable"
		if len(ctx.Args) > 2 {
			action = strings.ToLower(ctx.Args[2])
		}

		isEveryone := targetArg == "all" || targetArg == "everyone"
		targetID := "everyone"

		if !isEveryone {
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			targetID = m.User.ID
		}

		if action == "disable" || action == "remove" || action == "off" {
			_ = ctx.DB.DeleteStickyRole(gid, targetID, rid)
			if isEveryone {
				return ctx.Reply("[+] Disabled sticky auto-role for everyone.")
			}
			return ctx.Reply(fmt.Sprintf("[+] Disabled sticky role for user ID `%s`.", targetID))
		}

		err := ctx.DB.SaveStickyRole(gid, targetID, rid)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to save sticky role: %v", err))
		}

		if isEveryone {
			return ctx.Reply("[+] Enabled sticky auto-role for everyone.")
		}

		_ = ctx.Session.GuildMemberRoleAdd(gid, targetID, rid)

		return ctx.Reply(fmt.Sprintf("[+] Sticky role enabled for ID `%s`.", targetID))
	},
}
