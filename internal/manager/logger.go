package manager

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var permNames = map[int64]string{
	1:             "Create Instant Invite",
	2:             "Kick Members",
	4:             "Ban Members",
	8:             "Administrator",
	16:            "Manage Channels",
	32:            "Manage Server",
	64:            "Add Reactions",
	128:           "View Audit Log",
	256:           "Priority Speaker",
	512:           "Video",
	1024:          "Read Text Channels & See Voice Channels",
	2048:          "Send Messages",
	4096:          "Send TTS Messages",
	8192:          "Manage Messages",
	16384:         "Embed Links",
	32768:         "Attach Files",
	65536:         "Read Message History",
	131072:        "Mention @everyone, @here, and All Roles",
	262144:        "Use External Emojis",
	524288:        "View Server Insights",
	1048576:       "Connect",
	2097152:       "Speak",
	4194304:       "Mute Members",
	8388608:       "Deafen Members",
	16777216:      "Move Members",
	33554432:      "Use Voice Activity",
	67108864:      "Change Nickname",
	134217728:     "Manage Nicknames",
	268435456:     "Manage Roles",
	536870912:     "Manage Webhooks",
	1073741824:    "Manage Emojis and Stickers",
	2147483648:    "Use Application Commands",
	4294967296:    "Request to Speak",
	8589934592:    "Manage Events",
	17179869184:   "Manage Threads",
	34359738368:   "Create Public Threads",
	68719476736:   "Create Private Threads",
	137438953472:  "Use External Stickers",
	274877906944:  "Send Messages in Threads",
	549755813888:  "Use Embedded Activities",
	1099511627776: "Moderate Members",
}

func parseColor(hex string) int {
	hex = strings.TrimPrefix(hex, "#")
	val, err := strconv.ParseInt(hex, 16, 32)
	if err != nil {
		return 0
	}
	return int(val)
}

func snowflakeTimestamp(id string) time.Time {
	i, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli((i >> 22) + 1420070400000)
}

func (m *Manager) SendAuditLog(s *discordgo.Session, guildID, category string, embed *discordgo.MessageEmbed, userID, channelID string) {
	if pCfg, err := m.GetPalantirCfg(); err == nil && pCfg.Enabled {
		allowed := true
		for _, g := range pCfg.BlockedGuilds {
			if g == guildID {
				allowed = false
				break
			}
		}
		if allowed && channelID != "" {
			for _, c := range pCfg.BlockedChannels {
				if c == channelID {
					allowed = false
					break
				}
			}
		}
		if allowed && userID != "" {
			for _, u := range pCfg.BlockedUsers {
				if u == userID {
					allowed = false
					break
				}
			}
		}
		if allowed && category != "" {
			for _, e := range pCfg.BlockedEvents {
				if strings.EqualFold(e, category) {
					allowed = false
					break
				}
			}
		}
		if allowed {
			select {
			case m.palantirChan <- &PalantirLog{
				Timestamp: time.Now(),
				GuildID:   guildID,
				Category:  category,
				Title:     embed.Title,
				Desc:      embed.Description,
				UserID:    userID,
				ChannelID: channelID,
			}:
			default:
			}
		}
	}
	if guildID == "" {
		return
	}
	if userID != "" && m.db.IsLoggerIgnored(guildID, userID) {
		return
	}
	if channelID != "" && m.db.IsLoggerIgnored(guildID, channelID) {
		return
	}

	subs, err := m.db.GetLoggerSubs(guildID, category)
	if err != nil || len(subs) == 0 {
		return
	}

	for _, sub := range subs {
		logEmbed := &discordgo.MessageEmbed{
			Title:       embed.Title,
			Description: embed.Description,
			Color:       embed.Color,
			Fields:      embed.Fields,
			Thumbnail:   embed.Thumbnail,
			Image:       embed.Image,
			Footer:      embed.Footer,
			Timestamp:   embed.Timestamp,
			Author:      embed.Author,
		}
		if sub.EmbedColor != "" {
			logEmbed.Color = parseColor(sub.EmbedColor)
		} else if logEmbed.Color == 0 {
			logEmbed.Color = 0x2b2d31
		}
		if logEmbed.Footer == nil {
			logEmbed.Footer = &discordgo.MessageEmbedFooter{
				Text: "Audit Logging System",
			}
		}
		_, _ = s.ChannelMessageSendEmbed(sub.ChannelID, logEmbed)
	}
}

