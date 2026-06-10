package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var RMute = &manager.Command{
	Trigger:     "rmute",
	Name:        "rmute",
	Description: "Restrict a member from using reactions and external emotes",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageRoles) {
			return ctx.Reply("[!] You need Manage Roles permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: rmute <user> [reason]")
		}

		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}

		reason := "No reason provided"
		if len(ctx.Args) > 1 {
			reason = strings.Join(ctx.Args[1:], " ")
		}

		cid := ctx.ChanID()
		err = ctx.Session.ChannelPermissionSet(cid, m.User.ID, discordgo.PermissionOverwriteTypeMember, 0, discordgo.PermissionAddReactions|discordgo.PermissionUseExternalEmojis)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to reaction mute: %v", err))
		}

		return ctx.Reply(fmt.Sprintf("[+] Restrained **%s** from using reactions and external emotes in this channel for: %s", m.User.Username, reason))
	},
}
