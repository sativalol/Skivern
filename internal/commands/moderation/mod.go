package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"skyvern/internal/storage"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("timeout", []manager.HelpPage{
		{
			Command:     "Timeout Member",
			Syntax:      ".timeout <user> <duration>",
			Description: "Timeout a member. Duration supports: m (minutes), h (hours), d (days). e.g. .timeout @user 10m",
		},
	})
	manager.RegisterHelp("tempban", []manager.HelpPage{
		{
			Command:     "Temporary Ban",
			Syntax:      ".tempban <user> <duration>",
			Description: "Bans a user temporarily. Duration supports: m, h, d. e.g. .tempban @user 1d",
		},
	})
}

var Ban = &manager.Command{
	Trigger:     "ban",
	Aliases:     []string{"b"},
	Name:        "ban",
	Description: "Ban a user from the server",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionBanMembers) {
			return ctx.Reply("[!] You do not have permission to ban members.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: ban <user> [reason]")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}
		r := strings.Join(ctx.Args[1:], " ")
		if r == "" {
			r = "No reason provided."
		}

		moderation.DMUserAction(ctx.Session, gid, "Ban", m.User.ID, ctx.AuthorID(), r)

		if err := ctx.Ban(m.User.ID, r, 0); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to ban: %v", err))
		}

		c := storage.Case{
			UserID:    m.User.ID,
			ModID:     ctx.AuthorID(),
			Type:      "ban",
			Reason:    r,
			Timestamp: time.Now(),
		}
		id, _ := ctx.DB.AddCase(gid, c)

		moderation.LogAction(ctx.Session, ctx.DB, gid, fmt.Sprintf("Ban (Case #%d)", id), ctx.AuthorID(), m.User.ID, r)
		return ctx.Reply(fmt.Sprintf("[+] Banned **%s** (Case #%d) | Reason: %s", m.User.Username, id, r))
	},
}

var Unban = &manager.Command{
	Trigger:     "unban",
	Aliases:     []string{"ub"},
	Name:        "unban",
	Description: "Unban a user by their ID",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionBanMembers) {
			return ctx.Reply("[!] You do not have permission to unban members.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: unban <user_id>")
		}
		gid := ctx.GuildID()
		uid := ctx.Args[0]

		moderation.DMUserAction(ctx.Session, gid, "Unban", uid, ctx.AuthorID(), "Manual unban command")

		if err := ctx.Unban(uid, "Manual unban command"); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to unban: %v", err))
		}
		moderation.LogAction(ctx.Session, ctx.DB, gid, "Unban", ctx.AuthorID(), uid, "Manual unban command")
		return ctx.Reply(fmt.Sprintf("[+] Unbanned user ID **%s**.", uid))
	},
}

var Hardban = &manager.Command{
	Trigger:     "hardban",
	Aliases:     []string{"hb"},
	Name:        "hardban",
	Description: "Ban user and purge their messages",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionBanMembers) {
			return ctx.Reply("[!] You do not have permission to ban members.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: hardban <user> [reason]")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}
		r := strings.Join(ctx.Args[1:], " ") + " (Purge 7d)"

		moderation.DMUserAction(ctx.Session, gid, "Hardban", m.User.ID, ctx.AuthorID(), r)

		if err := ctx.Ban(m.User.ID, r, 7); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to hardban: %v", err))
		}

		c := storage.Case{
			UserID:    m.User.ID,
			ModID:     ctx.AuthorID(),
			Type:      "hardban",
			Reason:    r,
			Timestamp: time.Now(),
		}
		id, _ := ctx.DB.AddCase(gid, c)

		moderation.LogAction(ctx.Session, ctx.DB, gid, fmt.Sprintf("Hardban (Case #%d)", id), ctx.AuthorID(), m.User.ID, r)
		return ctx.Reply(fmt.Sprintf("[+] Hardbanned **%s** (Case #%d) and purged message history.", m.User.Username, id))
	},
}

var Softban = &manager.Command{
	Trigger:     "softban",
	Aliases:     []string{"sb"},
	Name:        "softban",
	Description: "Kick user and purge their messages via quick ban/unban",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionBanMembers) {
			return ctx.Reply("[!] You do not have permission to ban members.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: softban <user>")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}
		reason := "Softban (ban & unban to purge messages)"

		moderation.DMUserAction(ctx.Session, gid, "Softban", m.User.ID, ctx.AuthorID(), reason)

		if err := ctx.Ban(m.User.ID, "Softban (Purge)", 7); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed ban phase: %v", err))
		}
		_ = ctx.Unban(m.User.ID, "Softban completion")

		c := storage.Case{
			UserID:    m.User.ID,
			ModID:     ctx.AuthorID(),
			Type:      "softban",
			Reason:    reason,
			Timestamp: time.Now(),
		}
		id, _ := ctx.DB.AddCase(gid, c)

		moderation.LogAction(ctx.Session, ctx.DB, gid, fmt.Sprintf("Softban (Case #%d)", id), ctx.AuthorID(), m.User.ID, reason)
		return ctx.Reply(fmt.Sprintf("[+] Softbanned and kicked **%s** (Case #%d) (purged messages).", m.User.Username, id))
	},
}

