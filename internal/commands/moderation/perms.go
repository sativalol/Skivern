package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var Perms = &manager.Command{
	Trigger:     "perms",
	Aliases:     []string{"permissions", "perms"},
	Name:        "perms",
	Description: "Display a member's permissions and admin status",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageRoles) {
			return ctx.Reply("[!] You need Manage Roles permission.")
		}

		targetQuery := ctx.AuthorID()
		if len(ctx.Args) > 0 {
			targetQuery = ctx.Args[0]
		}

		m, err := moderation.ResolveMember(ctx.Session, ctx.GuildID(), targetQuery)
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}

		p, err := ctx.Session.UserChannelPermissions(m.User.ID, ctx.ChanID())
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to compute permissions: %v", err))
		}

		isAdmin := (p & discordgo.PermissionAdministrator) != 0
		adminText := "False"
		if isAdmin {
			adminText = "True (Administrator)"
		}

		var permsList []string
		allPerms := map[int64]string{
			discordgo.PermissionCreateInstantInvite: "Create Instant Invite",
			discordgo.PermissionKickMembers:         "Kick Members",
			discordgo.PermissionBanMembers:          "Ban Members",
			discordgo.PermissionAdministrator:       "Administrator",
			discordgo.PermissionManageChannels:      "Manage Channels",
			discordgo.PermissionManageGuild:         "Manage Guild",
			discordgo.PermissionAddReactions:        "Add Reactions",
			discordgo.PermissionViewAuditLogs:       "View Audit Logs",
			discordgo.PermissionVoicePrioritySpeaker: "Priority Speaker",
			discordgo.PermissionVoiceStreamVideo:    "Video Stream",
			discordgo.PermissionViewChannel:         "View Channel",
			discordgo.PermissionSendMessages:        "Send Messages",
			discordgo.PermissionSendTTSMessages:     "Send TTS Messages",
			discordgo.PermissionManageMessages:      "Manage Messages",
			discordgo.PermissionEmbedLinks:          "Embed Links",
			discordgo.PermissionAttachFiles:         "Attach Files",
			discordgo.PermissionReadMessageHistory:  "Read Message History",
			discordgo.PermissionMentionEveryone:     "Mention Everyone",
			discordgo.PermissionUseExternalEmojis:   "Use External Emojis",
			discordgo.PermissionViewGuildInsights:   "View Guild Insights",
			discordgo.PermissionVoiceConnect:         "Voice Connect",
			discordgo.PermissionVoiceSpeak:           "Voice Speak",
			discordgo.PermissionVoiceMuteMembers:     "Voice Mute Members",
			discordgo.PermissionVoiceDeafenMembers:   "Voice Deafen Members",
			discordgo.PermissionVoiceMoveMembers:     "Voice Move Members",
			discordgo.PermissionVoiceUseVAD:          "Voice Use VAD",
			discordgo.PermissionChangeNickname:      "Change Nickname",
			discordgo.PermissionManageNicknames:     "Manage Nicknames",
			discordgo.PermissionManageRoles:         "Manage Roles",
			discordgo.PermissionManageWebhooks:      "Manage Webhooks",
			discordgo.PermissionManageGuildExpressions: "Manage Guild Expressions",
			discordgo.PermissionUseApplicationCommands:  "Use Application Commands",
			discordgo.PermissionVoiceRequestToSpeak:     "Request To Speak",
			discordgo.PermissionManageThreads:           "Manage Threads",
			discordgo.PermissionCreatePublicThreads:     "Create Public Threads",
			discordgo.PermissionCreatePrivateThreads:    "Create Private Threads",
			discordgo.PermissionUseExternalStickers:     "Use External Stickers",
			discordgo.PermissionSendMessagesInThreads:   "Send Messages In Threads",
			discordgo.PermissionModerateMembers:         "Moderate Members",
		}

		for permBit, name := range allPerms {
			if (p & permBit) != 0 {
				permsList = append(permsList, name)
			}
		}

		desc := fmt.Sprintf("**User:** <@%s> (`%s`)\n**Administrator status:** %s\n\n**Permissions List:**\n%s",
			m.User.ID, m.User.ID, adminText, "- "+strings.Join(permsList, "\n- "))

		e := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       fmt.Sprintf("Permissions check for %s", m.User.Username),
			Description: desc,
		})
		return ctx.Respond(e)
	},
}
