package moderation

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	rxUtilityChan = regexp.MustCompile(`^<#(\d+)>$`)
	rxUtilityRole = regexp.MustCompile(`^<@&(\d+)>$`)
	rxUtilityUser = regexp.MustCompile(`^<@!?(\d+)>$`)
)

func init() {
	manager.RegisterHelp("clearinvites", []manager.HelpPage{
		{
			Command:     "Clear Invites",
			Syntax:      ".clearinvites",
			Description: "Delete all invite codes in the server.",
		},
	})
	manager.RegisterHelp("drag", []manager.HelpPage{
		{
			Command:     "Drag",
			Syntax:      ".drag <@member1> [@member2...] <#channel>",
			Description: "Drag members to a specific voice channel.",
		},
	})
	manager.RegisterHelp("newmembers", []manager.HelpPage{
		{
			Command:     "New Members",
			Syntax:      ".newmembers [count]",
			Description: "View recently joined server members.",
		},
	})
	manager.RegisterHelp("recentban", []manager.HelpPage{
		{
			Command:     "Recent Ban",
			Syntax:      ".recentban <count> [reason]",
			Description: "Ban a chunk of recently joined members.",
		},
	})
	manager.RegisterHelp("talk", []manager.HelpPage{
		{
			Command:     "Talk Toggle",
			Syntax:      ".talk <#channel> <@role>",
			Description: "Toggle send message permissions for a role in a channel.",
		},
	})
	manager.RegisterHelp("revokefiles", []manager.HelpPage{
		{
			Command:     "Revoke Files",
			Syntax:      ".revokefiles <on|off> [channel]",
			Description: "Enable or disable file attachments and embed link permissions for everyone in a channel.",
		},
	})
	manager.RegisterHelp("restrictcommand", []manager.HelpPage{
		{
			Command:     "Restrict Command",
			Syntax:      ".restrictcommand <command> <@role>",
			Description: "Restrict a bot command usage to a specific role.",
		},
		{
			Command:     "Restrict Command Remove",
			Syntax:      ".restrictcommand remove <command> <@role>",
			Description: "Remove role restriction from a command.",
		},
		{
			Command:     "Restrict Command List",
			Syntax:      ".restrictcommand list",
			Description: "List all restricted commands.",
		},
		{
			Command:     "Restrict Command Reset",
			Syntax:      ".restrictcommand reset",
			Description: "Reset all command restrictions.",
		},
	})
	manager.RegisterHelp("topic", []manager.HelpPage{
		{
			Command:     "Topic",
			Syntax:      ".topic <text>",
			Description: "Change the topic of the current text channel.",
		},
	})
	manager.RegisterHelp("naughty", []manager.HelpPage{
		{
			Command:     "Naughty",
			Syntax:      ".naughty [channel]",
			Description: "Temporarily make a text channel NSFW for 30 seconds.",
		},
	})
}

var ClearInvites = &manager.Command{
	Trigger:     "clearinvites",
	Name:        "clearinvites",
	Description: "Delete all invite codes in the server",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission.")
		}
		gid := ctx.GuildID()
		invites, err := ctx.Session.GuildInvites(gid)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to fetch invites: %v", err))
		}
		if len(invites) == 0 {
			return ctx.Reply("[*] No active invites found in the server.")
		}
		deleted := 0
		for _, inv := range invites {
			if _, err := ctx.Session.InviteDelete(inv.Code); err == nil {
				deleted++
			}
		}
		return ctx.Reply(fmt.Sprintf("[+] Successfully deleted %d invite codes.", deleted))
	},
}

var Drag = &manager.Command{
	Trigger:     "drag",
	Name:        "drag",
	Description: "Drag members to a voice channel",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) < 2 {
			return ctx.SendHelp("drag")
		}
		gid := ctx.GuildID()
		// Last argument is voice channel
		chanArg := ctx.Args[len(ctx.Args)-1]
		var channelID string
		if m := rxUtilityChan.FindStringSubmatch(chanArg); len(m) > 1 {
			channelID = m[1]
		} else {
			channelID = chanArg
		}

		ch, err := ctx.Session.Channel(channelID)
		if err != nil || ch.Type != discordgo.ChannelTypeGuildVoice {
			return ctx.Reply("[!] Target channel must be a valid voice channel.")
		}

		moved := 0
		memberArgs := ctx.Args[:len(ctx.Args)-1]
		for _, arg := range memberArgs {
			var uid string
			if m := rxUtilityUser.FindStringSubmatch(arg); len(m) > 1 {
				uid = m[1]
			} else {
				uid = arg
			}
			err := ctx.Session.GuildMemberMove(gid, uid, &channelID)
			if err == nil {
				moved++
			}
		}
		return ctx.Reply(fmt.Sprintf("[+] Moved %d member(s) to voice channel <#%s>.", moved, channelID))
	},
}

