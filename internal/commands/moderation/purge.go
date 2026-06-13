package moderation

import (
	"fmt"
	"regexp"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var rxPurgeMsgLink = regexp.MustCompile(`discord(?:app)?\.com/channels/\d+/\d+/(\d+)`)

func init() {
	manager.RegisterHelp("purge", []manager.HelpPage{
		{
			Command:     "Purge Amount",
			Syntax:      ".purge <amount> [member/search]",
			Description: "Purge a specific amount of messages, optionally from a member or matching a search query.",
		},
		{
			Command:     "Purge Before/After",
			Syntax:      ".purge [before|after] <messagelink>",
			Description: "Purge messages before or after a given message link.",
		},
		{
			Command:     "Purge Between",
			Syntax:      ".purge between <start_id/link> <finish_id/link>",
			Description: "Purge all messages between two message IDs or links.",
		},
		{
			Command:     "Purge Upto",
			Syntax:      ".purge upto <messagelink>",
			Description: "Purge messages up to a specific message link.",
		},
		{
			Command:     "Purge Filters",
			Syntax:      ".purge <filter> [search]",
			Description: "Filters: startswith, endswith, contains, embeds, files, images, bots, humans, webhooks, links, activity, reactions, stickers, mentions, emoji, emotes.",
		},
	})
}

var Purge = &manager.Command{
	Trigger:     "clear",
	Aliases:     []string{"purge", "c", "prg"},
	Name:        "clear",
	Description: "Bulk delete messages with advanced filters",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageMessages) {
			return ctx.Reply("[!] You need Manage Messages permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("purge")
		}

		gid := ctx.GuildID()
		cid := ctx.ChanID()

		// 1. Detect sub-commands that don't start with amount (e.g. .purge before <link>)
		firstArg := strings.ToLower(ctx.Args[0])
		isRangeCmd := firstArg == "before" || firstArg == "after" || firstArg == "upto" || firstArg == "between"
		isFilterCmd := isFilterKeyword(firstArg)

		amount := 100
		startIdx := 0

		if !isRangeCmd && !isFilterCmd {
			if val, err := strconv.Atoi(ctx.Args[0]); err == nil && val > 0 {
				amount = val
				startIdx = 1
			} else {
				// No valid amount specified, default to 100 messages check
				amount = 100
				startIdx = 0
			}
		}

		sub := ""
		if len(ctx.Args) > startIdx {
			sub = strings.ToLower(ctx.Args[startIdx])
		}

		var mList []*discordgo.Message
		var err error

		// Handle Range commands: before, after, upto, between
		if sub == "before" {
			if len(ctx.Args) < startIdx+2 {
				return ctx.Reply("Usage: `.purge before <messagelink>`")
			}
			msgID := resolveMsgID(ctx.Args[startIdx+1])
			mList, err = ctx.Session.ChannelMessages(cid, amount, msgID, "", "")
		} else if sub == "after" {
			if len(ctx.Args) < startIdx+2 {
				return ctx.Reply("Usage: `.purge after <messagelink>`")
			}
			msgID := resolveMsgID(ctx.Args[startIdx+1])
			mList, err = ctx.Session.ChannelMessages(cid, amount, "", msgID, "")
		} else if sub == "upto" {
			if len(ctx.Args) < startIdx+2 {
				return ctx.Reply("Usage: `.purge upto <messagelink>`")
			}
			msgID := resolveMsgID(ctx.Args[startIdx+1])
			mList, err = ctx.Session.ChannelMessages(cid, amount, "", "", "")
			if err == nil {
				// Keep messages up to the msgID (inclusive/exclusive depending on list)
				var filtered []*discordgo.Message
				found := false
				for _, m := range mList {
					filtered = append(filtered, m)
					if m.ID == msgID {
						found = true
						break
					}
				}
				if found {
					mList = filtered
				}
			}
		} else if sub == "between" {
			if len(ctx.Args) < startIdx+3 {
				return ctx.Reply("Usage: `.purge between <start_id/link> <finish_id/link>`")
			}
			startID := resolveMsgID(ctx.Args[startIdx+1])
			finishID := resolveMsgID(ctx.Args[startIdx+2])

			mList, err = ctx.Session.ChannelMessages(cid, 100, "", "", "")
			if err == nil {
				var filtered []*discordgo.Message
				inRange := false
				for _, m := range mList {
					if m.ID == startID || m.ID == finishID {
						if inRange {
							filtered = append(filtered, m)
							break
						}
						inRange = true
					}
					if inRange {
						filtered = append(filtered, m)
					}
				}
				mList = filtered
			}
		} else {
			// Normal filters or default search
			mList, err = ctx.Session.ChannelMessages(cid, amount, "", "", "")
		}

		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to fetch messages: %v", err))
		}

		// Filter the messages
		var delIDs []string
		now := time.Now()

		var filterArgs []string
		if len(ctx.Args) > startIdx+1 {
			filterArgs = ctx.Args[startIdx+1:]
		}

		// Default search parameters if not a filter keyword
		isFilter := isFilterKeyword(sub)
		var filterMember *discordgo.Member
		searchQuery := ""

		if !isFilter && sub != "" && sub != "before" && sub != "after" && sub != "upto" && sub != "between" {
			// Check if first arg is a member
			filterMember, _ = moderation.ResolveMember(ctx.Session, gid, ctx.Args[startIdx])
			if filterMember == nil {
				searchQuery = strings.ToLower(strings.Join(ctx.Args[startIdx:], " "))
			}
		}

		for _, m := range mList {
			if len(delIDs) >= amount {
				break
			}
			if ctx.Message != nil && m.ID == ctx.Message.ID {
				continue
			}

			keep := false
			if isFilter {
				keep = matchFilter(m, sub, filterArgs)
			} else if filterMember != nil {
				keep = m.Author != nil && m.Author.ID == filterMember.User.ID
			} else if searchQuery != "" {
				keep = strings.Contains(strings.ToLower(m.Content), searchQuery)
			} else {
				// No filter, delete everything
				keep = true
			}

			if keep {
				delIDs = append(delIDs, m.ID)
			}
		}

		if len(delIDs) == 0 {
			return ctx.Reply("[+] No matching messages found to clear.")
		}

		// Split into bulk delete (under 14 days) and single delete (older)
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

		moderation.LogAction(ctx.Session, ctx.DB, gid, "Bulk Clear", ctx.AuthorID(), cid,
			fmt.Sprintf("Cleared %d messages in <#%s>.", cnt, cid),
			config.Field("Filter", sub, true),
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

func isFilterKeyword(s string) bool {
	switch s {
	case "startswith", "endswith", "contains", "embeds", "files", "images", "bots",
		"humans", "webhooks", "links", "activity", "reactions", "stickers", "mentions", "emoji", "emotes":
		return true
	}
	return false
}

func matchFilter(m *discordgo.Message, sub string, args []string) bool {
	switch sub {
	case "startswith":
		if len(args) == 0 {
			return false
		}
		return strings.HasPrefix(m.Content, strings.Join(args, " "))
	case "endswith":
		if len(args) == 0 {
			return false
		}
		return strings.HasSuffix(m.Content, strings.Join(args, " "))
	case "contains":
		if len(args) == 0 {
			return false
		}
		return strings.Contains(m.Content, strings.Join(args, " "))
	case "embeds":
		return len(m.Embeds) > 0
	case "files":
		return len(m.Attachments) > 0
	case "images":
		if len(m.Attachments) > 0 {
			for _, a := range m.Attachments {
				if strings.HasPrefix(a.ContentType, "image/") {
					return true
				}
			}
		}
		lower := strings.ToLower(m.Content)
		return strings.Contains(lower, ".png") || strings.Contains(lower, ".jpg") ||
			strings.Contains(lower, ".jpeg") || strings.Contains(lower, ".gif") ||
			strings.Contains(lower, ".webp")
	case "bots":
		return m.Author != nil && m.Author.Bot
	case "humans":
		return m.Author != nil && !m.Author.Bot
	case "webhooks":
		return m.WebhookID != ""
	case "links":
		return strings.Contains(m.Content, "http://") || strings.Contains(m.Content, "https://")
	case "activity":
		return m.Type == discordgo.MessageTypeGuildMemberJoin || m.Type == discordgo.MessageTypeUserPremiumGuildSubscription ||
			m.Type == discordgo.MessageTypeUserPremiumGuildSubscriptionTierOne || m.Type == discordgo.MessageTypeUserPremiumGuildSubscriptionTierTwo ||
			m.Type == discordgo.MessageTypeUserPremiumGuildSubscriptionTierThree
	case "reactions":
		return len(m.Reactions) > 0
	case "stickers":
		return len(m.StickerItems) > 0
	case "mentions":
		if len(args) == 0 {
			return false
		}
		uid := strings.Trim(args[0], "<@!?")
		for _, ment := range m.Mentions {
			if ment.ID == uid {
				return true
			}
		}
		return false
	case "emoji", "emotes":
		return strings.Contains(m.Content, "<:") || strings.Contains(m.Content, "<a:")
	}
	return false
}

func resolveMsgID(arg string) string {
	if m := rxPurgeMsgLink.FindStringSubmatch(arg); len(m) > 1 {
		return m[1]
	}
	return arg
}

func sfTime(id string) (time.Time, error) {
	sf, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	t := (sf >> 22) + 1420070400000
	return time.Unix(0, t*int64(time.Millisecond)), nil
}
