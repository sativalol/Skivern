package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"skyvern/internal/storage"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("history", []manager.HelpPage{
		{
			Command:     "History View User",
			Syntax:      ".history <member>",
			Description: "List all moderation history and cases for a specific user.",
		},
		{
			Command:     "History View Case",
			Syntax:      ".history view <case_id>",
			Description: "Show details about a specific moderation case ID.",
		},
		{
			Command:     "History Remove Case",
			Syntax:      ".history remove <member> <case_id>",
			Description: "Delete a specific moderation case for a user.",
		},
		{
			Command:     "History Remove All Cases",
			Syntax:      ".history removeall <member>",
			Description: "Delete all historical moderation cases for a user.",
		},
	})
}

var Warn = &manager.Command{
	Trigger:     "warn",
	Name:        "warn",
	Description: "Warn a member",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageMessages) {
			return ctx.Reply("[!] You need Manage Messages permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: warn <user> [reason]")
		}
		m, err := moderation.ResolveMember(ctx.Session, ctx.GuildID(), ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		reason := strings.Join(ctx.Args[1:], " ")
		if reason == "" {
			reason = "No reason provided."
		}

		c := storage.Case{
			UserID:    m.User.ID,
			ModID:     ctx.AuthorID(),
			Type:      "warn",
			Reason:    reason,
			Timestamp: time.Now(),
		}
		id, err := ctx.DB.AddCase(ctx.GuildID(), c)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to record warning: %v", err))
		}

		moderation.LogAction(ctx.Session, ctx.DB, ctx.GuildID(), fmt.Sprintf("Warn (Case #%d)", id), ctx.AuthorID(), m.User.ID, reason)
		return ctx.Reply(fmt.Sprintf("[+] Warned **%s** (Case #%d) | Reason: %s", m.User.Username, id, reason))
	},
}

var Unwarn = &manager.Command{
	Trigger:     "unwarn",
	Aliases:     []string{"rmwarn"},
	Name:        "unwarn",
	Description: "Remove a warning from a member by Case ID",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageMessages) {
			return ctx.Reply("[!] You need Manage Messages permission.")
		}
		if len(ctx.Args) < 1 {
			return ctx.Reply("Usage: unwarn <case_id>")
		}
		id, err := strconv.Atoi(ctx.Args[0])
		if err != nil {
			return ctx.Reply("[!] Invalid Case ID.")
		}

		c, err := ctx.DB.GetCase(ctx.GuildID(), id)
		if err != nil || c.Type != "warn" {
			return ctx.Reply(fmt.Sprintf("[!] Case #%d not found or is not a warning.", id))
		}

		if err := ctx.DB.DeleteCase(ctx.GuildID(), id); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to delete case: %v", err))
		}

		moderation.LogAction(ctx.Session, ctx.DB, ctx.GuildID(), fmt.Sprintf("Unwarn (Case #%d removed)", id), ctx.AuthorID(), c.UserID, "Warning revoked")
		return ctx.Reply(fmt.Sprintf("[+] Revoked warning Case #%d from <@%s>.", id, c.UserID))
	},
}

var Jail = &manager.Command{
	Trigger:     "jail",
	Name:        "jail",
	Description: "Jail a user by stripping roles and applying Jailed role",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageRoles) {
			return ctx.Reply("[!] You need Manage Roles permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: jail <user> [reason]")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}

		roles, err := ctx.Session.GuildRoles(gid)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to fetch guild roles: %v", err))
		}
		var jailRoleID string
		for _, r := range roles {
			if strings.ToLower(r.Name) == "jailed" {
				jailRoleID = r.ID
				break
			}
		}

		if jailRoleID == "" {
			name := "Jailed"
			role, err := ctx.Session.GuildRoleCreate(gid, &discordgo.RoleParams{Name: name})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to create Jailed role: %v", err))
			}
			jailRoleID = role.ID
		}

		var oldRoles []string
		for _, rid := range m.Roles {
			oldRoles = append(oldRoles, rid)
			_ = ctx.Session.GuildMemberRoleRemove(gid, m.User.ID, rid)
		}
		_ = ctx.DB.SaveJailed(gid, m.User.ID, oldRoles)
		_ = ctx.Session.GuildMemberRoleAdd(gid, m.User.ID, jailRoleID)

		reason := strings.Join(ctx.Args[1:], " ")
		if reason == "" {
			reason = "No reason provided."
		}

		c := storage.Case{
			UserID:    m.User.ID,
			ModID:     ctx.AuthorID(),
			Type:      "jail",
			Reason:    reason,
			Timestamp: time.Now(),
		}
		id, err := ctx.DB.AddCase(gid, c)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Jail succeeded, but failed to log case: %v", err))
		}

		moderation.LogAction(ctx.Session, ctx.DB, gid, fmt.Sprintf("Jail (Case #%d)", id), ctx.AuthorID(), m.User.ID, reason)
		return ctx.Reply(fmt.Sprintf("[+] Jailed **%s** (Case #%d) | Reason: %s", m.User.Username, id, reason))
	},
}

