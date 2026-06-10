package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxReactRole = regexp.MustCompile(`^<@&(\d+)>$`)

var ReactRole = &manager.Command{
	Trigger:     "reactrole",
	Aliases:     []string{"rr"},
	Name:        "reactrole",
	Description: "Configure reaction roles on a message",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionAdministrator) {
			return ctx.Reply("[!] You need Administrator permission to use this command.")
		}

		if len(ctx.Args) < 3 {
			return ctx.Reply("Usage:\n" +
				"`.reactrole add <message_id> <emoji> <@role>`\n" +
				"`.reactrole remove <message_id> <emoji>`")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		switch sub {
		case "add":
			if len(ctx.Args) < 4 {
				return ctx.Reply("Usage: `.reactrole add <message_id> <emoji> <@role>`")
			}
			msgID := ctx.Args[1]
			emoji := ctx.Args[2]
			roleArg := ctx.Args[3]

			rid := ""
			if m := rxReactRole.FindStringSubmatch(roleArg); len(m) > 1 {
				rid = m[1]
			} else {
				rid = roleArg
			}

			roles, err := ctx.Session.GuildRoles(gid)
			if err != nil {
				return ctx.Reply("[!] Failed to fetch guild roles.")
			}

			var targetRole *discordgo.Role
			for _, r := range roles {
				if r.ID == rid {
					targetRole = r
					break
				}
			}
			if targetRole == nil {
				return ctx.Reply("[!] Role not found in server.")
			}

			botMember, err := ctx.Session.GuildMember(gid, ctx.ClientID)
			if err != nil {
				return ctx.Reply("[!] Failed to verify bot hierarchy status.")
			}

			botMaxPos := -1
			for _, r := range roles {
				for _, botRoleID := range botMember.Roles {
					if r.ID == botRoleID && r.Position > botMaxPos {
						botMaxPos = r.Position
					}
				}
			}

			if targetRole.Position >= botMaxPos {
				return ctx.Reply("[!] Security Alert: Target role is higher than or equal to the bot's own role. Action blocked.")
			}

			dangerousPerms := int64(discordgo.PermissionAdministrator |
				discordgo.PermissionManageRoles |
				discordgo.PermissionManageGuild |
				discordgo.PermissionBanMembers |
				discordgo.PermissionKickMembers |
				discordgo.PermissionManageWebhooks |
				discordgo.PermissionManageChannels)
			if (targetRole.Permissions & dangerousPerms) != 0 {
				return ctx.Reply("[!] Security Alert: Target role has administrative/moderation permissions. Action blocked.")
			}

			err = ctx.DB.SaveReactRole(gid, msgID, emoji, rid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save reaction role mapping: %v", err))
			}

			err = ctx.Session.MessageReactionAdd(ctx.ChanID(), msgID, emoji)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[+] Associated reaction role, but could not react to message: %v. Make sure bot has Add Reactions permission and is in the correct channel.", err))
			}

			return ctx.Reply(fmt.Sprintf("[+] Set reaction role on message `%s` with emoji %s assigning <@&%s>.", msgID, emoji, rid))

		case "remove":
			msgID := ctx.Args[1]
			emoji := ctx.Args[2]

			err := ctx.DB.DeleteReactRole(gid, msgID, emoji)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to delete reaction role: %v", err))
			}

			return ctx.Reply(fmt.Sprintf("[+] Removed reaction role on message `%s` with emoji %s.", msgID, emoji))

		default:
			return ctx.Reply("Unknown subcommand. Use add or remove.")
		}
	},
}
