package utility

import (
	"fmt"
	"skyvern/internal/manager"

	"github.com/bwmarrin/discordgo"
)

var MoveAll = &manager.Command{
	Trigger:     "moveall",
	Name:        "moveall",
	Description: "Move all members from one Voice Channel to another",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		if ctx.Message != nil {
			p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
			if err != nil || (p&discordgo.PermissionVoiceMoveMembers) == 0 {
				return ctx.Reply("[!] You need Move Members permission to move members.")
			}
		}

		if len(ctx.Args) < 2 {
			return ctx.Reply("Usage: moveall <from_vc_id> <to_vc_id>")
		}

		gid := ctx.GuildID()
		fromVC := ctx.Args[0]
		toVC := ctx.Args[1]

		g, err := ctx.Session.State.Guild(gid)
		if err != nil {
			g, err = ctx.Session.Guild(gid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to fetch guild: %v", err))
			}
		}

		cnt := 0
		for _, vs := range g.VoiceStates {
			if vs.ChannelID == fromVC {
				err := ctx.Session.GuildMemberMove(gid, vs.UserID, &toVC)
				if err == nil {
					cnt++
				}
			}
		}

		return ctx.Reply(fmt.Sprintf("[+] Successfully moved %d members from voice channel <#%s> to <#%s>.", cnt, fromVC, toVC))
	},
}