func (m *Manager) LogMessageDelete(s *discordgo.Session, e *discordgo.MessageDelete) {
	if e.BeforeDelete == nil || e.BeforeDelete.Author == nil || e.BeforeDelete.Author.Bot {
		return
	}
	desc := e.BeforeDelete.Content
	if desc == "" {
		desc = "*No text content*"
	}
	emb := &discordgo.MessageEmbed{
		Color: 0xff4747,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    e.BeforeDelete.Author.Username,
			IconURL: e.BeforeDelete.Author.AvatarURL("64"),
		},
		Title:       "Message Deleted",
		Description: desc,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Author", Value: fmt.Sprintf("<@%s> `%s`", e.BeforeDelete.Author.ID, e.BeforeDelete.Author.ID), Inline: true},
			{Name: "Channel", Value: fmt.Sprintf("<#%s>", e.ChannelID), Inline: true},
		},
	}
	if len(e.BeforeDelete.Attachments) > 0 {
		var atts []string
		for _, a := range e.BeforeDelete.Attachments {
			atts = append(atts, fmt.Sprintf("[%s](%s) (%d KB)", a.Filename, a.URL, a.Size/1024))
		}
		emb.Fields = append(emb.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("Attachments (%d)", len(e.BeforeDelete.Attachments)),
			Value:  strings.Join(atts, "\n"),
			Inline: false,
		})
	}
	m.SendAuditLog(s, e.GuildID, "messages", emb, e.BeforeDelete.Author.ID, e.ChannelID)
}

func (m *Manager) LogMessageUpdate(s *discordgo.Session, e *discordgo.MessageUpdate) {
	if e.BeforeUpdate == nil || e.BeforeUpdate.Author == nil || e.BeforeUpdate.Author.Bot {
		return
	}
	if e.BeforeUpdate.Content == e.Content {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color: 0x3498db,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    e.BeforeUpdate.Author.Username,
			IconURL: e.BeforeUpdate.Author.AvatarURL("64"),
		},
		Title: "Message Edited",
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Author", Value: fmt.Sprintf("<@%s> `%s`", e.BeforeUpdate.Author.ID, e.BeforeUpdate.Author.ID), Inline: true},
			{Name: "Channel", Value: fmt.Sprintf("<#%s>", e.ChannelID), Inline: true},
			{Name: "Before", Value: e.BeforeUpdate.Content, Inline: false},
			{Name: "After", Value: e.Content, Inline: false},
		},
	}
	m.SendAuditLog(s, e.GuildID, "messages", emb, e.BeforeUpdate.Author.ID, e.ChannelID)
}

func (m *Manager) LogMessageDeleteBulk(s *discordgo.Session, e *discordgo.MessageDeleteBulk) {
	emb := &discordgo.MessageEmbed{
		Color:       0xff4747,
		Title:       "Bulk Messages Deleted",
		Description: fmt.Sprintf("**%d** messages purged in <#%s>", len(e.Messages), e.ChannelID),
	}
	m.SendAuditLog(s, e.GuildID, "messages", emb, "", e.ChannelID)
}

func (m *Manager) LogReactionAdd(s *discordgo.Session, e *discordgo.MessageReactionAdd) {
	if e.UserID == s.State.User.ID {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color: 0x43b581,
		Title: "Reaction Added",
		Description: fmt.Sprintf("<@%s> reacted with %s on message `%s`", e.UserID, e.Emoji.MessageFormat(), e.MessageID),
	}
	m.SendAuditLog(s, e.GuildID, "messages", emb, e.UserID, e.ChannelID)
}

