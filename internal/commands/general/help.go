package general

import (
	"encoding/json"
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var Help = &manager.Command{
	Trigger:     "help",
	Name:        "help",
	Description: "List all available commands grouped by category",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		cMap := groupCmds(ctx.Mgr.Commands())
		def := "general"
		if _, ok := cMap[def]; !ok {
			for k := range cMap {
				def = k
				break
			}
		}

		e := buildHelp(ctx.Cfg, cMap, def, 0, ctx.Cfg.Prefix)
		comps := buildComps(cMap, def, 0)

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
	},
}

func groupCmds(cmds []*manager.Command) map[string][]*manager.Command {
	cMap := make(map[string][]*manager.Command)
	for _, cmd := range cmds {
		c := strings.ToLower(cmd.Category)
		if c == "" {
			c = "other"
		}
		cMap[c] = append(cMap[c], cmd)
	}
	return cMap
}

const pageSize = 5

func buildHelp(cfg config.ResCfg, cMap map[string][]*manager.Command, cat string, page int, prefix string) *discordgo.MessageEmbed {
	cmds := cMap[cat]
	totPages := (len(cmds) + pageSize - 1) / pageSize
	if totPages < 1 {
		totPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totPages {
		page = totPages - 1
	}

	e := config.Build(cfg, config.EmbedOpt{
		Title:       "Help Panel - " + strings.Title(cat),
		Description: fmt.Sprintf("Use the select menu below to cycle categories.\nActive Prefix: `%s`\nPage %d of %d", prefix, page+1, totPages),
	})

	start := page * pageSize
	end := start + pageSize
	if end > len(cmds) {
		end = len(cmds)
	}

	for _, cmd := range cmds[start:end] {
		aliases := ""
		if len(cmd.Aliases) > 0 {
			aliases = fmt.Sprintf(" `[%s]`", strings.Join(cmd.Aliases, ", "))
		}
		val := fmt.Sprintf("`%s%s` %s\n*%s*", prefix, cmd.Trigger, aliases, cmd.Description)
		e.Fields = append(e.Fields, config.Field(
			strings.Title(cmd.Name),
			val,
			true,
		))
	}
	return e
}

func buildComps(cMap map[string][]*manager.Command, activeCat string, page int) []discordgo.MessageComponent {
	var opts []discordgo.SelectMenuOption
	for c := range cMap {
		opts = append(opts, discordgo.SelectMenuOption{
			Label:       strings.Title(c),
			Value:       c,
			Description: fmt.Sprintf("Show %s commands", c),
			Default:     c == activeCat,
		})
	}

	cmds := cMap[activeCat]
	totPages := (len(cmds) + pageSize - 1) / pageSize
	if totPages < 1 {
		totPages = 1
	}

	prevDisabled := page <= 0
	nextDisabled := page >= totPages-1

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "help_select",
					Placeholder: "Select category...",
					Options:     opts,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Previous",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("help_prev:%s:%d", activeCat, page),
					Disabled: prevDisabled,
				},
				discordgo.Button{
					Label:    "Next",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("help_next:%s:%d", activeCat, page),
					Disabled: nextDisabled,
				},
				discordgo.Button{
					Label:    "Page...",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("help_btn_goto:%s", activeCat),
				},
				discordgo.Button{
					Label:    "Clear Panel",
					Style:    discordgo.DangerButton,
					CustomID: "help_btn_clear",
				},
			},
		},
	}
}

func HandleHelpComponent(s *discordgo.Session, i *discordgo.InteractionCreate, mgr *manager.Manager) {
	inst, ok := mgr.ResolvedCfgFor(s.State.User.ID)
	if !ok {
		inst = config.Resolve(config.GetGlobal(), config.BotInst{})
	}

	cMap := groupCmds(mgr.Commands())

	if i.Type == discordgo.InteractionModalSubmit {
		modalData := i.ModalSubmitData()
		if strings.HasPrefix(modalData.CustomID, "help_modal_goto:") {
			parts := strings.Split(modalData.CustomID, ":")
			cat := "general"
			if len(parts) == 2 {
				cat = parts[1]
			}

			pageNumStr := ""
			if b, err := json.Marshal(modalData.Components); err == nil {
				type rawComp struct {
					Type     int        `json:"type"`
					CustomID string     `json:"custom_id"`
					Value    string     `json:"value"`
					List     []rawComp  `json:"components"`
				}
				var rows []rawComp
				if err := json.Unmarshal(b, &rows); err == nil {
					for _, r := range rows {
						for _, c := range r.List {
							if c.CustomID == "page_num" {
								pageNumStr = c.Value
								break
							}
						}
					}
				}
			}

			var page int
			_, _ = fmt.Sscanf(pageNumStr, "%d", &page)
			page-- // user page 1 is page index 0

			e := buildHelp(inst, cMap, cat, page, inst.Prefix)
			comps := buildComps(cMap, cat, page)

			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Embeds:     []*discordgo.MessageEmbed{e},
					Components: comps,
				},
			})
		}
		return
	}

	data := i.MessageComponentData()

	if data.CustomID == "help_btn_clear" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Help panel closed.",
				Embeds:     []*discordgo.MessageEmbed{},
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	if strings.HasPrefix(data.CustomID, "help_btn_goto:") {
		parts := strings.Split(data.CustomID, ":")
		cat := "general"
		if len(parts) == 2 {
			cat = parts[1]
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: fmt.Sprintf("help_modal_goto:%s", cat),
				Title:    "Go to Page",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "page_num",
								Label:       "Enter Page Number",
								Style:       discordgo.TextInputShort,
								Placeholder: "e.g. 1",
								Required:    true,
								MinLength:   1,
								MaxLength:   3,
							},
						},
					},
				},
			},
		})
		return
	}

	if data.CustomID == "help_select" && len(data.Values) > 0 {
		cat := data.Values[0]
		e := buildHelp(inst, cMap, cat, 0, inst.Prefix)
		comps := buildComps(cMap, cat, 0)

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{e},
				Components: comps,
			},
		})
		return
	}

	if strings.HasPrefix(data.CustomID, "help_prev:") || strings.HasPrefix(data.CustomID, "help_next:") {
		parts := strings.Split(data.CustomID, ":")
		if len(parts) == 3 {
			cat := parts[1]
			var page int
			_, _ = fmt.Sscanf(parts[2], "%d", &page)
			if strings.HasPrefix(data.CustomID, "help_prev:") {
				page--
			} else {
				page++
			}
			e := buildHelp(inst, cMap, cat, page, inst.Prefix)
			comps := buildComps(cMap, cat, page)

			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Embeds:     []*discordgo.MessageEmbed{e},
					Components: comps,
				},
			})
		}
	}
}