var NewMembers = &manager.Command{
	Trigger:     "newmembers",
	Name:        "newmembers",
	Description: "List recently joined members",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		count := 10
		if len(ctx.Args) > 0 {
			if val, err := strconv.Atoi(ctx.Args[0]); err == nil && val > 0 {
				count = val
			}
		}
		if count > 50 {
			count = 50
		}

		mList, err := ctx.Session.GuildMembers(gid, "", 1000)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to fetch members: %v", err))
		}

		sort.Slice(mList, func(i, j int) bool {
			return mList[i].JoinedAt.After(mList[j].JoinedAt)
		})

		if len(mList) < count {
			count = len(mList)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Recently Joined Members (Showing %d):\n\n", count))
		for i := 0; i < count; i++ {
			m := mList[i]
			joinTime := m.JoinedAt.Format("2006-01-02 15:04")
			sb.WriteString(fmt.Sprintf("- <@%s> (`%s`) | Joined: %s\n", m.User.ID, m.User.ID, joinTime))
		}
		return ctx.Reply(sb.String())
	},
}

var RecentBan = &manager.Command{
	Trigger:     "recentban",
	Name:        "recentban",
	Description: "Ban recently joined members",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionBanMembers) {
			return ctx.Reply("[!] You need Ban Members permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("recentban")
		}
		count, err := strconv.Atoi(ctx.Args[0])
		if err != nil || count <= 0 {
			return ctx.Reply("[!] Invalid count specified.")
		}
		if count > 100 {
			return ctx.Reply("[!] Limit chunk ban count to max 100.")
		}
		reason := "Recent ban chunk execution"
		if len(ctx.Args) > 1 {
			reason = strings.Join(ctx.Args[1:], " ")
		}

		gid := ctx.GuildID()
		mList, err := ctx.Session.GuildMembers(gid, "", 1000)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to fetch members: %v", err))
		}

		sort.Slice(mList, func(i, j int) bool {
			return mList[i].JoinedAt.After(mList[j].JoinedAt)
		})

		if len(mList) < count {
			count = len(mList)
		}

		banned := 0
		for i := 0; i < count; i++ {
			m := mList[i]
			// Avoid banning the author or bot
			if m.User.ID == ctx.AuthorID() || m.User.Bot {
				continue
			}
			err := ctx.Session.GuildBanCreateWithReason(gid, m.User.ID, reason, 0)
			if err == nil {
				banned++
			}
		}
		return ctx.Reply(fmt.Sprintf("[+] Successfully banned %d recently joined member(s).", banned))
	},
}

var Talk = &manager.Command{
	Trigger:     "talk",
	Name:        "talk",
	Description: "Toggle a role permissions to send messages",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageChannels) {
			return ctx.Reply("[!] You need Manage Channels permission.")
		}
		if len(ctx.Args) < 2 {
			return ctx.SendHelp("talk")
		}
		gid := ctx.GuildID()
		cid := strings.Trim(ctx.Args[0], "<#>")
		roleArg := ctx.Args[1]
		var rid string
		if m := rxUtilityRole.FindStringSubmatch(roleArg); len(m) > 1 {
			rid = m[1]
		} else {
			rid = roleArg
		}

		ch, err := ctx.Session.Channel(cid)
		if err != nil || ch.GuildID != gid {
			return ctx.Reply("[!] Invalid channel.")
		}

		var currentOverwrite *discordgo.PermissionOverwrite
		for _, o := range ch.PermissionOverwrites {
			if o.ID == rid && o.Type == discordgo.PermissionOverwriteTypeRole {
				currentOverwrite = o
				break
			}
		}

		allowSend := false
		if currentOverwrite != nil && (currentOverwrite.Allow&discordgo.PermissionSendMessages) != 0 {
			allowSend = true
		}

		if allowSend {
			// Change to deny SendMessages
			err = ctx.ChannelPermissionSet(cid, rid, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionSendMessages, "Talk disabled")
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to deny talk: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Role <@&%s> is now denied from sending messages in <#%s>.", rid, cid))
		} else {
			// Change to allow SendMessages
			err = ctx.ChannelPermissionSet(cid, rid, discordgo.PermissionOverwriteTypeRole, discordgo.PermissionSendMessages, 0, "Talk enabled")
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to allow talk: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Role <@&%s> is now allowed to send messages in <#%s>.", rid, cid))
		}
	},
}

