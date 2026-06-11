package utility

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxChan = regexp.MustCompile(`^<#(\d+)>$`)

var Hide = &manager.Command{
	Trigger:     "hide",
	Name:        "hide",
	Description: "Hide channel(s) from members by denying ViewChannel permission",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if ctx.Message != nil {
			p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
			if err != nil || (p&discordgo.PermissionManageRoles) == 0 {
				return ctx.Reply("[!] You need Manage Roles permission to hide channels.")
			}
		}

		gid := ctx.GuildID()
		target := "current"
		if len(ctx.Args) > 0 {
			target = strings.ToLower(ctx.Args[0])
		}

		if target == "all" {
			chans, err := ctx.Session.GuildChannels(gid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to fetch channels: %v", err))
			}
			cnt := 0
			for _, ch := range chans {
				if ch.Type == discordgo.ChannelTypeGuildText || ch.Type == discordgo.ChannelTypeGuildVoice {
					err := setViewChannel(ctx, gid, ch.ID, false)
					if err == nil {
						cnt++
					}
				}
			}
			return ctx.Reply(fmt.Sprintf("[+] Hidden %d channels.", cnt))
		}

		cid := ctx.ChanID()
		if target != "current" {
			if ch, err := moderation.ResolveChannel(ctx.Session, gid, target); err == nil && ch != nil {
				cid = ch.ID
			} else {
				return ctx.Reply("[!] Could not resolve channel: " + target)
			}
		}

		if err := setViewChannel(ctx, gid, cid, false); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to hide channel: %v", err))
		}
		return ctx.Reply(fmt.Sprintf("[+] Channel <#%s> is now hidden.", cid))
	},
}

var Unhide = &manager.Command{
	Trigger:     "unhide",
	Name:        "unhide",
	Description: "Unhide channel(s) for members by allowing ViewChannel permission",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if ctx.Message != nil {
			p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
			if err != nil || (p&discordgo.PermissionManageRoles) == 0 {
				return ctx.Reply("[!] You need Manage Roles permission to unhide channels.")
			}
		}

		gid := ctx.GuildID()
		target := "current"
		if len(ctx.Args) > 0 {
			target = strings.ToLower(ctx.Args[0])
		}

		if target == "all" {
			chans, err := ctx.Session.GuildChannels(gid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to fetch channels: %v", err))
			}
			cnt := 0
			for _, ch := range chans {
				if ch.Type == discordgo.ChannelTypeGuildText || ch.Type == discordgo.ChannelTypeGuildVoice {
					err := setViewChannel(ctx, gid, ch.ID, true)
					if err == nil {
						cnt++
					}
				}
			}
			return ctx.Reply(fmt.Sprintf("[+] Unhidden %d channels.", cnt))
		}

		cid := ctx.ChanID()
		if target != "current" {
			if ch, err := moderation.ResolveChannel(ctx.Session, gid, target); err == nil && ch != nil {
				cid = ch.ID
			} else {
				return ctx.Reply("[!] Could not resolve channel: " + target)
			}
		}

		if err := setViewChannel(ctx, gid, cid, true); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to unhide channel: %v", err))
		}
		return ctx.Reply(fmt.Sprintf("[+] Channel <#%s> is now visible.", cid))
	},
}

func setViewChannel(ctx *manager.CommandContext, gid, cid string, allow bool) error {
	ch, err := ctx.Session.Channel(cid)
	if err != nil {
		return err
	}

	targetID := gid
	var targetType discordgo.PermissionOverwriteType = discordgo.PermissionOverwriteTypeRole

	var denyVal int64
	var allowVal int64

	for _, o := range ch.PermissionOverwrites {
		if o.ID == targetID {
			denyVal = o.Deny
			allowVal = o.Allow
			break
		}
	}

	if allow {
		denyVal &= ^discordgo.PermissionViewChannel
		allowVal |= discordgo.PermissionViewChannel
	} else {
		allowVal &= ^discordgo.PermissionViewChannel
		denyVal |= discordgo.PermissionViewChannel
	}

	action := "hide"
	if allow {
		action = "unhide"
	}

	return ctx.ChannelPermissionSet(cid, targetID, targetType, allowVal, denyVal, fmt.Sprintf("Channel %s", action))
}

