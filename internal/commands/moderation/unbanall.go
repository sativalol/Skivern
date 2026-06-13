package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"strings"
	"sync"
	"time"
)

var (
	unbanTasks   = make(map[string]chan struct{})
	unbanTasksMu sync.Mutex
)

func init() {
	manager.RegisterHelp("unbanall", []manager.HelpPage{
		{
			Command:     "Unban All",
			Syntax:      ".unbanall",
			Description: "Unbans every member in the server. Server Owner only.",
		},
		{
			Command:     "Unban All Cancel",
			Syntax:      ".unbanall cancel",
			Description: "Cancels an ongoing unbanall process. Server Owner only.",
		},
	})
}

var UnbanAll = &manager.Command{
	Trigger:     "unbanall",
	Name:        "unbanall",
	Description: "Unbans every banned member in the guild",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		g, err := ctx.Session.State.Guild(gid)
		if err != nil {
			g, err = ctx.Session.Guild(gid)
		}
		if err != nil || g.OwnerID != ctx.AuthorID() {
			return ctx.Reply("[!] Only the Server Owner can execute this command.")
		}

		if len(ctx.Args) > 0 && strings.ToLower(ctx.Args[0]) == "cancel" {
			unbanTasksMu.Lock()
			ch, ok := unbanTasks[gid]
			if ok {
				close(ch)
				delete(unbanTasks, gid)
			}
			unbanTasksMu.Unlock()

			if ok {
				return ctx.Reply("[+] Unbanall task cancellation requested.")
			}
			return ctx.Reply("[!] No unbanall task is currently running in this server.")
		}

		unbanTasksMu.Lock()
		if _, exists := unbanTasks[gid]; exists {
			unbanTasksMu.Unlock()
			return ctx.Reply("[!] An unbanall task is already running in this server. Use `.unbanall cancel` to stop it.")
		}

		cancelCh := make(chan struct{})
		unbanTasks[gid] = cancelCh
		unbanTasksMu.Unlock()

		_ = ctx.Reply("[*] Fetching ban list and starting unbanall process...")

		go func() {
			defer func() {
				unbanTasksMu.Lock()
				delete(unbanTasks, gid)
				unbanTasksMu.Unlock()
			}()

			var lastUserID string
			unbannedCount := 0

			for {
				select {
				case <-cancelCh:
					_, _ = ctx.Session.ChannelMessageSend(ctx.ChanID(), fmt.Sprintf("[-] Unbanall process cancelled. Unbanned %d members.", unbannedCount))
					return
				default:
				}

				// Fetch in batches of 1000
				bans, err := ctx.Session.GuildBans(gid, 1000, lastUserID, "")
				if err != nil {
					_, _ = ctx.Session.ChannelMessageSend(ctx.ChanID(), fmt.Sprintf("[!] Error fetching bans: %v", err))
					return
				}

				if len(bans) == 0 {
					break
				}

				for _, ban := range bans {
					select {
					case <-cancelCh:
						_, _ = ctx.Session.ChannelMessageSend(ctx.ChanID(), fmt.Sprintf("[-] Unbanall process cancelled. Unbanned %d members.", unbannedCount))
						return
					default:
					}

					err := ctx.Session.GuildBanDelete(gid, ban.User.ID)
					if err == nil {
						unbannedCount++
					}
					// Avoid hitting Discord rate limit heavily
					time.Sleep(150 * time.Millisecond)
				}

				lastUserID = bans[len(bans)-1].User.ID
				if len(bans) < 1000 {
					break
				}
			}

			_, _ = ctx.Session.ChannelMessageSend(ctx.ChanID(), fmt.Sprintf("[+] Unbanall process completed. Successfully unbanned %d members.", unbannedCount))
		}()

		return nil
	},
}
