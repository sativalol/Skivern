package manager

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"skyvern/internal/config"
	"skyvern/internal/storage"
)

type CommandContext struct {
	Session  *discordgo.Session
	Message  *discordgo.Message
	Interact *discordgo.Interaction
	Args     []string
	Cfg      config.ResCfg
	DB       *storage.DB
	ClientID string
	Mgr      *Manager
}

type Command struct {
	Trigger     string
	Aliases     []string
	Name        string
	Description string
	Category    string
	Execute     func(ctx *CommandContext) error
}

func (ctx *CommandContext) Respond(embed *discordgo.MessageEmbed) error {
	if ctx.Interact != nil {
		return ctx.Session.InteractionRespond(ctx.Interact, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}},
		})
	}
	_, err := ctx.Session.ChannelMessageSendEmbed(ctx.Message.ChannelID, embed)
	return err
}

func (ctx *CommandContext) GuildID() string {
	if ctx.Interact != nil {
		return ctx.Interact.GuildID
	}
	if ctx.Message != nil {
		return ctx.Message.GuildID
	}
	return ""
}

func (ctx *CommandContext) ChanID() string {
	if ctx.Interact != nil {
		return ctx.Interact.ChannelID
	}
	if ctx.Message != nil {
		return ctx.Message.ChannelID
	}
	return ""
}

func (ctx *CommandContext) AuthorID() string {
	if ctx.Interact != nil && ctx.Interact.Member != nil && ctx.Interact.Member.User != nil {
		return ctx.Interact.Member.User.ID
	}
	if ctx.Message != nil && ctx.Message.Author != nil {
		return ctx.Message.Author.ID
	}
	return ""
}

func (ctx *CommandContext) AuthorTag() string {
	if ctx.Interact != nil && ctx.Interact.Member != nil && ctx.Interact.Member.User != nil {
		return ctx.Interact.Member.User.Username
	}
	if ctx.Message != nil && ctx.Message.Author != nil {
		return ctx.Message.Author.Username
	}
	return "Unknown"
}

func (ctx *CommandContext) Reply(text string) error {
	return ctx.Respond(config.Wrap(ctx.Cfg, text))
}

func (ctx *CommandContext) Ban(uid, reason string, days int) error {
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), reason)
	return ctx.Session.GuildBanCreateWithReason(ctx.GuildID(), uid, auditReason, days)
}

func (ctx *CommandContext) Unban(uid string, reason ...string) error {
	r := "Manual unban"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.GuildBanDelete(ctx.GuildID(), uid, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) Kick(uid string, reason ...string) error {
	r := "Manual kick"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.GuildMemberDelete(ctx.GuildID(), uid, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) Timeout(uid string, until *time.Time, reason ...string) error {
	r := "Manual timeout"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.GuildMemberTimeout(ctx.GuildID(), uid, until, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) Nick(uid, nick string, reason ...string) error {
	r := "Manual nickname update"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.GuildMemberNickname(ctx.GuildID(), uid, nick, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) ChannelPermissionSet(chID, targetID string, targetType discordgo.PermissionOverwriteType, allowVal, denyVal int64, reason ...string) error {
	r := "Update channel permissions"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.ChannelPermissionSet(chID, targetID, targetType, allowVal, denyVal, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) ChannelPermissionDelete(chID, targetID string, reason ...string) error {
	r := "Delete channel permissions override"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.ChannelPermissionDelete(chID, targetID, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) Delete(msgID string) error {
	return ctx.Session.ChannelMessageDelete(ctx.ChanID(), msgID)
}

func (ctx *CommandContext) BulkDelete(ids []string) error {
	return ctx.Session.ChannelMessagesBulkDelete(ctx.ChanID(), ids)
}