var ChannelCmd = &manager.Command{
	Trigger:     "channel",
	Aliases:     []string{"chan", "c"},
	Name:        "channel",
	Description: "Full-featured channel management tool.",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if ctx.Message != nil {
			p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
			if err != nil || (p&discordgo.PermissionManageChannels) == 0 {
				return ctx.Reply("[!] You need Manage Channels permission to manage channels.")
			}
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("channel")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		switch sub {
		case "create":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.channel create <text|voice|category> <name>`")
			}
			tStr := strings.ToLower(ctx.Args[1])
			name := strings.Join(ctx.Args[2:], " ")
			var chType discordgo.ChannelType
			switch tStr {
			case "text":
				chType = discordgo.ChannelTypeGuildText
			case "voice":
				chType = discordgo.ChannelTypeGuildVoice
			case "category", "cat":
				chType = discordgo.ChannelTypeGuildCategory
			default:
				return ctx.Reply("[!] Invalid channel type. Choose from: text, voice, category.")
			}

			ch, err := ctx.Session.GuildChannelCreate(gid, name, chType)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to create channel: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Created %s channel <#%s>.", tStr, ch.ID))

		case "delete":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.channel delete <channel>`")
			}
			ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1])
			if err != nil || ch == nil {
				return ctx.Reply("[!] Could not resolve channel: " + ctx.Args[1])
			}
			_, err = ctx.Session.ChannelDelete(ch.ID)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to delete channel: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Deleted channel **#%s**.", ch.Name))

		case "rename":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.channel rename <channel> <new_name>`")
			}
			ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1])
			if err != nil || ch == nil {
				return ctx.Reply("[!] Could not resolve channel: " + ctx.Args[1])
			}
			newName := strings.Join(ctx.Args[2:], " ")
			_, err = ctx.Session.ChannelEdit(ch.ID, &discordgo.ChannelEdit{
				Name: newName,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to rename channel: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Renamed <#%s> to **%s**.", ch.ID, newName))

		case "topic":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.channel topic <channel> <topic...>`")
			}
			ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1])
			if err != nil || ch == nil {
				return ctx.Reply("[!] Could not resolve channel: " + ctx.Args[1])
			}
			if ch.Type != discordgo.ChannelTypeGuildText {
				return ctx.Reply("[!] You can only set the topic for text channels.")
			}
			topic := strings.Join(ctx.Args[2:], " ")
			_, err = ctx.Session.ChannelEdit(ch.ID, &discordgo.ChannelEdit{
				Topic: topic,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to set topic: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Updated topic for <#%s>.", ch.ID))

		case "nsfw":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.channel nsfw <channel> <yes|no>`")
			}
			ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1])
			if err != nil || ch == nil {
				return ctx.Reply("[!] Could not resolve channel: " + ctx.Args[1])
			}
			if ch.Type != discordgo.ChannelTypeGuildText {
				return ctx.Reply("[!] NSFW toggle only applies to text channels.")
			}
			val := false
			vStr := strings.ToLower(ctx.Args[2])
			if vStr == "yes" || vStr == "y" || vStr == "true" {
				val = true
			}
			_, err = ctx.Session.ChannelEdit(ch.ID, &discordgo.ChannelEdit{
				NSFW: &val,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to set NSFW: %v", err))
			}
			status := "disabled"
			if val {
				status = "enabled"
			}
			return ctx.Reply(fmt.Sprintf("[+] NSFW status %s for <#%s>.", status, ch.ID))

		case "slowmode":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.channel slowmode <channel> <seconds>`")
			}
			ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1])
			if err != nil || ch == nil {
				return ctx.Reply("[!] Could not resolve channel: " + ctx.Args[1])
			}
			if ch.Type != discordgo.ChannelTypeGuildText {
				return ctx.Reply("[!] Slowmode only applies to text channels.")
			}
			secs, err := strconv.Atoi(ctx.Args[2])
			if err != nil || secs < 0 || secs > 21600 {
				return ctx.Reply("[!] Slowmode seconds must be a number between 0 and 21600 (6 hours).")
			}
			_, err = ctx.Session.ChannelEdit(ch.ID, &discordgo.ChannelEdit{
				RateLimitPerUser: &secs,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to set slowmode: %v", err))
			}
			if secs == 0 {
				return ctx.Reply(fmt.Sprintf("[+] Disabled slowmode for <#%s>.", ch.ID))
			}
			return ctx.Reply(fmt.Sprintf("[+] Set slowmode to **%d seconds** for <#%s>.", secs, ch.ID))

		case "limit":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.channel limit <voice_channel> <limit>`")
			}
			ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1])
			if err != nil || ch == nil {
				return ctx.Reply("[!] Could not resolve channel: " + ctx.Args[1])
			}
			if ch.Type != discordgo.ChannelTypeGuildVoice {
				return ctx.Reply("[!] User limits only apply to voice channels.")
			}
			lim, err := strconv.Atoi(ctx.Args[2])
			if err != nil || lim < 0 || lim > 99 {
				return ctx.Reply("[!] Limit must be a number between 0 and 99.")
			}
			_, err = ctx.Session.ChannelEdit(ch.ID, &discordgo.ChannelEdit{
				UserLimit: lim,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to set user limit: %v", err))
			}
			if lim == 0 {
				return ctx.Reply(fmt.Sprintf("[+] Removed user limit for voice channel <#%s>.", ch.ID))
			}
			return ctx.Reply(fmt.Sprintf("[+] Set user limit to **%d** for voice channel <#%s>.", lim, ch.ID))

		case "bitrate":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.channel bitrate <voice_channel> <kbps>`")
			}
			ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1])
			if err != nil || ch == nil {
				return ctx.Reply("[!] Could not resolve channel: " + ctx.Args[1])
			}
			if ch.Type != discordgo.ChannelTypeGuildVoice {
				return ctx.Reply("[!] Bitrate only applies to voice channels.")
			}
			kbps, err := strconv.Atoi(ctx.Args[2])
			if err != nil || kbps < 8 || kbps > 384 {
				return ctx.Reply("[!] Bitrate must be between 8 and 384 kbps.")
			}
			bps := kbps * 1000
			_, err = ctx.Session.ChannelEdit(ch.ID, &discordgo.ChannelEdit{
				Bitrate: bps,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to set bitrate: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Set bitrate to **%d kbps** for voice channel <#%s>.", kbps, ch.ID))

		case "lock":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.channel lock <channel>`")
			}
			ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1])
			if err != nil || ch == nil {
				return ctx.Reply("[!] Could not resolve channel: " + ctx.Args[1])
			}
			var errLock error
			if ch.Type == discordgo.ChannelTypeGuildText {
				errLock = ctx.ChannelPermissionSet(ch.ID, gid, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionSendMessages, "Channel lock")
			} else if ch.Type == discordgo.ChannelTypeGuildVoice {
				errLock = ctx.ChannelPermissionSet(ch.ID, gid, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionVoiceSpeak, "Channel lock")
			} else {
				return ctx.Reply("[!] Lock only applies to text or voice channels.")
			}
			if errLock != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to lock channel: %v", errLock))
			}
			return ctx.Reply(fmt.Sprintf("[+] Locked channel <#%s>.", ch.ID))

		case "unlock":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.channel unlock <channel>`")
			}
			ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[1])
			if err != nil || ch == nil {
				return ctx.Reply("[!] Could not resolve channel: " + ctx.Args[1])
			}
			if errUnlock := ctx.ChannelPermissionDelete(ch.ID, gid, "Channel unlock"); errUnlock != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to unlock channel: %v", errUnlock))
			}
			return ctx.Reply(fmt.Sprintf("[+] Unlocked channel <#%s>.", ch.ID))

		default:
			return ctx.Reply("[!] Unknown subcommand. Options: create, delete, rename, topic, nsfw, slowmode, limit, bitrate, lock, unlock")
		}
	},
}

