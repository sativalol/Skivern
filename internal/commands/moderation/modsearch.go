package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var ModSearch = &manager.Command{
	Trigger:     "modsearch",
	Aliases:     []string{"msrch"},
	Name:        "modsearch",
	Description: "Search moderation actions matching target, moderator, action type, or reason",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageMessages) {
			return ctx.Reply("[!] You need Manage Messages permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: modsearch <query>")
		}

		query := strings.ToLower(strings.Join(ctx.Args, " "))
		gid := ctx.GuildID()

		list, err := ctx.DB.ListCases(gid, "")
		if err != nil || len(list) == 0 {
			return ctx.Reply("[+] No moderation cases recorded in this server.")
		}

		var matches []string
		for _, c := range list {
			match := false
			if strings.Contains(strings.ToLower(c.UserID), query) {
				match = true
			} else if strings.Contains(strings.ToLower(c.ModID), query) {
				match = true
			} else if strings.Contains(strings.ToLower(c.Type), query) {
				match = true
			} else if strings.Contains(strings.ToLower(c.Reason), query) {
				match = true
			}

			if !match {
				// try to resolve user names from query
				m, _ := moderation.ResolveMember(ctx.Session, gid, c.UserID)
				if m != nil && strings.Contains(strings.ToLower(m.User.Username), query) {
					match = true
				}
				mod, _ := moderation.ResolveMember(ctx.Session, gid, c.ModID)
				if mod != nil && strings.Contains(strings.ToLower(mod.User.Username), query) {
					match = true
				}
			}

			if match {
				matches = append(matches, fmt.Sprintf("`#%d` | **%s** | Target: <@%s> | Mod: <@%s>\n└ Reason: *%s* (%s)",
					c.ID, strings.ToUpper(c.Type), c.UserID, c.ModID, c.Reason, c.Timestamp.Format("2006-01-02 15:04")))
			}
		}

		if len(matches) == 0 {
			return ctx.Reply(fmt.Sprintf("[+] No moderation actions found matching query: `%s`", query))
		}

		limit := len(matches)
		if limit > 10 {
			limit = 10
		}

		desc := strings.Join(matches[len(matches)-limit:], "\n\n")
		if len(matches) > 10 {
			desc = fmt.Sprintf("Showing latest 10 of %d matches:\n\n", len(matches)) + desc
		}

		e := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:       fmt.Sprintf("Moderation Search results for: %s", query),
			Description: desc,
		})
		return ctx.Respond(e)
	},
}
