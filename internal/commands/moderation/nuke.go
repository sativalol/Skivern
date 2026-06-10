package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("nuke", []manager.HelpPage{
		{
			Command:     "Nuke Channel",
			Syntax:      ".nuke [#channel]",
			Description: "Clone the current channel or specified channel, delete the old channel, and recreate it clean.",
		},
		{
			Command:     "Nuke Server",
			Syntax:      ".nuke server",
			Description: "Delete all channels and roles, and reset the server (owner or bypassed admins only).",
		},
	})
}

var Nuke = &manager.Command{
	Trigger:     "nuke",
	Name:        "nuke",
	Description: "Nuke a channel or reset the entire server",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()

		// Server nuke requires owner or bypassed antinuke admin
		isServerNuke := len(ctx.Args) > 0 && strings.ToLower(ctx.Args[0]) == "server"

		if isServerNuke {
			if !isOwnerOrBypassed(ctx) {
				return ctx.Reply("[!] Only the server owner or antinuke bypassed administrators can nuke the server.")
			}

			// Nuke roles
			roles, err := ctx.Session.GuildRoles(gid)
			if err == nil {
				for _, r := range roles {
					if r.Managed || r.Name == "@everyone" {
						continue
					}
					_ = ctx.Session.GuildRoleDelete(gid, r.ID)
				}
			}

			// Nuke channels
			chans, err := ctx.Session.GuildChannels(gid)
			if err == nil {
				for _, ch := range chans {
					_, _ = ctx.Session.ChannelDelete(ch.ID)
				}
			}

			// Create a single main category and text channel to reset
			cat, _ := ctx.Session.GuildChannelCreateComplex(gid, discordgo.GuildChannelCreateData{
				Name: "skyvern nuke",
				Type: discordgo.ChannelTypeGuildCategory,
			})
			catID := ""
			if cat != nil {
				catID = cat.ID
			}
			_, _ = ctx.Session.GuildChannelCreateComplex(gid, discordgo.GuildChannelCreateData{
				Name:     "general",
				Type:     discordgo.ChannelTypeGuildText,
				ParentID: catID,
			})
			return nil
		}

		// Channel nuke
		// Check Manage Channels perm
		if !checkPerm(ctx, discordgo.PermissionManageChannels) {
			return ctx.Reply("[!] You need Manage Channels permission.")
		}

		targetChanID := ctx.ChanID()
		if len(ctx.Args) > 0 {
			chArg := ctx.Args[0]
			if strings.HasPrefix(chArg, "<#") && strings.HasSuffix(chArg, ">") {
				targetChanID = chArg[2 : len(chArg)-1]
			} else {
				targetChanID = chArg
			}
		}

		ch, err := ctx.Session.Channel(targetChanID)
		if err != nil || ch.GuildID != gid {
			return ctx.Reply("[!] Could not resolve target channel.")
		}

		if ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildVoice {
			return ctx.Reply("[!] Can only nuke text or voice channels.")
		}

		// Clone the channel
		parentID := ch.ParentID
		pos := ch.Position
		topic := ch.Topic
		nsfw := ch.NSFW
		rateLimit := ch.RateLimitPerUser
		bitrate := ch.Bitrate
		userLimit := ch.UserLimit

		overwrites := make([]*discordgo.PermissionOverwrite, len(ch.PermissionOverwrites))
		for i, ow := range ch.PermissionOverwrites {
			overwrites[i] = &discordgo.PermissionOverwrite{
				ID:    ow.ID,
				Type:  ow.Type,
				Allow: ow.Allow,
				Deny:  ow.Deny,
			}
		}

		newCh, err := ctx.Session.GuildChannelCreateComplex(gid, discordgo.GuildChannelCreateData{
			Name:                 ch.Name,
			Type:                 ch.Type,
			Topic:                topic,
			Bitrate:              bitrate,
			UserLimit:            userLimit,
			RateLimitPerUser:     rateLimit,
			Position:             pos,
			PermissionOverwrites: overwrites,
			ParentID:             parentID,
			NSFW:                 nsfw,
		})

		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to recreate channel: %v", err))
		}

		// Delete old channel
		_, err = ctx.Session.ChannelDelete(targetChanID)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to delete old channel: %v", err))
		}

		// Send success embed to new channel
		_, _ = ctx.Session.ChannelMessageSendEmbed(newCh.ID, config.Wrap(ctx.Cfg, fmt.Sprintf("Channel Recreated Successfully! (Nuked at %s)", time.Now().Format("15:04:05"))))
		return nil
	},
}
