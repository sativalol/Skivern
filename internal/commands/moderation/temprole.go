package moderation

import (
	"fmt"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"skyvern/internal/storage"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var Temprole = &manager.Command{
	Trigger:     "temprole",
	Aliases:     []string{"tr"},
	Name:        "temprole",
	Description: "Temporarily assign a role to a member or members",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageRoles) {
			return ctx.Reply("[!] You need Manage Roles permission.")
		}

		if len(ctx.Args) < 3 {
			return ctx.Reply("Usage: .temprole <user|all> <role> <duration> (e.g. .temprole @user @Role 1h)")
		}

		gid := ctx.GuildID()

		durStr := ctx.Args[len(ctx.Args)-1]
		dur, err := time.ParseDuration(durStr)
		if err != nil {
			return ctx.Reply("[!] Invalid duration format. Examples: 30m, 2h, 1d.")
		}
		expiresAt := time.Now().Add(dur)

		roleArg := ctx.Args[len(ctx.Args)-2]
		rid := resolveRole(ctx.Session, gid, roleArg)
		if rid == "" {
			return ctx.Reply("[!] Could not resolve role.")
		}

		userArgs := ctx.Args[:len(ctx.Args)-2]

		if len(userArgs) == 1 && strings.ToLower(userArgs[0]) == "all" {
			members, err := ctx.Session.GuildMembers(gid, "", 1000)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to fetch guild members: %v", err))
			}
			go func() {
				for _, m := range members {
					if m.User.Bot {
						continue
					}
					_ = ctx.Session.GuildMemberRoleAdd(gid, m.User.ID, rid)
					_ = ctx.DB.SaveTempRole(storage.TempRole{
						GuildID:   gid,
						UserID:    m.User.ID,
						RoleID:    rid,
						ExpiresAt: expiresAt,
					})
				}
			}()
			return ctx.Reply(fmt.Sprintf("[+] Temporarily assigning role to everyone for %s.", durStr))
		}

		var targets []string
		for _, arg := range userArgs {
			subparts := strings.Split(arg, ",")
			for _, part := range subparts {
				p := strings.TrimSpace(part)
				if p != "" {
					targets = append(targets, p)
				}
			}
		}

		cnt := 0
		for _, target := range targets {
			m, err := moderation.ResolveMember(ctx.Session, gid, target)
			if err != nil || m == nil {
				continue
			}
			err = ctx.Session.GuildMemberRoleAdd(gid, m.User.ID, rid)
			if err == nil {
				_ = ctx.DB.SaveTempRole(storage.TempRole{
					GuildID:   gid,
					UserID:    m.User.ID,
					RoleID:    rid,
					ExpiresAt: expiresAt,
				})
				cnt++
			}
		}

		return ctx.Reply(fmt.Sprintf("[+] Temporarily assigned role to %d members for %s.", cnt, durStr))
	},
}
