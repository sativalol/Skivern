package utility

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
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
			if m := rxChan.FindStringSubmatch(target); len(m) > 1 {
				cid = m[1]
			} else {
				cid = target
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
			if m := rxChan.FindStringSubmatch(target); len(m) > 1 {
				cid = m[1]
			} else {
				cid = target
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