func (m *Manager) LogReactionRemove(s *discordgo.Session, e *discordgo.MessageReactionRemove) {
	emb := &discordgo.MessageEmbed{
		Color: 0xff4747,
		Title: "Reaction Removed",
		Description: fmt.Sprintf("<@%s> removed reaction %s on message `%s`", e.UserID, e.Emoji.MessageFormat(), e.MessageID),
	}
	m.SendAuditLog(s, e.GuildID, "messages", emb, e.UserID, e.ChannelID)
}

func (m *Manager) LogMemberJoin(s *discordgo.Session, e *discordgo.GuildMemberAdd) {
	if e.Member == nil || e.Member.User == nil {
		return
	}
	age := int(time.Since(snowflakeTimestamp(e.Member.User.ID)).Hours() / 24)
	emb := &discordgo.MessageEmbed{
		Color: 0x43b581,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    e.Member.User.Username,
			IconURL: e.Member.User.AvatarURL("64"),
		},
		Title:       "Member Joined",
		Description: fmt.Sprintf("<@%s> %s", e.Member.User.ID, e.Member.User.Username),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Account Age", Value: fmt.Sprintf("%d days", age), Inline: true},
			{Name: "ID", Value: fmt.Sprintf("`%s`", e.Member.User.ID), Inline: true},
		},
	}
	m.SendAuditLog(s, e.GuildID, "members", emb, e.Member.User.ID, "")
}

func (m *Manager) LogMemberLeave(s *discordgo.Session, e *discordgo.GuildMemberRemove) {
	if e.Member == nil || e.Member.User == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color: 0xff6b6b,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    e.Member.User.Username,
			IconURL: e.Member.User.AvatarURL("64"),
		},
		Title:       "Member Left",
		Description: fmt.Sprintf("%s `%s`", e.Member.User.Username, e.Member.User.ID),
	}
	m.SendAuditLog(s, e.GuildID, "members", emb, e.Member.User.ID, "")
}

func (m *Manager) LogMemberBan(s *discordgo.Session, e *discordgo.GuildBanAdd) {
	if e.User == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0xff4747,
		Title:       "Member Banned",
		Description: fmt.Sprintf("%s `%s`", e.User.Username, e.User.ID),
	}
	m.SendAuditLog(s, e.GuildID, "members", emb, e.User.ID, "")
}

func (m *Manager) LogMemberUnban(s *discordgo.Session, e *discordgo.GuildBanRemove) {
	if e.User == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0x43b581,
		Title:       "Member Unbanned",
		Description: fmt.Sprintf("%s `%s`", e.User.Username, e.User.ID),
	}
	m.SendAuditLog(s, e.GuildID, "members", emb, e.User.ID, "")
}

func (m *Manager) LogMemberUpdate(s *discordgo.Session, e *discordgo.GuildMemberUpdate) {
	if e.Member == nil || e.Member.User == nil {
		return
	}

	oldMem, err := s.State.Member(e.GuildID, e.Member.User.ID)

	var changes []string
	if err == nil && oldMem != nil {
		if oldMem.Nick != e.Member.Nick {
			oldNick := oldMem.Nick
			if oldNick == "" {
				oldNick = "*None*"
			}
			newNick := e.Member.Nick
			if newNick == "" {
				newNick = "*None*"
			}
			changes = append(changes, fmt.Sprintf("**Nickname:** `%s` → `%s`", oldNick, newNick))
		}
		if oldMem.Pending && !e.Member.Pending {
			changes = append(changes, "**Verification:** Completed community rules gating.")
		}

		if len(oldMem.Roles) != len(e.Member.Roles) {
			changes = append(changes, fmt.Sprintf("**Roles:** Updated (%d roles total)", len(e.Member.Roles)))
		}
	}

	if len(changes) == 0 {
		return
	}

	emb := &discordgo.MessageEmbed{
		Color: 0x3498db,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    e.Member.User.Username,
			IconURL: e.Member.User.AvatarURL("64"),
		},
		Title:       "Member Updated",
		Description: strings.Join(changes, "\n"),
	}
	m.SendAuditLog(s, e.GuildID, "members", emb, e.Member.User.ID, "")
}

