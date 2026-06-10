package utility

import (
	"fmt"
	"math/rand"
	"regexp"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("schedule", []manager.HelpPage{
		{
			Command:     "Schedule",
			Syntax:      ".schedule <time> <channel> <message>",
			Description: "Schedule a message to be sent at a specific time.",
		},
		{
			Command:     "Schedule List",
			Syntax:      ".schedule list",
			Description: "List active scheduled messages for the server.",
		},
		{
			Command:     "Schedule Cancel",
			Syntax:      ".schedule cancel <id>",
			Description: "Cancel a scheduled message.",
		},
	})
}

var rxSchedChan = regexp.MustCompile(`^<#(\d+)>$`)

var Schedule = &manager.Command{
	Trigger:     "schedule",
	Aliases:     []string{"schedulemsg", "sched"},
	Name:        "schedule",
	Description: "Schedule a message to be sent at a specific time",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionManageMessages) == 0 {
			return ctx.Reply("[!] You need Manage Messages permission to schedule messages.")
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("schedule")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		switch sub {
		case "list":
			list := ctx.Mgr.ListSchedules(gid)
			if len(list) == 0 {
				return ctx.Reply("[*] No scheduled messages for this server.")
			}
			var sb strings.Builder
			sb.WriteString("Scheduled Messages:\n\n")
			for _, s := range list {
				timeStr := s.Time.Format("2006-01-02 15:04:05 MST")
				sb.WriteString(fmt.Sprintf("- `ID: %s` | Send: %s | Target: <#%s> | Msg: %s\n", s.ID, timeStr, s.ChannelID, s.Message))
			}
			return ctx.Reply(sb.String())

		case "cancel":
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("schedule")
			}
			id := ctx.Args[1]
			err := ctx.Mgr.DeleteSchedule(gid, id)
			if err != nil {
				return ctx.Reply("[!] Scheduled message not found or could not be cancelled.")
			}
			return ctx.Reply(fmt.Sprintf("[+] Scheduled message `%s` cancelled.", id))

		default:
			if len(ctx.Args) < 3 {
				return ctx.SendHelp("schedule")
			}
			timeArg := ctx.Args[0]
			chanArg := ctx.Args[1]
			msgText := strings.Join(ctx.Args[2:], " ")

			dur, err := parseDuration(timeArg)
			if err != nil {
				return ctx.Reply("[!] Invalid time duration. Use formats like: 10s, 5m, 2h, 1d.")
			}

			cid := ""
			if m := rxSchedChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			} else {
				cid = chanArg
			}

			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Could not resolve target text channel.")
			}

			id := fmt.Sprintf("%04x", rand.Intn(0xffff))
			s := storage.ScheduledMsg{
				ID:        id,
				GuildID:   gid,
				ChannelID: cid,
				Time:      time.Now().Add(dur),
				Message:   msgText,
			}

			err = ctx.Mgr.SaveSchedule(s)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save scheduled message: %v", err))
			}

			dueStr := s.Time.Format("2006-01-02 15:04:05 MST")
			return ctx.Reply(fmt.Sprintf("[+] Message scheduled! (ID: `%s`). It will be sent to <#%s> at `%s`.", id, cid, dueStr))
		}
	},
}
