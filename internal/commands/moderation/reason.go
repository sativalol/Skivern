package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var Reason = &manager.Command{
	Trigger:     "reason",
	Name:        "reason",
	Description: "Update or view the reason for the last moderation action in modlog",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageGuild) {
			return ctx.Reply("[!] Only Administrators / Managers can modify case reasons.")
		}

		gid := ctx.GuildID()
		list, err := ctx.DB.ListCases(gid, "")
		if err != nil || len(list) == 0 {
			return ctx.Reply("[!] No moderation cases found in this guild.")
		}

		latestCase := list[len(list)-1]

		if len(ctx.Args) == 0 {
			return ctx.Reply(fmt.Sprintf("[+] Last Mod Action: Case #%d (%s) on <@%s> | Current Reason: *%s*",
				latestCase.ID, strings.ToUpper(latestCase.Type), latestCase.UserID, latestCase.Reason))
		}

		newReason := strings.Join(ctx.Args, " ")
		updatedCase := storage.Case{
			ID:        latestCase.ID,
			GuildID:   latestCase.GuildID,
			UserID:    latestCase.UserID,
			ModID:     latestCase.ModID,
			Type:      latestCase.Type,
			Reason:    newReason,
			Timestamp: latestCase.Timestamp,
		}

		err = ctx.DB.DeleteCase(gid, latestCase.ID)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to update case: %v", err))
		}
		_, err = ctx.DB.AddCase(gid, updatedCase)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to update case: %v", err))
		}

		return ctx.Reply(fmt.Sprintf("[+] Updated Case #%d reason to: *%s*", latestCase.ID, newReason))
	},
}