var RevokeFiles = &manager.Command{
	Trigger:     "revokefiles",
	Name:        "revokefiles",
	Description: "Enable/disable file attachments & links permission for everyone",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageChannels) {
			return ctx.Reply("[!] You need Manage Channels permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("revokefiles")
		}
		action := strings.ToLower(ctx.Args[0])
		if action != "on" && action != "off" {
			return ctx.Reply("[!] Specify action: `on` or `off`.")
		}

		gid := ctx.GuildID()
		cid := ctx.ChanID()
		if len(ctx.Args) > 1 {
			cid = strings.Trim(ctx.Args[1], "<#>")
		}

		ch, err := ctx.Session.Channel(cid)
		if err != nil || ch.GuildID != gid {
			return ctx.Reply("[!] Invalid channel.")
		}

		everyoneRoleID := gid
		const filePerms = discordgo.PermissionAttachFiles | discordgo.PermissionEmbedLinks

		if action == "off" {
			// Disable files/links
			err = ctx.ChannelPermissionSet(cid, everyoneRoleID, discordgo.PermissionOverwriteTypeRole, 0, filePerms, "Files revoked")
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to revoke permissions: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] File attachments and embed links revoked for everyone in <#%s>.", cid))
		} else {
			// Enable files/links
			err = ctx.ChannelPermissionSet(cid, everyoneRoleID, discordgo.PermissionOverwriteTypeRole, filePerms, 0, "Files granted")
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to grant permissions: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] File attachments and embed links granted to everyone in <#%s>.", cid))
		}
	},
}

var RestrictCommand = &manager.Command{
	Trigger:     "restrictcommand",
	Aliases:     []string{"rc"},
	Name:        "restrictcommand",
	Description: "Restrict a command to a specific role",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("restrictcommand")
		}
		gid := ctx.GuildID()
		sub := strings.ToLower(ctx.Args[0])

		if sub == "list" {
			m, _ := ctx.DB.ListRestrictedCommands(gid)
			if len(m) == 0 {
				return ctx.Reply("[*] No restricted commands configured.")
			}
			var sb strings.Builder
			sb.WriteString("Restricted Commands:\n\n")
			for cmd, roleID := range m {
				sb.WriteString(fmt.Sprintf("- Command `%s` restricted to <@&%s>\n", cmd, roleID))
			}
			return ctx.Reply(sb.String())
		}

		if sub == "reset" {
			_ = ctx.DB.DeleteAllRestrictedCommands(gid)
			return ctx.Reply("[+] All command restrictions successfully reset.")
		}

		if sub == "remove" {
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.restrictcommand remove <command>`")
			}
			cmdName := strings.ToLower(ctx.Args[1])
			_ = ctx.DB.DeleteRestrictedCommand(gid, cmdName)
			return ctx.Reply(fmt.Sprintf("[+] Removed restriction from command `%s`.", cmdName))
		}

		if len(ctx.Args) < 2 {
			return ctx.Reply("Usage: `.restrictcommand <command> <@role>`")
		}

		cmdName := strings.ToLower(ctx.Args[0])
		roleArg := ctx.Args[1]
		var rid string
		if m := rxUtilityRole.FindStringSubmatch(roleArg); len(m) > 1 {
			rid = m[1]
		} else {
			rid = roleArg
		}

		_, err := ctx.Session.State.Role(gid, rid)
		if err != nil {
			if roles, err := ctx.Session.GuildRoles(gid); err == nil {
				found := false
				for _, r := range roles {
					if r.ID == rid {
						found = true
						break
					}
				}
				if !found {
					return ctx.Reply("[!] Invalid role specified.")
				}
			}
		}

		_ = ctx.DB.SaveRestrictedCommand(gid, cmdName, rid)
		return ctx.Reply(fmt.Sprintf("[+] Command `%s` restricted to <@&%s>.", cmdName, rid))
	},
}

var Topic = &manager.Command{
	Trigger:     "topic",
	Name:        "topic",
	Description: "Set current channel topic",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageChannels) {
			return ctx.Reply("[!] You need Manage Channels permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: `.topic <text>`")
		}
		cid := ctx.ChanID()
		topicText := strings.Join(ctx.Args, " ")

		_, err := ctx.Session.ChannelEditComplex(cid, &discordgo.ChannelEdit{
			Topic: topicText,
		})
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to set channel topic: %v", err))
		}
		return ctx.Reply("[+] Successfully changed channel topic.")
	},
}

var Naughty = &manager.Command{
	Trigger:     "naughty",
	Name:        "naughty",
	Description: "Temporarily toggle text channel NSFW status",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageChannels) {
			return ctx.Reply("[!] You need Manage Channels permission.")
		}
		cid := ctx.ChanID()
		if len(ctx.Args) > 0 {
			cid = strings.Trim(ctx.Args[0], "<#>")
		}

		ch, err := ctx.Session.Channel(cid)
		if err != nil || ch.GuildID != ctx.GuildID() {
			return ctx.Reply("[!] Invalid channel.")
		}

		nsfw := true
		_, err = ctx.Session.ChannelEditComplex(cid, &discordgo.ChannelEdit{
			NSFW: &nsfw,
		})
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to set NSFW: %v", err))
		}

		_ = ctx.Reply(fmt.Sprintf("[+] Channel <#%s> is now NSFW for 30 seconds.", cid))

		go func(c string) {
			time.Sleep(30 * time.Second)
			restoreNSFW := false
			_, _ = ctx.Session.ChannelEditComplex(c, &discordgo.ChannelEdit{
				NSFW: &restoreNSFW,
			})
		}(cid)

		return nil
	},
}