var Unjail = &manager.Command{
	Trigger:     "unjail",
	Name:        "unjail",
	Description: "Restore roles and remove Jailed role from a user",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageRoles) {
			return ctx.Reply("[!] You need Manage Roles permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: unjail <user>")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}

		roles, err := ctx.Session.GuildRoles(gid)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to fetch guild roles: %v", err))
		}
		var jailRoleID string
		for _, r := range roles {
			if strings.ToLower(r.Name) == "jailed" {
				jailRoleID = r.ID
				break
			}
		}

		if jailRoleID != "" {
			_ = ctx.Session.GuildMemberRoleRemove(gid, m.User.ID, jailRoleID)
		}

		oldRoles, err := ctx.DB.GetJailed(gid, m.User.ID)
		if err == nil {
			for _, rid := range oldRoles {
				_ = ctx.Session.GuildMemberRoleAdd(gid, m.User.ID, rid)
			}
			_ = ctx.DB.DeleteJailed(gid, m.User.ID)
		}

		moderation.LogAction(ctx.Session, ctx.DB, gid, "Unjail", ctx.AuthorID(), m.User.ID, "Unjailed member")
		return ctx.Reply(fmt.Sprintf("[+] Unjailed **%s**.", m.User.Username))
	},
}

var Lockdown = &manager.Command{
	Trigger:     "lockdown",
	Aliases:     []string{"lock"},
	Name:        "lockdown",
	Description: "Lock down a channel or all text channels",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageChannels) {
			return ctx.Reply("[!] You need Manage Channels permission.")
		}
		gid := ctx.GuildID()

		target := ctx.ChanID()
		lockAll := false
		if len(ctx.Args) > 0 {
			if strings.ToLower(ctx.Args[0]) == "all" {
				lockAll = true
			} else {
				if ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[0]); err == nil && ch != nil {
					target = ch.ID
				} else {
					target = strings.Trim(ctx.Args[0], "<#>")
				}
			}
		}

		everyoneRoleID := gid

		lockChan := func(chID string) error {
			return ctx.ChannelPermissionSet(chID, everyoneRoleID, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionSendMessages, "Channel lockdown")
		}

		if lockAll {
			chans, err := ctx.Session.GuildChannels(gid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to get channels: %v", err))
			}
			for _, c := range chans {
				if c.Type == discordgo.ChannelTypeGuildText {
					_ = lockChan(c.ID)
				}
			}
			return ctx.Reply("[+] Locked down all text channels.")
		}

		if err := lockChan(target); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to lock channel: %v", err))
		}
		return ctx.Reply(fmt.Sprintf("[+] Channel <#%s> locked.", target))
	},
}

var Unlock = &manager.Command{
	Trigger:     "unlock",
	Name:        "unlock",
	Description: "Unlock a channel or all text channels",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageChannels) {
			return ctx.Reply("[!] You need Manage Channels permission.")
		}
		gid := ctx.GuildID()

		target := ctx.ChanID()
		unlockAll := false
		if len(ctx.Args) > 0 {
			if strings.ToLower(ctx.Args[0]) == "all" {
				unlockAll = true
			} else {
				if ch, err := moderation.ResolveChannel(ctx.Session, gid, ctx.Args[0]); err == nil && ch != nil {
					target = ch.ID
				} else {
					target = strings.Trim(ctx.Args[0], "<#>")
				}
			}
		}

		everyoneRoleID := gid

		unlockChan := func(chID string) error {
			return ctx.ChannelPermissionDelete(chID, everyoneRoleID, "Channel unlock")
		}

		if unlockAll {
			chans, err := ctx.Session.GuildChannels(gid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to get channels: %v", err))
			}
			for _, c := range chans {
				if c.Type == discordgo.ChannelTypeGuildText {
					_ = unlockChan(c.ID)
				}
			}
			return ctx.Reply("[+] Unlocked all text channels.")
		}

		if err := unlockChan(target); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to unlock channel: %v", err))
		}
		return ctx.Reply(fmt.Sprintf("[+] Channel <#%s> unlocked.", target))
	},
}

