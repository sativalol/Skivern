package manager

import (
	"fmt"
	"strings"
	"sync"

	"skyvern/internal/config"

	"github.com/bwmarrin/discordgo"
)

type HelpPage struct {
	Command     string
	Syntax      string
	Description string
}

var (
	HelpMu    sync.RWMutex
	HelpCache = make(map[string][]HelpPage)
)

func RegisterHelp(cmd string, pages []HelpPage) {
	HelpMu.Lock()
	defer HelpMu.Unlock()
	HelpCache[strings.ToLower(cmd)] = pages
}

func GetHelp(cmd string) ([]HelpPage, bool) {
	HelpMu.RLock()
	defer HelpMu.RUnlock()
	p, ok := HelpCache[strings.ToLower(cmd)]
	return p, ok
}

func (ctx *CommandContext) SendHelp(cmdName string) error {
	pages, ok := GetHelp(cmdName)
	if !ok || len(pages) == 0 {
		return ctx.Reply(fmt.Sprintf("%s Incorrect syntax. Try checking help for `%s`.", ctx.Cfg.WarnSym, cmdName))
	}

	e := BuildHelpEmbed(ctx.Cfg, cmdName, pages, 0)
	comps := BuildHelpComps(cmdName, 0, len(pages))

	if ctx.Interact != nil {
		return ctx.Session.InteractionRespond(ctx.Interact, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{e},
				Components: comps,
			},
		})
	}

	_, err := ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{e},
		Components: comps,
	})
	return err
}

func BuildHelpEmbed(cfg config.ResCfg, cmdName string, pages []HelpPage, idx int) *discordgo.MessageEmbed {
	page := pages[idx]
	desc := fmt.Sprintf("**Command:** %s\n**Syntax:** `%s`\n**Description:** %s",
		page.Command, page.Syntax, page.Description)

	e := config.Build(cfg, config.EmbedOpt{
		Title:       fmt.Sprintf("%s Help", strings.Title(cmdName)),
		Description: desc,
	})
	e.Footer = &discordgo.MessageEmbedFooter{
		Text:    fmt.Sprintf("Page %d of %d | %s", idx+1, len(pages), cfg.Footer),
		IconURL: cfg.FooterIcon,
	}
	return e
}

func BuildHelpComps(cmdName string, idx, total int) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "◀",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("cmdhelp_prev:%s:%d", strings.ToLower(cmdName), idx),
					Disabled: idx <= 0,
				},
				discordgo.Button{
					Label:    "▶",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("cmdhelp_next:%s:%d", strings.ToLower(cmdName), idx),
					Disabled: idx >= total-1,
				},
			},
		},
	}
}

func HandleGlobalHelpComponent(s *discordgo.Session, i *discordgo.InteractionCreate, mgr *Manager) {
	data := i.MessageComponentData()
	parts := strings.Split(data.CustomID, ":")
	if len(parts) != 3 {
		return
	}

	action := parts[0]
	cmdName := parts[1]
	var idx int
	_, _ = fmt.Sscanf(parts[2], "%d", &idx)

	if action == "cmdhelp_prev" {
		idx--
	} else if action == "cmdhelp_next" {
		idx++
	} else {
		return
	}

	pages, ok := GetHelp(cmdName)
	if !ok || idx < 0 || idx >= len(pages) {
		return
	}

	inst, ok := mgr.ResolvedCfgFor(s.State.User.ID)
	if !ok {
		inst = config.Resolve(config.GetGlobal(), config.BotInst{})
	}

	e := BuildHelpEmbed(inst, cmdName, pages, idx)
	comps := BuildHelpComps(cmdName, idx, len(pages))

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{e},
			Components: comps,
		},
	})
}
