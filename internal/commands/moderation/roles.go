package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var Roles = &manager.Command{
	Trigger:     "roles",
	Name:        "roles",
	Description: "List all roles in the server from highest to lowest position",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageRoles) {
			return ctx.Reply("[!] You need Manage Roles permission.")
		}

		gid := ctx.GuildID()
		roles, err := ctx.Session.GuildRoles(gid)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to fetch roles: %v", err))
		}

		sort.Slice(roles, func(i, j int) bool {
			return roles[i].Position > roles[j].Position
		})

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Total Roles: %d\n\n", len(roles)))
		for _, r := range roles {
			sb.WriteString(fmt.Sprintf("`@%-25s` | Position: %d | ID: `%s` | Color: `#%06x`\n",
				r.Name, r.Position, r.ID, r.Color))
		}

		e := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       "Server Roles",
			Description: sb.String(),
		})
		return ctx.Respond(e)
	},
}