func (m *Manager) LogUserUpdate(s *discordgo.Session, e *discordgo.UserUpdate) {
	if e.User == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color: 0x3498db,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    e.User.Username,
			IconURL: e.User.AvatarURL("64"),
		},
		Title:       "User Profile Updated",
		Description: fmt.Sprintf("<@%s> updated profile settings.", e.User.ID),
	}
	m.SendAuditLog(s, "", "members", emb, e.User.ID, "")
}

func (m *Manager) LogRoleCreate(s *discordgo.Session, e *discordgo.GuildRoleCreate) {
	if e.Role == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0x43b581,
		Title:       "Role Created",
		Description: fmt.Sprintf("<@&%s> `%s`", e.Role.ID, e.Role.Name),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Color", Value: fmt.Sprintf("`#%06x`", e.Role.Color), Inline: true},
			{Name: "ID", Value: fmt.Sprintf("`%s`", e.Role.ID), Inline: true},
		},
	}
	m.SendAuditLog(s, e.GuildID, "roles", emb, "", "")
}

func (m *Manager) LogRoleDelete(s *discordgo.Session, e *discordgo.GuildRoleDelete) {
	emb := &discordgo.MessageEmbed{
		Color:       0xff4747,
		Title:       "Role Deleted",
		Description: fmt.Sprintf("Role ID: `%s`", e.RoleID),
	}
	m.SendAuditLog(s, e.GuildID, "roles", emb, "", "")
}

func (m *Manager) LogRoleUpdate(s *discordgo.Session, e *discordgo.GuildRoleUpdate) {
	if e.Role == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0xf39c12,
		Title:       "Role Updated",
		Description: fmt.Sprintf("<@&%s> `%s` has been updated.", e.Role.ID, e.Role.Name),
	}
	m.SendAuditLog(s, e.GuildID, "roles", emb, "", "")
}

func (m *Manager) LogChannelCreate(s *discordgo.Session, e *discordgo.ChannelCreate) {
	if e.Channel == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0x43b581,
		Title:       "Channel Created",
		Description: fmt.Sprintf("<#%s> `%s`", e.Channel.ID, e.Channel.Name),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Type", Value: fmt.Sprintf("%d", e.Channel.Type), Inline: true},
			{Name: "ID", Value: fmt.Sprintf("`%s`", e.Channel.ID), Inline: true},
		},
	}
	m.SendAuditLog(s, e.GuildID, "channels", emb, "", e.Channel.ID)
}

func (m *Manager) LogChannelDelete(s *discordgo.Session, e *discordgo.ChannelDelete) {
	if e.Channel == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0xff4747,
		Title:       "Channel Deleted",
		Description: fmt.Sprintf("`#%s`", e.Channel.Name),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Type", Value: fmt.Sprintf("%d", e.Channel.Type), Inline: true},
			{Name: "ID", Value: fmt.Sprintf("`%s`", e.Channel.ID), Inline: true},
		},
	}
	m.SendAuditLog(s, e.GuildID, "channels", emb, "", e.Channel.ID)
}

func (m *Manager) LogChannelUpdate(s *discordgo.Session, e *discordgo.ChannelUpdate) {
	if e.Channel == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0x1abc9c,
		Title:       "Channel Updated",
		Description: fmt.Sprintf("<#%s> `%s` updated.", e.Channel.ID, e.Channel.Name),
	}
	m.SendAuditLog(s, e.GuildID, "channels", emb, "", e.Channel.ID)
}

