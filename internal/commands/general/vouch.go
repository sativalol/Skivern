package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("vouch", []manager.HelpPage{
		{
			Command:     "Vouch",
			Syntax:      ".vouch <user> [comment]",
			Description: "Vouch for a user to build their reputation.",
		},
		{
			Command:     "Vouch Remove",
			Syntax:      ".vouch remove <user>",
			Description: "Remove your vouch for a user.",
		},
		{
			Command:     "Vouch List",
			Syntax:      ".vouch list [@user]",
			Description: "List vouches for a user (scrollable).",
		},
	})
}

var rxVouchUser = regexp.MustCompile(`^<@!?(\d+)>$`)

func resolveVouchUser(s *discordgo.Session, gid, query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}
	if m := rxVouchUser.FindStringSubmatch(q); len(m) > 1 {
		return m[1]
	}
	members, err := s.GuildMembers(gid, "", 1000)
	if err != nil {
		return ""
	}
	for _, m := range members {
		if m.User.ID == q {
			return m.User.ID
		}
	}
	ql := strings.ToLower(q)
	for _, m := range members {
		if strings.ToLower(m.User.Username) == ql {
			return m.User.ID
		}
	}
	return ""
}

var Vouch = &manager.Command{
	Trigger:     "vouch",
	Name:        "vouch",
	Description: "Vouch for a user or manage/list vouches",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("vouch")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()
		sender := ctx.AuthorID()

		if sub == "remove" {
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.vouch remove <@user>`")
			}
			target := resolveVouchUser(ctx.Session, gid, ctx.Args[1])
			if target == "" {
				return ctx.Reply("[!] Could not resolve target user.")
			}
			_ = ctx.DB.DeleteVouch(target, sender)
			return ctx.Reply(fmt.Sprintf("[+] Removed your vouch for <@%s>.", target))
		}

		if sub == "list" {
			target := sender
			if len(ctx.Args) > 1 {
				t := resolveVouchUser(ctx.Session, gid, ctx.Args[1])
				if t != "" {
					target = t
				}
			}

			list, err := ctx.DB.ListVouches(target)
			if err != nil || len(list) == 0 {
				return ctx.Reply("[*] This user has no vouches.")
			}

			pages := (len(list) + 4) / 5
			end := 5
			if end > len(list) {
				end = len(list)
			}

			emb := renderVouchPage(ctx.Cfg, target, list[0:end], 1, pages, len(list))
			comp := getVouchButtons(target, 1, pages)

			if ctx.Interact != nil {
				return ctx.Session.InteractionRespond(ctx.Interact, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Embeds:     []*discordgo.MessageEmbed{emb},
						Components: comp,
					},
				})
			}

			_, err = ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
				Embeds:     []*discordgo.MessageEmbed{emb},
				Components: comp,
			})
			return err
		}

		target := resolveVouchUser(ctx.Session, gid, ctx.Args[0])
		if target == "" {
			return ctx.Reply("[!] Could not resolve target user.")
		}
		if target == sender {
			return ctx.Reply("[!] You cannot vouch for yourself.")
		}

		comment := "No comment provided."
		if len(ctx.Args) > 1 {
			comment = strings.Join(ctx.Args[1:], " ")
		}

		v := storage.Vouch{
			TargetUserID: target,
			VoucherID:    sender,
			Comment:      comment,
			Time:         time.Now().Unix(),
		}

		_ = ctx.DB.SaveVouch(v)
		return ctx.Reply(fmt.Sprintf("[+] Vouched for <@%s>: \"%s\"", target, comment))
	},
}

func renderVouchPage(cfg config.ResCfg, target string, list []storage.Vouch, page, pages, total int) *discordgo.MessageEmbed {
	var sb strings.Builder
	for i, v := range list {
		t := time.Unix(v.Time, 0).Format("2006-01-02")
		sb.WriteString(fmt.Sprintf("%d. <@%s> (%s): %s\n", (page-1)*5+i+1, v.VoucherID, t, v.Comment))
	}
	return config.Build(cfg, config.EmbedOpt{
		Title:       fmt.Sprintf("Vouches for User"),
		Description: fmt.Sprintf("User: <@%s>\nTotal: **%d** (Page %d/%d)\n\n%s", target, total, page, pages, sb.String()),
	})
}

func getVouchButtons(target string, page, pages int) []discordgo.MessageComponent {
	if pages <= 1 {
		return nil
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "◀",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("vouch:%s:%d", target, page-1),
					Disabled: page <= 1,
				},
				discordgo.Button{
					Label:    "▶",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("vouch:%s:%d", target, page+1),
					Disabled: page >= pages,
				},
			},
		},
	}
}

func HandleVouchComponent(s *discordgo.Session, i *discordgo.InteractionCreate, mgr *manager.Manager) {
	parts := strings.Split(i.MessageComponentData().CustomID, ":")
	if len(parts) < 3 {
		return
	}
	target := parts[1]
	page, _ := strconv.Atoi(parts[2])

	list, err := mgr.DB().ListVouches(target)
	if err != nil || len(list) == 0 {
		return
	}

	pages := (len(list) + 4) / 5
	if page < 1 {
		page = 1
	}
	if page > pages {
		page = pages
	}

	start := (page - 1) * 5
	end := start + 5
	if end > len(list) {
		end = len(list)
	}

	resCfg, _ := mgr.ResolvedCfgFor(s.State.User.ID)
	emb := renderVouchPage(resCfg, target, list[start:end], page, pages, len(list))
	comp := getVouchButtons(target, page, pages)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{emb},
			Components: comp,
		},
	})
}
