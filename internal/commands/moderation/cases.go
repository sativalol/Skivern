package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"skyvern/internal/storage"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var Warn = &manager.Command{
	Trigger:     "warn",
	Aliases:     []string{"w"},
	Name:        "warn",
	Description: "Issue a warning to a member",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
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

		moderation.DMUserAction(ctx.Session, ctx.GuildID(), "Warn", m.User.ID, ctx.AuthorID(), reason)

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
		ctx.Cfg.EmbedColor = 0x808080
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

		moderation.DMUserAction(ctx.Session, ctx.GuildID(), "Unwarn", c.UserID, ctx.AuthorID(), "Warning revoked")

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
		ctx.Cfg.EmbedColor = 0x808080
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

		moderation.DMUserAction(ctx.Session, gid, "Jail", m.User.ID, ctx.AuthorID(), reason)

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
		ctx.Cfg.EmbedColor = 0x808080
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

		moderation.DMUserAction(ctx.Session, gid, "Unjail", m.User.ID, ctx.AuthorID(), "Unjailed member")

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
		ctx.Cfg.EmbedColor = 0x808080
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
		ctx.Cfg.EmbedColor = 0x808080
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
		ctx.Cfg.EmbedColor = 0x808080
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
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionManageMessages) {
			return ctx.Reply("[!] You need Manage Messages permission.")
		}

		gid := ctx.GuildID()

		// If no argument, give correct syntax error as shown in screenshot
		if len(ctx.Args) == 0 {
			return ctx.Reply("[!] Incorrect syntax. Try checking help for `history`.")
		}

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

			authorName := ctx.AuthorTag()
			var authorAvatar string
			if ctx.Interact != nil && ctx.Interact.Member != nil && ctx.Interact.Member.User != nil {
				authorAvatar = ctx.Interact.Member.User.AvatarURL("64")
			} else if ctx.Message != nil && ctx.Message.Author != nil {
				authorAvatar = ctx.Message.Author.AvatarURL("64")
			}

			emb, comps := buildHistoryResponse(ctx.Session, m.User.Username, authorName, authorAvatar, list, 1, true)
			if ctx.Interact != nil {
				return ctx.Session.InteractionRespond(ctx.Interact, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Embeds:     []*discordgo.MessageEmbed{emb},
						Components: comps,
					},
				})
			}
			_, err = ctx.Session.ChannelMessageSendComplex(ctx.Message.ChannelID, &discordgo.MessageSend{
				Embeds:     []*discordgo.MessageEmbed{emb},
				Components: comps,
			})
			return err
		}
	},
}

var ModStats = &manager.Command{
	Trigger:     "modstats",
	Aliases:     []string{"ms"},
	Name:        "modstats",
	Description: "Shows moderation statistics for a user",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionManageMessages) {
			return ctx.Reply("[!] You need Manage Messages permission.")
		}
		gid := ctx.GuildID()

		var targetMember *discordgo.Member
		var err error
		if len(ctx.Args) > 0 {
			targetMember, err = moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
			if err != nil || targetMember == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
		} else {
			if ctx.Interact != nil && ctx.Interact.Member != nil {
				targetMember = ctx.Interact.Member
			} else if ctx.Message != nil && ctx.Message.Member != nil {
				targetMember = ctx.Message.Member
			} else if ctx.Message != nil && ctx.Message.Author != nil {
				targetMember, _ = ctx.Session.GuildMember(gid, ctx.Message.Author.ID)
			}
		}

		if targetMember == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}

		list, err := ctx.DB.ListCases(gid, "")
		if err != nil {
			return ctx.Reply("[+] No cases recorded in this guild.")
		}

		var w7, k7, b7, u7, t7, j7 int
		var w14, k14, b14, u14, t14, j14 int
		var wAll, kAll, bAll, uAll, tAll, jAll int

		now := time.Now()
		for _, c := range list {
			if c.UserID != targetMember.User.ID {
				continue
			}

			age := now.Sub(c.Timestamp)
			in7 := age <= 7*24*time.Hour
			in14 := age <= 14*24*time.Hour

			switch strings.ToLower(c.Type) {
			case "warn":
				wAll++
				if in7 { w7++ }
				if in14 { w14++ }
			case "kick":
				kAll++
				if in7 { k7++ }
				if in14 { k14++ }
			case "ban", "tempban", "softban", "hardban":
				bAll++
				if in7 { b7++ }
				if in14 { b14++ }
			case "unban":
				uAll++
				if in7 { u7++ }
				if in14 { u14++ }
			case "timeout":
				tAll++
				if in7 { t7++ }
				if in14 { t14++ }
			case "jail":
				jAll++
				if in7 { j7++ }
				if in14 { j14++ }
			}
		}

		val7 := fmt.Sprintf("> **Warned:** %d\n> **Kicked:** %d\n> **Banned:** %d\n> **Unbanned:** %d\n> **Timed Out:** %d\n> **Jailed:** %d", w7, k7, b7, u7, t7, j7)
		val14 := fmt.Sprintf("> **Warned:** %d\n> **Kicked:** %d\n> **Banned:** %d\n> **Unbanned:** %d\n> **Timed Out:** %d\n> **Jailed:** %d", w14, k14, b14, u14, t14, j14)
		valAll := fmt.Sprintf("> **Warned:** %d\n> **Kicked:** %d\n> **Banned:** %d\n> **Unbanned:** %d\n> **Timed Out:** %d\n> **Jailed:** %d", wAll, kAll, bAll, uAll, tAll, jAll)

		emb := &discordgo.MessageEmbed{
			Author: &discordgo.MessageEmbedAuthor{
				Name:    targetMember.User.Username,
				IconURL: targetMember.User.AvatarURL("64"),
			},
			Title: fmt.Sprintf("Moderation Statistics for %s", targetMember.User.Username),
			Color: 0x808080,
			Fields: []*discordgo.MessageEmbedField{
				config.Field("7 days", val7, true),
				config.Field("14 days", val14, true),
				config.Field("All time", valAll, true),
			},
		}

		return ctx.Respond(emb)
	},
}

