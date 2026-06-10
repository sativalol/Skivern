package utility

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var Snipe = &manager.Command{
	Trigger:     "snipe",
	Aliases:     []string{"s"},
	Name:        "snipe",
	Description: "Snipe the latest deleted message in the channel",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		cid := ctx.ChanID()
		manager.DeletedMu.Lock()
		lst := manager.Deleted[cid]
		manager.DeletedMu.Unlock()

		if len(lst) == 0 {
			return ctx.Reply("[!] Nothing to snipe.")
		}

		idx := 0
		if len(ctx.Args) > 0 {
			if i, err := strconv.Atoi(ctx.Args[0]); err == nil {
				idx = i - 1
			}
		}

		if idx < 0 || idx >= len(lst) {
			return ctx.Reply(fmt.Sprintf("[!] Invalid index. Choose 1 to %d.", len(lst)))
		}

		m := lst[idx]
		e := buildSnipeEmbed(ctx.Cfg, m, idx, len(lst))
		comps := buildSnipeComps("snipe", idx, len(lst))

		if ctx.Interact != nil {
			return ctx.Session.InteractionRespond(ctx.Interact, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds:     []*discordgo.MessageEmbed{e},
					Components: comps,
				},
			})
		}
		_, err := ctx.Session.ChannelMessageSendComplex(cid, &discordgo.MessageSend{
			Embeds:     []*discordgo.MessageEmbed{e},
			Components: comps,
		})
		return err
	},
}

var EditSnipe = &manager.Command{
	Trigger:     "editsnipe",
	Aliases:     []string{"es"},
	Name:        "editsnipe",
	Description: "Snipe the latest edited message in the channel",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		cid := ctx.ChanID()
		manager.EditedMu.Lock()
		lst := manager.Edited[cid]
		manager.EditedMu.Unlock()

		if len(lst) == 0 {
			return ctx.Reply("[!] Nothing to editsnipe.")
		}

		idx := 0
		if len(ctx.Args) > 0 {
			if i, err := strconv.Atoi(ctx.Args[0]); err == nil {
				idx = i - 1
			}
		}

		if idx < 0 || idx >= len(lst) {
			return ctx.Reply(fmt.Sprintf("[!] Invalid index. Choose 1 to %d.", len(lst)))
		}

		m := lst[idx]
		e := buildEditSnipeEmbed(ctx.Cfg, m, idx, len(lst))
		comps := buildSnipeComps("esnipe", idx, len(lst))

		if ctx.Interact != nil {
			return ctx.Session.InteractionRespond(ctx.Interact, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds:     []*discordgo.MessageEmbed{e},
					Components: comps,
				},
			})
		}
		_, err := ctx.Session.ChannelMessageSendComplex(cid, &discordgo.MessageSend{
			Embeds:     []*discordgo.MessageEmbed{e},
			Components: comps,
		})
		return err
	},
}

var ReactionSnipe = &manager.Command{
	Trigger:     "reactionsnipe",
	Aliases:     []string{"rs"},
	Name:        "reactionsnipe",
	Description: "Snipe the latest removed reaction in the channel",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		cid := ctx.ChanID()
		manager.ReactMu.Lock()
		lst := manager.React[cid]
		manager.ReactMu.Unlock()

		if len(lst) == 0 {
			return ctx.Reply("[!] Nothing to reactionsnipe.")
		}

		idx := 0
		if len(ctx.Args) > 0 {
			if i, err := strconv.Atoi(ctx.Args[0]); err == nil {
				idx = i - 1
			}
		}

		if idx < 0 || idx >= len(lst) {
			return ctx.Reply(fmt.Sprintf("[!] Invalid index. Choose 1 to %d.", len(lst)))
		}

		m := lst[idx]
		e := buildReactSnipeEmbed(ctx.Cfg, m, idx, len(lst))
		comps := buildSnipeComps("rsnipe", idx, len(lst))

		if ctx.Interact != nil {
			return ctx.Session.InteractionRespond(ctx.Interact, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds:     []*discordgo.MessageEmbed{e},
					Components: comps,
				},
			})
		}
		_, err := ctx.Session.ChannelMessageSendComplex(cid, &discordgo.MessageSend{
			Embeds:     []*discordgo.MessageEmbed{e},
			Components: comps,
		})
		return err
	},
}

var ClearSnipe = &manager.Command{
	Trigger:     "clearsnipe",
	Aliases:     []string{"cs"},
	Name:        "clearsnipe",
	Description: "Clear the snipe cache for this channel",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if ctx.Message != nil {
			p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
			if err != nil || (p&discordgo.PermissionManageMessages) == 0 {
				return ctx.Reply("[!] You need Manage Messages permission to clear snipes.")
			}
		}
		manager.ClearSnipe(ctx.ChanID())
		return ctx.Reply("[+] Cleared all snipes for this channel.")
	},
}

