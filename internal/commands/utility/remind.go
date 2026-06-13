package utility

import (
	"fmt"
	"math/rand"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strconv"
	"strings"
	"time"
)

func init() {
	manager.RegisterHelp("remind", []manager.HelpPage{
		{
			Command:     "Remind",
			Syntax:      ".remind <time> <message>",
			Description: "Set a reminder (e.g. 2h Buy milk).",
		},
		{
			Command:     "Remind List",
			Syntax:      ".remind list",
			Description: "List your active reminders.",
		},
		{
			Command:     "Remind Cancel",
			Syntax:      ".remind cancel <id>",
			Description: "Cancel an active reminder.",
		},
	})
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.ToLower(s)
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

var Remind = &manager.Command{
	Trigger:     "remind",
	Aliases:     []string{"reminder", "remindme"},
	Name:        "remind",
	Description: "Set a reminder",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("remind")
		}

		sub := strings.ToLower(ctx.Args[0])
		uid := ctx.AuthorID()

		switch sub {
		case "list":
			list := ctx.Mgr.ListReminders(uid)
			if len(list) == 0 {
				return ctx.Reply("[*] You have no active reminders.")
			}
			var sb strings.Builder
			sb.WriteString("Your Active Reminders:\n\n")
			for _, r := range list {
				timeStr := r.Time.Format("2006-01-02 15:04:05 MST")
				sb.WriteString(fmt.Sprintf("- `ID: %s` | Due: %s | Message: %s\n", r.ID, timeStr, r.Message))
			}
			return ctx.Reply(sb.String())

		case "cancel", "remove":
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("remind")
			}
			id := ctx.Args[1]
			err := ctx.Mgr.DeleteReminder(uid, id)
			if err != nil {
				return ctx.Reply("[!] Reminder not found or could not be cancelled.")
			}
			return ctx.Reply(fmt.Sprintf("[+] Reminder `%s` cancelled.", id))

		default:
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("remind")
			}
			timeArg := ctx.Args[0]
			msgText := strings.Join(ctx.Args[1:], " ")

			dur, err := parseDuration(timeArg)
			if err != nil {
				return ctx.Reply("[!] Invalid time duration. Use formats like: 10s, 5m, 2h, 1d.")
			}

			id := fmt.Sprintf("%04x", rand.Intn(0xffff))
			r := storage.Reminder{
				ID:      id,
				UserID:  uid,
				Time:    time.Now().Add(dur),
				Message: msgText,
			}

			err = ctx.Mgr.SaveReminder(r)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save reminder: %v", err))
			}

			dueStr := r.Time.Format("2006-01-02 15:04:05 MST")
			return ctx.Reply(fmt.Sprintf("[+] Reminder set! (ID: `%s`). I will remind you at `%s`.", id, dueStr))
		}
	},
}

var Reminders = &manager.Command{
	Trigger:     "reminders",
	Name:        "reminders",
	Description: "List your active reminders",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		ctx.Args = []string{"list"}
		return Remind.Execute(ctx)
	},
}
