package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var Purge = &manager.Command{
	Trigger:     "clear",
	Aliases:     []string{"purge", "c", "prg"},
	Name:        "clear",
	Description: "Bulk delete messages with optional filters",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageMessages) {
			return ctx.Reply("[!] You need Manage Messages permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: clear <amount> [filter: links|images|bots|humans|user] [filter_value]")
		}

		lim, err := strconv.Atoi(ctx.Args[0])
		if err != nil || lim <= 0 || lim > 100 {
			return ctx.Respond(config.Wrap(ctx.Cfg, "[!] Specify a valid amount of messages (1-100)."))
		}

		cid := ctx.ChanID()
		max := lim
		if len(ctx.Args) > 1 {
			max = 100
		}
		mList, err := ctx.Session.ChannelMessages(cid, max, "", "", "")
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to fetch messages: %v", err))
		}

		var fType string
		var fVal string
		if len(ctx.Args) > 1 {
			fType = strings.ToLower(ctx.Args[1])
			if len(ctx.Args) > 2 {
				fVal = ctx.Args[2]
			}
		}

		var user *discordgo.Member
		if fType == "user" && fVal != "" {
			user, _ = moderation.ResolveMember(ctx.Session, ctx.GuildID(), fVal)
			if user == nil {
				return ctx.Reply("[!] Could not resolve user filter value.")
			}
		}

		var delIDs []string
		now := time.Now()

		for _, m := range mList {
			if len(delIDs) >= lim {
				break
			}
			if ctx.Message != nil && m.ID == ctx.Message.ID {
				continue
			}

			keep := true
			switch fType {
			case "links":
				keep = strings.Contains(m.Content, "http://") || strings.Contains(m.Content, "https://")
			case "images":
				keep = len(m.Attachments) > 0
			case "bots":
				keep = m.Author != nil && m.Author.Bot
			case "humans":
				keep = m.Author != nil && !m.Author.Bot
			case "user":
				keep = m.Author != nil && user != nil && m.Author.ID == user.User.ID
			}

			if keep {
				delIDs = append(delIDs, m.ID)
			}
		}

		if len(delIDs) == 0 {
			return ctx.Reply("[+] No matching messages found to clear.")
		}

		var bulk []string
		var old []string

		for _, id := range delIDs {
			t, err := sfTime(id)
			if err == nil && now.Sub(t) < 14*24*time.Hour {
				bulk = append(bulk, id)
			} else {
				old = append(old, id)
			}
		}

		cnt := 0
		if len(bulk) > 0 {
			if err := ctx.BulkDelete(bulk); err == nil {
				cnt += len(bulk)
			} else {
				old = append(old, bulk...)
			}
		}
		if len(old) > 0 {
			for _, id := range old {
				if err := ctx.Delete(id); err == nil {
					cnt++
					time.Sleep(150 * time.Millisecond)
				}
			}
		}

		moderation.LogAction(ctx.Session, ctx.DB, ctx.GuildID(), "Bulk Clear", ctx.AuthorID(), cid,
			fmt.Sprintf("Cleared %d messages in <#%s>.", cnt, cid),
			config.Field("Filter", fType, true),
		)

		res, err := ctx.Session.ChannelMessageSendEmbed(cid, config.Wrap(ctx.Cfg, fmt.Sprintf("[+] Cleared %d messages.", cnt)))
		if err == nil {
			go func() {
				time.Sleep(3 * time.Second)
				_ = ctx.Delete(res.ID)
			}()
		}
		return nil
	},
}

func sfTime(id string) (time.Time, error) {
	sf, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	t := (sf >> 22) + 1420070400000
	return time.Unix(0, t*int64(time.Millisecond)), nil
}