func buildSnipeEmbed(cfg config.ResCfg, m manager.DeletedMsg, idx, total int) *discordgo.MessageEmbed {
	content := m.Content
	if content == "" {
		content = "*Message had no text content (possibly embed or attachment only).*"
	}
	e := config.Build(cfg, config.EmbedOpt{
		Description: content,
	})
	e.Author = &discordgo.MessageEmbedAuthor{
		Name:    m.Author.Username,
		IconURL: m.Author.AvatarURL(""),
	}
	e.Timestamp = m.Time.Format(time.RFC3339)
	e.Footer = &discordgo.MessageEmbedFooter{
		Text:    fmt.Sprintf("Snipe %d/%d | %s", idx+1, total, cfg.Footer),
		IconURL: cfg.FooterIcon,
	}
	return e
}

func buildEditSnipeEmbed(cfg config.ResCfg, m manager.EditedMsg, idx, total int) *discordgo.MessageEmbed {
	oldContent := m.Old
	if oldContent == "" {
		oldContent = "*(empty)*"
	}
	newContent := m.New
	if newContent == "" {
		newContent = "*(empty)*"
	}
	e := config.Build(cfg, config.EmbedOpt{
		Description: fmt.Sprintf("**Before:**\n%s\n\n**After:**\n%s", oldContent, newContent),
	})
	e.Author = &discordgo.MessageEmbedAuthor{
		Name:    m.Author.Username,
		IconURL: m.Author.AvatarURL(""),
	}
	e.Timestamp = m.Time.Format(time.RFC3339)
	e.Footer = &discordgo.MessageEmbedFooter{
		Text:    fmt.Sprintf("Edit Snipe %d/%d | %s", idx+1, total, cfg.Footer),
		IconURL: cfg.FooterIcon,
	}
	return e
}

func buildReactSnipeEmbed(cfg config.ResCfg, m manager.DeletedReact, idx, total int) *discordgo.MessageEmbed {
	emojiStr := m.Emoji.Name
	if m.Emoji.ID != "" {
		emojiStr = fmt.Sprintf("<:%s:%s>", m.Emoji.Name, m.Emoji.ID)
	}
	e := config.Build(cfg, config.EmbedOpt{
		Description: fmt.Sprintf("Removed reaction: %s", emojiStr),
	})
	e.Author = &discordgo.MessageEmbedAuthor{
		Name:    m.Author.Username,
		IconURL: m.Author.AvatarURL(""),
	}
	e.Timestamp = m.Time.Format(time.RFC3339)
	e.Footer = &discordgo.MessageEmbedFooter{
		Text:    fmt.Sprintf("Reaction Snipe %d/%d | %s", idx+1, total, cfg.Footer),
		IconURL: cfg.FooterIcon,
	}
	return e
}

func buildSnipeComps(action string, idx, total int) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "◀",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("%s_prev:%d", action, idx),
					Disabled: idx <= 0,
				},
				discordgo.Button{
					Label:    "▶",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("%s_next:%d", action, idx),
					Disabled: idx >= total-1,
				},
			},
		},
	}
}

func HandleSnipeComponent(s *discordgo.Session, i *discordgo.InteractionCreate, mgr *manager.Manager) {
	data := i.MessageComponentData()
	parts := strings.Split(data.CustomID, ":")
	if len(parts) != 2 {
		return
	}

	actionWithDirection := parts[0]
	var idx int
	_, _ = fmt.Sscanf(parts[1], "%d", &idx)

	var prefix string
	if strings.HasPrefix(actionWithDirection, "snipe_") {
		prefix = "snipe"
	} else if strings.HasPrefix(actionWithDirection, "esnipe_") {
		prefix = "esnipe"
	} else if strings.HasPrefix(actionWithDirection, "rsnipe_") {
		prefix = "rsnipe"
	} else {
		return
	}

	if strings.HasSuffix(actionWithDirection, "_prev") {
		idx--
	} else {
		idx++
	}

	cid := i.ChannelID
	inst, ok := mgr.ResolvedCfgFor(s.State.User.ID)
	if !ok {
		inst = config.Resolve(config.GetGlobal(), config.BotInst{})
	}

	var e *discordgo.MessageEmbed
	var comps []discordgo.MessageComponent

	switch prefix {
	case "snipe":
		manager.DeletedMu.Lock()
		lst := manager.Deleted[cid]
		manager.DeletedMu.Unlock()
		if idx >= 0 && idx < len(lst) {
			e = buildSnipeEmbed(inst, lst[idx], idx, len(lst))
			comps = buildSnipeComps("snipe", idx, len(lst))
		}
	case "esnipe":
		manager.EditedMu.Lock()
		lst := manager.Edited[cid]
		manager.EditedMu.Unlock()
		if idx >= 0 && idx < len(lst) {
			e = buildEditSnipeEmbed(inst, lst[idx], idx, len(lst))
			comps = buildSnipeComps("esnipe", idx, len(lst))
		}
	case "rsnipe":
		manager.ReactMu.Lock()
		lst := manager.React[cid]
		manager.ReactMu.Unlock()
		if idx >= 0 && idx < len(lst) {
			e = buildReactSnipeEmbed(inst, lst[idx], idx, len(lst))
			comps = buildSnipeComps("rsnipe", idx, len(lst))
		}
	}

	if e != nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{e},
				Components: comps,
			},
		})
	}
}