func buildHistoryResponse(s *discordgo.Session, targetName, authorName, authorAvatar string, list []storage.Case, page int, desc bool) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	sort.Slice(list, func(i, j int) bool {
		if desc {
			return list[i].ID > list[j].ID
		}
		return list[i].ID < list[j].ID
	})

	pageSize := 3
	totalCases := len(list)
	totalPages := (totalCases + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if end > totalCases {
		end = totalCases
	}
	pageList := list[start:end]

	var sb strings.Builder
	for _, c := range pageList {
		actName := strings.Title(c.Type)
		switch strings.ToLower(c.Type) {
		case "warn":
			actName = "Warned"
		case "jail":
			actName = "Jailed"
		case "kick":
			actName = "Kicked"
		case "ban", "tempban", "softban", "hardban":
			actName = "Banned"
		case "unban":
			actName = "Unbanned"
		case "timeout":
			actName = "Timed Out"
		case "untimeout":
			actName = "Untimeouted"
		}

		sb.WriteString(fmt.Sprintf("**Case Log #%d | %s**\n", c.ID, actName))
		sb.WriteString(fmt.Sprintf("**Punished:** %s\n", c.Timestamp.Format("January 2, 2006 at 3:04 PM")))
		
		modName := c.ModID
		if modUser, err := s.User(c.ModID); err == nil && modUser != nil {
			modName = modUser.Username
		}
		sb.WriteString(fmt.Sprintf("**Moderator:** %s ( %s )\n", modName, c.ModID))
		sb.WriteString(fmt.Sprintf("**Reason:** %s\n\n", c.Reason))
	}

	sb.WriteString(fmt.Sprintf("Page %d/%d (%d punishments, 0 notes)", page, totalPages, totalCases))

	emb := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    authorName,
			IconURL: authorAvatar,
		},
		Title:       fmt.Sprintf("Punishment History for %s", targetName),
		Description: sb.String(),
		Color:       0x808080,
	}

	sortVal := 0
	if desc {
		sortVal = 1
	}

	prevPage := page - 1
	if prevPage < 1 {
		prevPage = totalPages
	}
	nextPage := page + 1
	if nextPage > totalPages {
		nextPage = 1
	}

	targetID := list[0].UserID

	row := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "◀",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("history_prev_%s_%d_%d", targetID, prevPage, sortVal),
				Disabled: totalPages <= 1,
			},
			discordgo.Button{
				Label:    "▶",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("history_next_%s_%d_%d", targetID, nextPage, sortVal),
				Disabled: totalPages <= 1,
			},
			discordgo.Button{
				Label:    "⇅",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("history_sort_%s_%d_%d", targetID, page, 1-sortVal),
			},
			discordgo.Button{
				Label:    "❌",
				Style:    discordgo.DangerButton,
				CustomID: fmt.Sprintf("history_close_%s_%d_%d", targetID, page, sortVal),
			},
		},
	}

	return emb, []discordgo.MessageComponent{row}
}

func HandleHistoryComponent(s *discordgo.Session, i *discordgo.InteractionCreate, mgr *manager.Manager) {
	id := i.MessageComponentData().CustomID
	parts := strings.Split(id, "_")
	if len(parts) < 5 {
		return
	}

	action := parts[1]
	targetID := parts[2]
	page, _ := strconv.Atoi(parts[3])
	sortVal, _ := strconv.Atoi(parts[4])
	desc := sortVal == 1

	if action == "close" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "[+] Punishment history closed.",
				Embeds:     nil,
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	gid := i.GuildID
	list, err := mgr.DB().ListCases(gid, targetID)
	if err != nil || len(list) == 0 {
		return
	}

	targetName := targetID
	if targetUser, err := s.User(targetID); err == nil && targetUser != nil {
		targetName = targetUser.Username
	}

	authorName := i.Member.User.Username
	authorAvatar := i.Member.User.AvatarURL("64")

	emb, comps := buildHistoryResponse(s, targetName, authorName, authorAvatar, list, page, desc)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{emb},
			Components: comps,
		},
	})
}
