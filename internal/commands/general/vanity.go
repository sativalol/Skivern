package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("vanity", []manager.HelpPage{
		{
			Command:     "Vanity Set",
			Syntax:      ".vanity set <text>",
			Description: "Set custom status string to match.",
		},
		{
			Command:     "Vanity Role",
			Syntax:      ".vanity role <role>",
			Description: "Set status reward role.",
		},
		{
			Command:     "Vanity Enable",
			Syntax:      ".vanity enable",
			Description: "Enable vanity rewards.",
		},
		{
			Command:     "Vanity Disable",
			Syntax:      ".vanity disable",
			Description: "Disable vanity rewards.",
		},
	})
}

var rxVanityRole = regexp.MustCompile(`^<@&(\d+)>$`)

var Vanity = &manager.Command{
	Trigger:     "vanity",
	Aliases:     []string{"statusreward", "vr"},
	Name:        "vanity",
	Description: "Configure vanity status rewards",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageServer) {
			return ctx.Reply("[!] You need Manage Server permission to configure vanity rewards.")
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("vanity")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		cfg, err := ctx.DB.GetVanityCfg(gid)
		if err != nil {
			cfg = storage.VanityCfg{Enabled: false}
		}

		switch sub {
		case "set":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.vanity set <text>`")
			}
			cfg.Text = strings.Join(ctx.Args[1:], " ")
			_ = ctx.DB.SaveVanityCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Set status matching text to `%s`.", cfg.Text))

		case "role":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.vanity role <@role/ID>`")
			}
			roleArg := ctx.Args[1]
			rid := ""
			if m := rxVanityRole.FindStringSubmatch(roleArg); len(m) > 1 {
				rid = m[1]
			} else {
				rid = roleArg
			}

			roles, err := ctx.Session.GuildRoles(gid)
			if err != nil {
				return ctx.Reply("[!] Failed to verify guild roles.")
			}
			found := false
			for _, r := range roles {
				if r.ID == rid {
					found = true
					break
				}
			}
			if !found {
				return ctx.Reply("[!] Invalid role.")
			}

			cfg.RoleID = rid
			_ = ctx.DB.SaveVanityCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Set reward role to <@&%s>.", rid))

		case "enable":
			if cfg.Text == "" || cfg.RoleID == "" {
				return ctx.Reply("[!] Please configure both text and role first.")
			}
			cfg.Enabled = true
			_ = ctx.DB.SaveVanityCfg(gid, cfg)
			return ctx.Reply("[+] Vanity status rewards enabled.")

		case "disable":
			cfg.Enabled = false
			_ = ctx.DB.SaveVanityCfg(gid, cfg)
			return ctx.Reply("[+] Vanity status rewards disabled.")

		default:
			return ctx.Reply("Unknown subcommand. Use set, role, enable, or disable.")
		}
	},
}