func (m *Manager) LogVoiceStateUpdate(s *discordgo.Session, e *discordgo.VoiceStateUpdate) {
	if e.VoiceState == nil {
		return
	}
	if e.BeforeUpdate == nil {
		emb := &discordgo.MessageEmbed{
			Color:       0x43b581,
			Title:       "Joined Voice Channel",
			Description: fmt.Sprintf("<@%s> joined <#%s>", e.UserID, e.ChannelID),
		}
		m.SendAuditLog(s, e.GuildID, "voice", emb, e.UserID, e.ChannelID)
	} else if e.ChannelID == "" {
		emb := &discordgo.MessageEmbed{
			Color:       0xff4747,
			Title:       "Left Voice Channel",
			Description: fmt.Sprintf("<@%s> left <#%s>", e.UserID, e.BeforeUpdate.ChannelID),
		}
		m.SendAuditLog(s, e.GuildID, "voice", emb, e.UserID, e.BeforeUpdate.ChannelID)
	} else if e.BeforeUpdate.ChannelID != e.ChannelID {
		emb := &discordgo.MessageEmbed{
			Color:       0xf39c12,
			Title:       "Switched Voice Channel",
			Description: fmt.Sprintf("<@%s> moved: <#%s> → <#%s>", e.UserID, e.BeforeUpdate.ChannelID, e.ChannelID),
		}
		m.SendAuditLog(s, e.GuildID, "voice", emb, e.UserID, e.ChannelID)
	}
}

func (m *Manager) LogGuildUpdate(s *discordgo.Session, e *discordgo.GuildUpdate) {
	if e.Guild == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0xf39c12,
		Title:       "Server Settings Updated",
		Description: fmt.Sprintf("Server configuration changed for **%s**", e.Guild.Name),
	}
	m.SendAuditLog(s, e.Guild.ID, "server", emb, "", "")
}

func (m *Manager) LogInviteCreate(s *discordgo.Session, e *discordgo.InviteCreate) {
	emb := &discordgo.MessageEmbed{
		Color:       0x43b581,
		Title:       "Invite Created",
		Description: fmt.Sprintf("Code: `discord.gg/%s`", e.Code),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Channel", Value: fmt.Sprintf("<#%s>", e.ChannelID), Inline: true},
			{Name: "Created By", Value: fmt.Sprintf("<@%s>", e.Inviter.ID), Inline: true},
		},
	}
	m.SendAuditLog(s, e.GuildID, "invites", emb, e.Inviter.ID, e.ChannelID)
}

func (m *Manager) LogInviteDelete(s *discordgo.Session, e *discordgo.InviteDelete) {
	emb := &discordgo.MessageEmbed{
		Color:       0xff4747,
		Title:       "Invite Deleted",
		Description: fmt.Sprintf("Code: `discord.gg/%s`", e.Code),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Channel", Value: fmt.Sprintf("<#%s>", e.ChannelID), Inline: true},
		},
	}
	m.SendAuditLog(s, e.GuildID, "invites", emb, "", e.ChannelID)
}

func (m *Manager) LogCommandUsage(s *discordgo.Session, cmd *Command, ctx *CommandContext) {
	emb := &discordgo.MessageEmbed{
		Color:       0x2b2d31,
		Title:       "Command Executed",
		Description: fmt.Sprintf("<@%s> ran `.%s` in <#%s>", ctx.AuthorID(), cmd.Name, ctx.ChanID()),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Arguments", Value: strings.Join(ctx.Args, " "), Inline: false},
		},
	}
	m.SendAuditLog(s, ctx.GuildID(), "server", emb, ctx.AuthorID(), ctx.ChanID())
}