var Tempban = &manager.Command{
	Trigger:     "tempban",
	Aliases:     []string{"tb"},
	Name:        "tempban",
	Description: "Temporarily ban a user",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionBanMembers) {
			return ctx.Reply("[!] You do not have permission to ban members.")
		}
		if len(ctx.Args) < 2 {
			return ctx.SendHelp("tempban")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}

		durStr := ctx.Args[1]
		lastChar := durStr[len(durStr)-1]
		if lastChar >= '0' && lastChar <= '9' {
			durStr += "m"
		}
		dur, err := time.ParseDuration(durStr)
		if err != nil {
			return ctx.Reply("[!] Invalid duration. Use e.g. 60m, 2h, 1d.")
		}
		reason := "No reason provided."
		if len(ctx.Args) > 2 {
			reason = strings.Join(ctx.Args[2:], " ")
		}

		moderation.DMUserAction(ctx.Session, gid, "Tempban", m.User.ID, ctx.AuthorID(), fmt.Sprintf("Duration: %s | Reason: %s", dur.String(), reason))

		if err := ctx.Ban(m.User.ID, fmt.Sprintf("Tempban: %s | Reason: %s", dur.String(), reason), 0); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed tempban: %v", err))
		}

		c := storage.Case{
			UserID:    m.User.ID,
			ModID:     ctx.AuthorID(),
			Type:      "tempban",
			Reason:    fmt.Sprintf("Tempban: %s | Reason: %s", dur.String(), reason),
			Timestamp: time.Now(),
		}
		id, _ := ctx.DB.AddCase(gid, c)

		moderation.LogAction(ctx.Session, ctx.DB, gid, fmt.Sprintf("Tempban (Case #%d)", id), ctx.AuthorID(), m.User.ID, reason, config.Field("Duration", dur.String(), true))
		go func() {
			time.Sleep(dur)
			_ = ctx.Unban(m.User.ID, "Temporary ban expired")
			moderation.LogAction(ctx.Session, ctx.DB, gid, "Tempban Expired (Auto-Unban)", ctx.Session.State.User.ID, m.User.ID, "Automatic temporary ban expiration")
		}()
		return ctx.Reply(fmt.Sprintf("[+] Tempbanned **%s** (Case #%d) for %s | Reason: %s", m.User.Username, id, dur.String(), reason))
	},
}

var Kick = &manager.Command{
	Trigger:     "kick",
	Aliases:     []string{"k"},
	Name:        "kick",
	Description: "Kick a user from the server",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionKickMembers) {
			return ctx.Reply("[!] You do not have permission to kick members.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: kick <user>")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}
		reason := "Manual kick command"

		moderation.DMUserAction(ctx.Session, gid, "Kick", m.User.ID, ctx.AuthorID(), reason)

		if err := ctx.Kick(m.User.ID, reason); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to kick: %v", err))
		}

		c := storage.Case{
			UserID:    m.User.ID,
			ModID:     ctx.AuthorID(),
			Type:      "kick",
			Reason:    reason,
			Timestamp: time.Now(),
		}
		id, _ := ctx.DB.AddCase(gid, c)

		moderation.LogAction(ctx.Session, ctx.DB, gid, fmt.Sprintf("Kick (Case #%d)", id), ctx.AuthorID(), m.User.ID, reason)
		return ctx.Reply(fmt.Sprintf("[+] Kicked **%s** (Case #%d).", m.User.Username, id))
	},
}

var Timeout = &manager.Command{
	Trigger:     "timeout",
	Aliases:     []string{"to", "time"},
	Name:        "timeout",
	Description: "Timeout a user",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionModerateMembers) {
			return ctx.Reply("[!] You do not have permission to moderate members.")
		}
		if len(ctx.Args) < 2 {
			return ctx.SendHelp("timeout")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}

		durStr := ctx.Args[1]
		lastChar := durStr[len(durStr)-1]
		if lastChar >= '0' && lastChar <= '9' {
			durStr += "m"
		}
		dur, err := time.ParseDuration(durStr)
		if err != nil {
			return ctx.Reply("[!] Invalid duration. Use e.g. 15m, 2h, 1d.")
		}
		reason := "No reason provided."
		if len(ctx.Args) > 2 {
			reason = strings.Join(ctx.Args[2:], " ")
		}
		until := time.Now().Add(dur)

		moderation.DMUserAction(ctx.Session, gid, "Timeout", m.User.ID, ctx.AuthorID(), fmt.Sprintf("Duration: %s | Reason: %s", dur.String(), reason))

		if err := ctx.Timeout(m.User.ID, &until, reason); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to timeout: %v", err))
		}

		c := storage.Case{
			UserID:    m.User.ID,
			ModID:     ctx.AuthorID(),
			Type:      "timeout",
			Reason:    reason,
			Timestamp: time.Now(),
		}
		id, _ := ctx.DB.AddCase(gid, c)

		moderation.LogAction(ctx.Session, ctx.DB, gid, fmt.Sprintf("Timeout (Case #%d)", id), ctx.AuthorID(), m.User.ID, reason, config.Field("Duration", dur.String(), true))
		return ctx.Reply(fmt.Sprintf("[+] Timed out **%s** (Case #%d) until %s | Reason: %s", m.User.Username, id, until.Format("15:04:05"), reason))
	},
}

