package utility

import (
	"fmt"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strings"
	"time"
)

var AFK = &manager.Command{
	Trigger:     "afk",
	Aliases:     []string{"brb", "away"},
	Name:        "afk",
	Description: "Set your AFK status with an optional reason",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		uid := ctx.AuthorID()

		reason := "AFK"
		if len(ctx.Args) > 0 {
			reason = strings.Join(ctx.Args, " ")
		}

		status := storage.AFKStatus{
			Reason: reason,
			Time:   time.Now(),
			Pings:  0,
		}

		err := ctx.DB.SaveAFK(gid, uid, status)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to set AFK status: %v", err))
		}

		return ctx.Reply(fmt.Sprintf("[+] <@%s> is now AFK: %s", uid, reason))
	},
}