func init() {
	manager.RegisterHelp("channel", []manager.HelpPage{
		{
			Command:     "Channel Create",
			Syntax:      ".channel create <text|voice|category> <name>",
			Description: "Creates a new guild channel or category.",
		},
		{
			Command:     "Channel Delete",
			Syntax:      ".channel delete <channel>",
			Description: "Deletes a guild channel (resolves name/ID/mention).",
		},
		{
			Command:     "Channel Rename",
			Syntax:      ".channel rename <channel> <new_name>",
			Description: "Renames a channel.",
		},
		{
			Command:     "Channel Topic",
			Syntax:      ".channel topic <channel> <topic...>",
			Description: "Sets the topic of a text channel.",
		},
		{
			Command:     "Channel NSFW",
			Syntax:      ".channel nsfw <channel> <yes|no>",
			Description: "Toggles NSFW status on a text channel.",
		},
		{
			Command:     "Channel Slowmode",
			Syntax:      ".channel slowmode <channel> <seconds>",
			Description: "Sets slowmode cooldown on a text channel.",
		},
		{
			Command:     "Channel Limit",
			Syntax:      ".channel limit <voice_channel> <limit>",
			Description: "Sets voice user limit (0 to 99).",
		},
		{
			Command:     "Channel Bitrate",
			Syntax:      ".channel bitrate <voice_channel> <kbps>",
			Description: "Sets voice bitrate in kbps (8 to 384).",
		},
		{
			Command:     "Channel Lock",
			Syntax:      ".channel lock <channel>",
			Description: "Prevents everyone from speaking/typing in a channel.",
		},
		{
			Command:     "Channel Unlock",
			Syntax:      ".channel unlock <channel>",
			Description: "Restores speaking/typing permission in a channel.",
		},
	})
}