func (m *Manager) LogAutoModExecution(s *discordgo.Session, e *discordgo.AutoModerationActionExecution) {
	emb := &discordgo.MessageEmbed{
		Color:       0xe74c3c,
		Title:       "AutoMod Action Triggered",
		Description: fmt.Sprintf("<@%s> matched rule `%s` in <#%s>", e.UserID, e.RuleID, e.ChannelID),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Matched Keyword", Value: e.MatchedKeyword, Inline: true},
			{Name: "Matched Content", Value: e.MatchedContent, Inline: false},
		},
	}
	m.SendAuditLog(s, e.GuildID, "messages", emb, e.UserID, e.ChannelID)
}

func (m *Manager) LogScheduledEventCreate(s *discordgo.Session, e *discordgo.GuildScheduledEventCreate) {
	if e.GuildScheduledEvent == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0x9b59b6,
		Title:       "Scheduled Event Created",
		Description: fmt.Sprintf("Event **%s** created.", e.GuildScheduledEvent.Name),
	}
	m.SendAuditLog(s, e.GuildID, "server", emb, "", "")
}

func (m *Manager) LogScheduledEventDelete(s *discordgo.Session, e *discordgo.GuildScheduledEventDelete) {
	if e.GuildScheduledEvent == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0xff4747,
		Title:       "Scheduled Event Deleted",
		Description: fmt.Sprintf("Event **%s** deleted.", e.GuildScheduledEvent.Name),
	}
	m.SendAuditLog(s, e.GuildID, "server", emb, "", "")
}

func (m *Manager) LogScheduledEventUpdate(s *discordgo.Session, e *discordgo.GuildScheduledEventUpdate) {
	if e.GuildScheduledEvent == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0xf39c12,
		Title:       "Scheduled Event Updated",
		Description: fmt.Sprintf("Event **%s** updated.", e.GuildScheduledEvent.Name),
	}
	m.SendAuditLog(s, e.GuildID, "server", emb, "", "")
}

func (m *Manager) LogThreadCreate(s *discordgo.Session, e *discordgo.ThreadCreate) {
	if e.Channel == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0x43b581,
		Title:       "Thread Created",
		Description: fmt.Sprintf("Thread <#%s> created.", e.Channel.ID),
	}
	m.SendAuditLog(s, e.GuildID, "channels", emb, "", e.Channel.ID)
}

func (m *Manager) LogThreadDelete(s *discordgo.Session, e *discordgo.ThreadDelete) {
	if e.Channel == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0xff4747,
		Title:       "Thread Deleted",
		Description: fmt.Sprintf("Thread ID: `%s`", e.Channel.ID),
	}
	m.SendAuditLog(s, e.GuildID, "channels", emb, "", e.Channel.ID)
}

func (m *Manager) LogThreadUpdate(s *discordgo.Session, e *discordgo.ThreadUpdate) {
	if e.Channel == nil {
		return
	}
	emb := &discordgo.MessageEmbed{
		Color:       0x1abc9c,
		Title:       "Thread Updated",
		Description: fmt.Sprintf("Thread <#%s> updated.", e.Channel.ID),
	}
	m.SendAuditLog(s, e.GuildID, "channels", emb, "", e.Channel.ID)
}

func (m *Manager) LogWebhooksUpdate(s *discordgo.Session, e *discordgo.WebhooksUpdate) {
	emb := &discordgo.MessageEmbed{
		Color:       0xe67e22,
		Title:       "Webhooks Updated",
		Description: fmt.Sprintf("Webhooks in channel <#%s> were updated.", e.ChannelID),
	}
	m.SendAuditLog(s, e.GuildID, "channels", emb, "", e.ChannelID)
}

func (m *Manager) LogGuildEmojisUpdate(s *discordgo.Session, e *discordgo.GuildEmojisUpdate) {
	emb := &discordgo.MessageEmbed{
		Color:       0xf39c12,
		Title:       "Emojis Updated",
		Description: fmt.Sprintf("Server emojis updated (%d emojis total).", len(e.Emojis)),
	}
	m.SendAuditLog(s, e.GuildID, "emojis", emb, "", "")
}

