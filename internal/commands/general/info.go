package general

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func resolveUser(s *discordgo.Session, gid, query string) (*discordgo.User, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	m, err := moderation.ResolveMember(s, gid, q)
	if err == nil && m != nil && m.User != nil {
		return m.User, nil
	}
	raw := q
	if strings.HasPrefix(raw, "<@") && strings.HasSuffix(raw, ">") {
		raw = strings.Trim(raw, "<@!>")
	}
	return s.User(raw)
}

func creationTime(id string) time.Time {
	var v uint64
	for _, r := range id {
		if r >= '0' && r <= '9' {
			v = v*10 + uint64(r-'0')
		}
	}
	t := (v >> 22) + 1420070400000
	return time.Unix(0, int64(t)*int64(time.Millisecond))
}

var ServerInfo = &manager.Command{
	Trigger:     "serverinfo",
	Aliases:     []string{"si", "server", "guildinfo"},
	Name:        "serverinfo",
	Description: "Display detailed information about the current server",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		g, err := ctx.Session.State.Guild(gid)
		if err != nil {
			g, err = ctx.Session.Guild(gid)
			if err != nil {
				return ctx.Reply("[!] Failed to fetch server info.")
			}
		}

		created := creationTime(g.ID)
		owner, _ := ctx.Session.User(g.OwnerID)
		ownerTag := g.OwnerID
		if owner != nil {
			ownerTag = fmt.Sprintf("%s (%s)", owner.Username, owner.ID)
		}

		fields := []*discordgo.MessageEmbedField{
			config.Field("Owner", ownerTag, true),
			config.Field("Server ID", g.ID, true),
			config.Field("Created At", created.Format("2006-01-02 15:04:05"), true),
			config.Field("Members", fmt.Sprintf("%d members", g.MemberCount), true),
			config.Field("Roles", fmt.Sprintf("%d roles", len(g.Roles)), true),
			config.Field("Channels", fmt.Sprintf("%d channels", len(g.Channels)), true),
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:        g.Name,
			Description:  fmt.Sprintf("Server information for **%s**", g.Name),
			Fields:       fields,
			ThumbnailURL: g.IconURL("256"),
		})

		return ctx.Respond(emb)
	},
}

var RoleInfo = &manager.Command{
	Trigger:     "roleinfo",
	Aliases:     []string{"ri"},
	Name:        "roleinfo",
	Description: "Display detailed information about a role",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: .roleinfo <role>")
		}

		gid := ctx.GuildID()
		roleArg := strings.Join(ctx.Args, " ")
		rid := resolveRole(ctx.Session, gid, roleArg)
		if rid == "" {
			return ctx.Reply("[!] Could not resolve role.")
		}

		roles, err := ctx.Session.GuildRoles(gid)
		if err != nil {
			return ctx.Reply("[!] Failed to fetch server roles.")
		}

		var targetRole *discordgo.Role
		for _, r := range roles {
			if r.ID == rid {
				targetRole = r
				break
			}
		}

		if targetRole == nil {
			return ctx.Reply("[!] Role not found.")
		}

		created := creationTime(targetRole.ID)
		colorHex := fmt.Sprintf("#%06X", targetRole.Color)

		fields := []*discordgo.MessageEmbedField{
			config.Field("Role ID", targetRole.ID, true),
			config.Field("Color", colorHex, true),
			config.Field("Position", fmt.Sprintf("%d", targetRole.Position), true),
			config.Field("Mentionable", fmt.Sprintf("%t", targetRole.Mentionable), true),
			config.Field("Managed", fmt.Sprintf("%t", targetRole.Managed), true),
			config.Field("Created At", created.Format("2006-01-02 15:04:05"), true),
			config.Field("Permissions", fmt.Sprintf("`%d`", targetRole.Permissions), false),
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:  "Role Info: " + targetRole.Name,
			Fields: fields,
		})

		return ctx.Respond(emb)
	},
}

var Whois = &manager.Command{
	Trigger:     "whois",
	Aliases:     []string{"userinfo", "ui", "user"},
	Name:        "whois",
	Description: "Fetch technical data on a specific user",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		query := ""
		if len(ctx.Args) > 0 {
			query = ctx.Args[0]
		} else {
			query = ctx.AuthorID()
		}

		usr, err := resolveUser(ctx.Session, gid, query)
		if err != nil || usr == nil {
			return ctx.Reply("[!] Could not resolve user.")
		}

		created := creationTime(usr.ID)
		joined := "Not in server"
		rolesStr := "None"

		mem, err := ctx.Session.State.Member(gid, usr.ID)
		if err != nil {
			mem, _ = ctx.Session.GuildMember(gid, usr.ID)
		}

		if mem != nil {
			if !mem.JoinedAt.IsZero() {
				joined = mem.JoinedAt.Format("2006-01-02 15:04:05")
			}
			if len(mem.Roles) > 0 {
				var mentionList []string
				for _, r := range mem.Roles {
					mentionList = append(mentionList, "<@&"+r+">")
				}
				rolesStr = strings.Join(mentionList, ", ")
			}
		}

		fields := []*discordgo.MessageEmbedField{
			config.Field("Username", usr.Username, true),
			config.Field("User ID", usr.ID, true),
			config.Field("Registered At", created.Format("2006-01-02 15:04:05"), true),
			config.Field("Joined Server At", joined, true),
			config.Field("Roles", rolesStr, false),
		}

		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:        fmt.Sprintf("User Info - %s", usr.Username),
			Fields:       fields,
			ThumbnailURL: usr.AvatarURL("256"),
		})

		return ctx.Respond(emb)
	},
}

var Pfp = &manager.Command{
	Trigger:     "pfp",
	Aliases:     []string{"avatar", "av"},
	Name:        "pfp",
	Description: "Shows a user's profile picture",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		query := ""
		if len(ctx.Args) > 0 {
			query = ctx.Args[0]
		} else {
			query = ctx.AuthorID()
		}

		usr, err := resolveUser(ctx.Session, gid, query)
		if err != nil || usr == nil {
			return ctx.Reply("[!] Could not resolve user.")
		}

		url := usr.AvatarURL("2048")
		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:    fmt.Sprintf("%s's Profile Picture", usr.Username),
			ImageURL: url,
		})

		return ctx.Respond(emb)
	},
}

var Banner = &manager.Command{
	Trigger:     "banner",
	Name:        "banner",
	Description: "Displays the server's banner or a user's banner",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()

		if len(ctx.Args) == 0 {
			g, err := ctx.Session.State.Guild(gid)
			if err != nil {
				g, err = ctx.Session.Guild(gid)
			}
			if err != nil || g.Banner == "" {
				return ctx.Reply("[!] Server does not have a banner set.")
			}
			url := g.BannerURL("2048")
			emb := config.Build(ctx.Cfg, config.EmbedOpt{
				Title:    fmt.Sprintf("%s's Server Banner", g.Name),
				ImageURL: url,
			})
			return ctx.Respond(emb)
		}

		usr, err := resolveUser(ctx.Session, gid, ctx.Args[0])
		if err != nil || usr == nil {
			return ctx.Reply("[!] Could not resolve user.")
		}

		fullUser, err := ctx.Session.User(usr.ID)
		if err != nil || fullUser.Banner == "" {
			return ctx.Reply(fmt.Sprintf("[!] **%s** does not have a banner set.", usr.Username))
		}

		url := fullUser.BannerURL("2048")
		emb := config.Build(ctx.Cfg, config.EmbedOpt{
			Title:    fmt.Sprintf("%s's User Banner", usr.Username),
			ImageURL: url,
		})

		return ctx.Respond(emb)
	},
}
