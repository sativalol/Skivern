package moderation

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("role", []manager.HelpPage{
		{
			Command:     "Role Add",
			Syntax:      ".role add <user> <role>",
			Description: "Assign a role to a specific server member.",
		},
		{
			Command:     "Role Remove",
			Syntax:      ".role remove <user> <role>",
			Description: "Strip a role from a specific server member.",
		},
		{
			Command:     "Role Create",
			Syntax:      ".role create <name> [color_hex]",
			Description: "Create a new role with an optional color code.",
		},
		{
			Command:     "Role Delete",
			Syntax:      ".role delete <role>",
			Description: "Permanently delete a role from the server.",
		},
		{
			Command:     "Role Edit Name",
			Syntax:      ".role edit <role> name <new_name>",
			Description: "Rename an existing role.",
		},
		{
			Command:     "Role Edit Color",
			Syntax:      ".role edit <role> color <color_hex>",
			Description: "Change the display color of a role.",
		},
		{
			Command:     "Role Mass Add",
			Syntax:      ".role massadd <role>",
			Description: "Assign a role to every single member of the server.",
		},
	})
}

var rxRole = regexp.MustCompile(`^<@&(\d+)>$`)

var Role = &manager.Command{
	Trigger:     "role",
	Aliases:     []string{"r"},
	Name:        "role",
	Description: "Manage server roles (add, remove, edit, delete, create, mass add, etc.)",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageRoles) {
			return ctx.Reply("[!] You need Manage Roles permission.")
		}

		if len(ctx.Args) < 2 {
			return ctx.SendHelp("role")
		}

		gid := ctx.GuildID()
		sub := strings.ToLower(ctx.Args[0])

		switch sub {
		case "add":
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			rid := resolveRole(ctx.Session, gid, ctx.Args[2])
			if rid == "" {
				return ctx.Reply("[!] Could not resolve role.")
			}
			err = ctx.Session.GuildMemberRoleAdd(gid, m.User.ID, rid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to add role: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Added role to **%s**.", m.User.Username))

		case "remove":
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			rid := resolveRole(ctx.Session, gid, ctx.Args[2])
			if rid == "" {
				return ctx.Reply("[!] Could not resolve role.")
			}
			err = ctx.Session.GuildMemberRoleRemove(gid, m.User.ID, rid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to remove role: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Removed role from **%s**.", m.User.Username))

		case "create":
			name := ctx.Args[1]
			color := 0
			if len(ctx.Args) > 2 {
				colStr := strings.TrimPrefix(ctx.Args[2], "#")
				if val, err := strconv.ParseInt(colStr, 16, 32); err == nil {
					color = int(val)
				}
			}
			r, err := ctx.Session.GuildRoleCreate(gid, &discordgo.RoleParams{
				Name:  name,
				Color: &color,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to create role: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Created role **%s** (`%s`).", r.Name, r.ID))

		case "delete":
			rid := resolveRole(ctx.Session, gid, ctx.Args[1])
			if rid == "" {
				return ctx.Reply("[!] Could not resolve role.")
			}
			err := ctx.Session.GuildRoleDelete(gid, rid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to delete role: %v", err))
			}
			return ctx.Reply("[+] Deleted role successfully.")

		case "edit":
			if len(ctx.Args) < 4 {
				return ctx.Reply("Usage:\n`.role edit <role> name <new_name>`\n`.role edit <role> color <color_hex>`")
			}
			rid := resolveRole(ctx.Session, gid, ctx.Args[1])
			if rid == "" {
				return ctx.Reply("[!] Could not resolve role.")
			}
			prop := strings.ToLower(ctx.Args[2])
			val := strings.Join(ctx.Args[3:], " ")

			roles, err := ctx.Session.GuildRoles(gid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to fetch roles: %v", err))
			}
			var curRole *discordgo.Role
			for _, r := range roles {
				if r.ID == rid {
					curRole = r
					break
				}
			}
			if curRole == nil {
				return ctx.Reply("[!] Role not found.")
			}

			params := &discordgo.RoleParams{
				Name:        curRole.Name,
				Color:       &curRole.Color,
				Hoist:       &curRole.Hoist,
				Mentionable: &curRole.Mentionable,
			}

			if prop == "name" {
				params.Name = val
			} else if prop == "color" {
				colStr := strings.TrimPrefix(val, "#")
				if colVal, err := strconv.ParseInt(colStr, 16, 32); err == nil {
					colorInt := int(colVal)
					params.Color = &colorInt
				} else {
					return ctx.Reply("[!] Invalid color format.")
				}
			} else {
				return ctx.Reply("[!] Invalid edit property. Use 'name' or 'color'.")
			}

			_, err = ctx.Session.GuildRoleEdit(gid, rid, params)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to edit role: %v", err))
			}
			return ctx.Reply("[+] Edited role successfully.")

		case "massadd":
			rid := resolveRole(ctx.Session, gid, ctx.Args[1])
			if rid == "" {
				return ctx.Reply("[!] Could not resolve role.")
			}
			mList, err := ctx.Session.GuildMembers(gid, "", 1000)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to fetch members: %v", err))
			}
			cnt := 0
			for _, m := range mList {
				hasRole := false
				for _, r := range m.Roles {
					if r == rid {
						hasRole = true
						break
					}
				}
				if !hasRole {
					err := ctx.Session.GuildMemberRoleAdd(gid, m.User.ID, rid)
					if err == nil {
						cnt++
					}
				}
			}
			return ctx.Reply(fmt.Sprintf("[+] Successfully added role to %d members.", cnt))

		default:
			return ctx.SendHelp("role")
		}
	},
}

func resolveRole(s *discordgo.Session, gid, query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}

	if m := rxRole.FindStringSubmatch(q); len(m) > 1 {
		return m[1]
	}

	roles, err := s.GuildRoles(gid)
	if err != nil {
		return ""
	}

	for _, r := range roles {
		if r.ID == q {
			return r.ID
		}
	}

	ql := strings.ToLower(q)
	for _, r := range roles {
		if strings.ToLower(r.Name) == ql {
			return r.ID
		}
	}

	for _, r := range roles {
		if strings.Contains(strings.ToLower(r.Name), ql) {
			return r.ID
		}
	}

	return ""
}