var Untimeout = &manager.Command{
	Trigger:     "untimeout",
	Aliases:     []string{"uto", "untime"},
	Name:        "untimeout",
	Description: "Remove timeout from a user",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionModerateMembers) {
			return ctx.Reply("[!] You do not have permission to moderate members.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: untimeout <user>")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}
		reason := "Manual untimeout command"

		moderation.DMUserAction(ctx.Session, gid, "Untimeout", m.User.ID, ctx.AuthorID(), reason)

		if err := ctx.Timeout(m.User.ID, nil, reason); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to remove timeout: %v", err))
		}
		moderation.LogAction(ctx.Session, ctx.DB, gid, "Untimeout", ctx.AuthorID(), m.User.ID, reason)
		return ctx.Reply(fmt.Sprintf("[+] Removed timeout from **%s**.", m.User.Username))
	},
}

var Nickname = &manager.Command{
	Trigger:     "nickname",
	Aliases:     []string{"nick"},
	Name:        "nickname",
	Description: "Change a user's nickname",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionManageNicknames) {
			return ctx.Reply("[!] You do not have permission to manage nicknames.")
		}
		if len(ctx.Args) < 2 {
			return ctx.Reply("Usage: nickname <user> <new_nickname>")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}
		nick := strings.Join(ctx.Args[1:], " ")
		if err := ctx.Nick(m.User.ID, nick, fmt.Sprintf("New nickname: %s", nick)); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to change nickname: %v", err))
		}
		moderation.LogAction(ctx.Session, ctx.DB, gid, "Nickname Change", ctx.AuthorID(), m.User.ID, fmt.Sprintf("New nickname: %s", nick))
		return ctx.Reply(fmt.Sprintf("[+] Changed **%s** nickname to **%s**.", m.User.Username, nick))
	},
}

var ForceNick = &manager.Command{
	Trigger:     "forcenick",
	Aliases:     []string{"fnick", "forcename", "fn"},
	Name:        "forcenick",
	Description: "Locks a user's nickname so they cannot change it",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionManageNicknames) {
			return ctx.Reply("[!] You do not have permission to manage nicknames.")
		}
		if len(ctx.Args) < 2 {
			return ctx.Reply("Usage: forcenick <user> <nickname>")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}
		nick := strings.Join(ctx.Args[1:], " ")
		_ = ctx.DB.SaveNicklock(gid, m.User.ID, nick)
		_ = ctx.Nick(m.User.ID, nick, fmt.Sprintf("Locked nickname: %s", nick))
		moderation.LogAction(ctx.Session, ctx.DB, gid, "Nickname Force Lock", ctx.AuthorID(), m.User.ID, fmt.Sprintf("Locked nickname: %s", nick))
		return ctx.Reply(fmt.Sprintf("[Locked] Nickname lock active for **%s** -> **%s**.", m.User.Username, nick))
	},
}

var UnforceNick = &manager.Command{
	Trigger:     "unforcenick",
	Aliases:     []string{"ufnick", "unforcename", "unfn"},
	Name:        "unforcenick",
	Description: "Unlock a user's nickname",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Cfg.EmbedColor = 0x808080
		if !checkPerm(ctx, discordgo.PermissionManageNicknames) {
			return ctx.Reply("[!] You do not have permission to manage nicknames.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: unforcenick <user>")
		}
		gid := ctx.GuildID()
		m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[0])
		if err != nil || m == nil {
			return ctx.Reply("[!] Could not resolve member.")
		}
		if !checkHierarchy(ctx, m.User.ID) {
			return ctx.Reply("[!] You cannot moderate this member due to role hierarchy.")
		}
		_ = ctx.DB.DeleteNicklock(gid, m.User.ID)
		moderation.LogAction(ctx.Session, ctx.DB, gid, "Nickname Force Unlock", ctx.AuthorID(), m.User.ID, "Manual nickname unlock")
		return ctx.Reply(fmt.Sprintf("[Unlocked] Nickname lock removed for **%s**.", m.User.Username))
	},
}