var StripStaff = &manager.Command{
	Trigger:     "stripstaff",
	Aliases:     []string{"strip"},
	Name:        "stripstaff",
	Description: "Remove all staff roles from a targeted member",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionAdministrator) {
			return ctx.Reply("[!] You need Administrator permission to use this.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: stripstaff <user>")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}

		roles, err := ctx.Session.GuildRoles(gid)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to fetch roles: %v", err))
		}
		roleMap := make(map[string]*discordgo.Role)
		for _, r := range roles {
			roleMap[r.ID] = r
		}

		strippedCount := 0
		for _, rid := range m.Roles {
			r, ok := roleMap[rid]
			if !ok {
				continue
			}
			const staffPerms = discordgo.PermissionAdministrator |
				discordgo.PermissionBanMembers |
				discordgo.PermissionKickMembers |
				discordgo.PermissionManageChannels |
				discordgo.PermissionManageGuild |
				discordgo.PermissionManageMessages |
				discordgo.PermissionManageRoles

			if (r.Permissions & staffPerms) != 0 {
				if err := ctx.Session.GuildMemberRoleRemove(gid, m.User.ID, rid); err == nil {
					strippedCount++
				}
			}
		}

		moderation.LogAction(ctx.Session, ctx.DB, gid, "Strip Staff", ctx.AuthorID(), m.User.ID, fmt.Sprintf("Removed %d staff roles", strippedCount))
		return ctx.Reply(fmt.Sprintf("[+] Stripped %d staff roles from **%s**.", strippedCount, m.User.Username))
	},
}

var History = &manager.Command{
	Trigger:     "history",
	Aliases:     []string{"cases", "h"},
	Name:        "history",
	Description: "View and manage moderation history/cases",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageMessages) {
			return ctx.Reply("[!] You need Manage Messages permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("history")
		}

		gid := ctx.GuildID()
		sub := strings.ToLower(ctx.Args[0])

		switch sub {
		case "view":
			if len(ctx.Args) < 2 {
				return ctx.Reply("[!] Specify a Case ID.")
			}
			cid, err := strconv.Atoi(ctx.Args[1])
			if err != nil {
				return ctx.Reply("[!] Invalid Case ID.")
			}
			c, err := ctx.DB.GetCase(gid, cid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Case #%d not found.", cid))
			}
			emb := config.Build(ctx.Cfg, config.EmbedOpt{
				Title:       fmt.Sprintf("Case #%d | %s", c.ID, strings.Title(c.Type)),
				Description: fmt.Sprintf("**User:** <@%s> (`%s`)\n**Moderator:** <@%s> (`%s`)\n**Reason:** %s\n**Date:** %s", c.UserID, c.UserID, c.ModID, c.ModID, c.Reason, c.Timestamp.Format("2006-01-02 15:04:05")),
			})
			return ctx.Respond(emb)

		case "remove":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: history remove <member> <case_id>")
			}
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			cid, err := strconv.Atoi(ctx.Args[2])
			if err != nil {
				return ctx.Reply("[!] Invalid Case ID.")
			}
			c, err := ctx.DB.GetCase(gid, cid)
			if err != nil || c.UserID != m.User.ID {
				return ctx.Reply(fmt.Sprintf("[!] Case #%d not found for this member.", cid))
			}
			_ = ctx.DB.DeleteCase(gid, cid)
			return ctx.Reply(fmt.Sprintf("[+] Removed Case #%d from **%s**.", cid, m.User.Username))

		case "removeall":
			if !checkPerm(ctx, discordgo.PermissionAdministrator) {
				return ctx.Reply("[!] Only Administrators can purge all history.")
			}
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: history removeall <member>")
			}
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			_ = ctx.DB.DeleteAllCases(gid, m.User.ID)
			return ctx.Reply(fmt.Sprintf("[+] Removed all cases/history for **%s**.", m.User.Username))

		default:
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			list, err := ctx.DB.ListCases(gid, m.User.ID)
			if err != nil || len(list) == 0 {
				return ctx.Reply(fmt.Sprintf("[+] No moderation history found for **%s**.", m.User.Username))
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Cases for **%s**:\n\n", m.User.Username))
			for _, c := range list {
				sb.WriteString(fmt.Sprintf("`#%d` | **%s** | Reason: %s | Mod: <@%s> | %s\n", c.ID, strings.Title(c.Type), c.Reason, c.ModID, c.Timestamp.Format("2006-01-02")))
			}
			return ctx.Reply(sb.String())
		}
	},
}

var ModStats = &manager.Command{
	Trigger:     "modstats",
	Aliases:     []string{"ms"},
	Name:        "modstats",
	Description: "Shows moderation statistics per moderator",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageMessages) {
			return ctx.Reply("[!] You need Manage Messages permission.")
		}
		gid := ctx.GuildID()
		list, err := ctx.DB.ListCases(gid, "")
		if err != nil || len(list) == 0 {
			return ctx.Reply("[+] No cases recorded in this guild.")
		}

		counts := make(map[string]int)
		for _, c := range list {
			counts[c.ModID]++
		}

		var sb strings.Builder
		sb.WriteString("Moderator Action Counts:\n\n")
		for modID, count := range counts {
			sb.WriteString(fmt.Sprintf("<@%s> (`%s`): %d cases\n", modID, modID, count))
		}
		return ctx.Reply(sb.String())
	},
}
