package moderation

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"regexp"
	"skyvern/internal/manager"
	"skyvern/internal/moderation"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	rxRoleRole  = regexp.MustCompile(`^<@&(\d+)>$`)
	rxRoleUser  = regexp.MustCompile(`^<@!?(\d+)>$`)
	roleTasks   = make(map[string]chan struct{})
	roleTasksMu sync.Mutex
)

func init() {
	manager.RegisterHelp("role", []manager.HelpPage{
		{
			Command:     "Role Add/Remove",
			Syntax:      ".role [add|remove] <member> <role>",
			Description: "Add or remove a role from a member.",
		},
		{
			Command:     "Role Create",
			Syntax:      ".role create <color> <name>",
			Description: "Create a role with optional hex color.",
		},
		{
			Command:     "Role Delete",
			Syntax:      ".role delete <role>",
			Description: "Deletes a role.",
		},
		{
			Command:     "Role Edit",
			Syntax:      ".role edit <role> <new_name>",
			Description: "Rename a role.",
		},
		{
			Command:     "Role Hoist/Mentionable",
			Syntax:      ".role [hoist|mentionable] <role>",
			Description: "Toggle hoist status or mentionability of a role.",
		},
		{
			Command:     "Role Color",
			Syntax:      ".role color <hex> <role>",
			Description: "Set a solid color for a role.",
		},
		{
			Command:     "Role Color Gradient",
			Syntax:      ".role color gradient <hex1> <hex2> <role>",
			Description: "Set a gradient color for a role (via role icon).",
		},
		{
			Command:     "Role Icon",
			Syntax:      ".role icon <url> <role>",
			Description: "Set role icon image.",
		},
		{
			Command:     "Role Topcolor",
			Syntax:      ".role topcolor <hex> <member>",
			Description: "Changes member's highest role color.",
		},
		{
			Command:     "Role Restore",
			Syntax:      ".role restore <member>",
			Description: "Restore stripped roles to a jailed member.",
		},
		{
			Command:     "Role Humans/Bots",
			Syntax:      ".role [humans|bots] [remove] <role>",
			Description: "Bulk add/remove a role to all humans or bots.",
		},
		{
			Command:     "Role Has",
			Syntax:      ".role has [remove] <role> <target_role>",
			Description: "Bulk add/remove a role for everyone who has a specific role.",
		},
		{
			Command:     "Role Cancel",
			Syntax:      ".role cancel",
			Description: "Cancel running bulk role tasks.",
		},
	})
}

var Role = &manager.Command{
	Trigger:     "role",
	Aliases:     []string{"r"},
	Name:        "role",
	Description: "Manage server roles",
	Category:    "moderation",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionManageRoles) {
			return ctx.Reply("[!] You need Manage Roles permission.")
		}
		if len(ctx.Args) == 0 {
			return ctx.SendHelp("role")
		}

		gid := ctx.GuildID()
		sub := strings.ToLower(ctx.Args[0])

		if sub == "cancel" {
			roleTasksMu.Lock()
			ch, ok := roleTasks[gid]
			if ok {
				close(ch)
				delete(roleTasks, gid)
			}
			roleTasksMu.Unlock()
			if ok {
				return ctx.Reply("[+] Cancelled bulk role updates.")
			}
			return ctx.Reply("[!] No bulk role update task is currently running.")
		}

		switch sub {
		case "add":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.role add <member> <role>`")
			}
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
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.role remove <member> <role>`")
			}
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
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.role create <color> <name>`")
			}
			colStr := strings.TrimPrefix(ctx.Args[1], "#")
			colorVal, err := strconv.ParseInt(colStr, 16, 32)
			var colorInt int
			nameIdx := 2
			if err == nil {
				colorInt = int(colorVal)
			} else {
				colorInt = 0
				nameIdx = 1
			}
			name := strings.Join(ctx.Args[nameIdx:], " ")
			if name == "" {
				name = "New Role"
			}
			r, err := ctx.Session.GuildRoleCreate(gid, &discordgo.RoleParams{
				Name:  name,
				Color: &colorInt,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to create role: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Created role **%s** (`%s`).", r.Name, r.ID))

		case "delete":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.role delete <role>`")
			}
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
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.role edit <role> <new_name>`")
			}
			rid := resolveRole(ctx.Session, gid, ctx.Args[1])
			if rid == "" {
				return ctx.Reply("[!] Could not resolve role.")
			}
			name := strings.Join(ctx.Args[2:], " ")
			_, err := ctx.Session.GuildRoleEdit(gid, rid, &discordgo.RoleParams{Name: name})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to edit role name: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Renamed role to `%s`.", name))

		case "hoist":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.role hoist <role>`")
			}
			rid := resolveRole(ctx.Session, gid, ctx.Args[1])
			if rid == "" {
				return ctx.Reply("[!] Could not resolve role.")
			}
			r, err := ctx.Session.State.Role(gid, rid)
			if err != nil {
				return ctx.Reply("[!] Role not found.")
			}
			hoist := !r.Hoist
			_, err = ctx.Session.GuildRoleEdit(gid, rid, &discordgo.RoleParams{Hoist: &hoist})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to toggle hoist: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Set hoist status of **%s** to `%v`.", r.Name, hoist))

		case "mentionable":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.role mentionable <role>`")
			}
			rid := resolveRole(ctx.Session, gid, ctx.Args[1])
			if rid == "" {
				return ctx.Reply("[!] Could not resolve role.")
			}
			r, err := ctx.Session.State.Role(gid, rid)
			if err != nil {
				return ctx.Reply("[!] Role not found.")
			}
			ment := !r.Mentionable
			_, err = ctx.Session.GuildRoleEdit(gid, rid, &discordgo.RoleParams{Mentionable: &ment})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to toggle mentionable: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Set mentionable status of **%s** to `%v`.", r.Name, ment))

		case "color":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.role color <color> <role>` or `.role color gradient <color1> <color2> <role>`")
			}
			if strings.ToLower(ctx.Args[1]) == "gradient" {
				if len(ctx.Args) < 5 {
					return ctx.Reply("Usage: `.role color gradient <color1> <color2> <role>`")
				}
				c1 := ctx.Args[2]
				c2 := ctx.Args[3]
				rid := resolveRole(ctx.Session, gid, ctx.Args[4])
				if rid == "" {
					return ctx.Reply("[!] Could not resolve role.")
				}
				b64, err := generateGradientPNG(c1, c2)
				if err != nil {
					return ctx.Reply(fmt.Sprintf("[!] Gradient generation failed: %v", err))
				}
				_, err = ctx.Session.GuildRoleEdit(gid, rid, &discordgo.RoleParams{
					Icon: &b64,
				})
				if err != nil {
					return ctx.Reply(fmt.Sprintf("[!] Failed to set role icon gradient: %v", err))
				}
				return ctx.Reply("[+] Successfully set role gradient icon.")
			}

			colStr := strings.TrimPrefix(ctx.Args[1], "#")
			colorVal, err := strconv.ParseInt(colStr, 16, 32)
			if err != nil {
				return ctx.Reply("[!] Invalid hex color.")
			}
			colorInt := int(colorVal)
			rid := resolveRole(ctx.Session, gid, ctx.Args[2])
			if rid == "" {
				return ctx.Reply("[!] Could not resolve role.")
			}
			_, err = ctx.Session.GuildRoleEdit(gid, rid, &discordgo.RoleParams{Color: &colorInt})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to set color: %v", err))
			}
			return ctx.Reply("[+] Successfully set role color.")

		case "icon":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.role icon <url> <role>`")
			}
			url := ctx.Args[1]
			rid := resolveRole(ctx.Session, gid, ctx.Args[2])
			if rid == "" {
				return ctx.Reply("[!] Could not resolve role.")
			}
			resp, err := http.Get(url)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to fetch icon URL: %v", err))
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return ctx.Reply("[!] Failed to read image body.")
			}
			mime := resp.Header.Get("Content-Type")
			if mime == "" {
				mime = "image/png"
			}
			b64 := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(body))
			_, err = ctx.Session.GuildRoleEdit(gid, rid, &discordgo.RoleParams{Icon: &b64})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to set role icon (check boost tier): %v", err))
			}
			return ctx.Reply("[+] Successfully updated role icon.")

		case "topcolor":
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.role topcolor <color> <member>`")
			}
			colStr := strings.TrimPrefix(ctx.Args[1], "#")
			colorVal, err := strconv.ParseInt(colStr, 16, 32)
			if err != nil {
				return ctx.Reply("[!] Invalid hex color.")
			}
			colorInt := int(colorVal)
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[2])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			// Find highest role that is editable
			roles, err := ctx.Session.GuildRoles(gid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to fetch guild roles: %v", err))
			}
			roleMap := make(map[string]*discordgo.Role)
			for _, r := range roles {
				roleMap[r.ID] = r
			}
			var highestRole *discordgo.Role
			for _, rid := range m.Roles {
				r, ok := roleMap[rid]
				if !ok {
					continue
				}
				if highestRole == nil || r.Position > highestRole.Position {
					highestRole = r
				}
			}
			if highestRole == nil {
				return ctx.Reply("[!] Member has no roles.")
			}
			_, err = ctx.Session.GuildRoleEdit(gid, highestRole.ID, &discordgo.RoleParams{Color: &colorInt})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to change highest role color: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Updated highest role **%s** color.", highestRole.Name))

		case "restore":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.role restore <member>`")
			}
			m, err := moderation.ResolveMember(ctx.Session, gid, ctx.Args[1])
			if err != nil || m == nil {
				return ctx.Reply("[!] Could not resolve member.")
			}
			oldRoles, err := ctx.DB.GetJailed(gid, m.User.ID)
			if err != nil || len(oldRoles) == 0 {
				return ctx.Reply("[!] No stripped role records found for this member.")
			}
			restored := 0
			for _, rid := range oldRoles {
				if err := ctx.Session.GuildMemberRoleAdd(gid, m.User.ID, rid); err == nil {
					restored++
				}
			}
			_ = ctx.DB.DeleteJailed(gid, m.User.ID)
			return ctx.Reply(fmt.Sprintf("[+] Restored %d roles to **%s**.", restored, m.User.Username))

		case "humans", "bots", "has":
			var targetRoles []string
			var targetMembers []*discordgo.Member
			var roleToAdd string
			var isRemove = false

			argIdx := 1
			if sub == "has" {
				if len(ctx.Args) < 3 {
					return ctx.Reply("Usage: `.role has [remove] <role> <target_role>`")
				}
				if strings.ToLower(ctx.Args[1]) == "remove" {
					isRemove = true
					argIdx = 2
				}
				sourceRole := resolveRole(ctx.Session, gid, ctx.Args[argIdx])
				targetRoles = append(targetRoles, resolveRole(ctx.Session, gid, ctx.Args[argIdx+1]))
				if sourceRole == "" || targetRoles[0] == "" {
					return ctx.Reply("[!] Could not resolve roles.")
				}
				roleToAdd = sourceRole
			} else {
				if len(ctx.Args) < 2 {
					return ctx.Reply("Usage: `.role [humans|bots] [remove] <role>`")
				}
				if strings.ToLower(ctx.Args[1]) == "remove" {
					isRemove = true
					argIdx = 2
				}
				rid := resolveRole(ctx.Session, gid, ctx.Args[argIdx])
				if rid == "" {
					return ctx.Reply("[!] Could not resolve role.")
				}
				roleToAdd = rid
			}

			roleTasksMu.Lock()
			if _, exists := roleTasks[gid]; exists {
				roleTasksMu.Unlock()
				return ctx.Reply("[!] A bulk role update task is already running in this server. Cancel it first with `.role cancel`.")
			}
			cancelCh := make(chan struct{})
			roleTasks[gid] = cancelCh
			roleTasksMu.Unlock()

			_ = ctx.Reply("[*] Fetching member list and starting bulk update. This might take a while...")

			go func() {
				defer func() {
					roleTasksMu.Lock()
					delete(roleTasks, gid)
					roleTasksMu.Unlock()
				}()

				mList, err := ctx.Session.GuildMembers(gid, "", 1000)
				if err != nil {
					_, _ = ctx.Session.ChannelMessageSend(ctx.ChanID(), fmt.Sprintf("[!] Failed to fetch members: %v", err))
					return
				}

				for _, m := range mList {
					if sub == "humans" && m.User.Bot {
						continue
					}
					if sub == "bots" && !m.User.Bot {
						continue
					}
					if sub == "has" {
						hasTarget := false
						for _, r := range m.Roles {
							if r == targetRoles[0] {
								hasTarget = true
								break
							}
						}
						if !hasTarget {
							continue
						}
					}
					targetMembers = append(targetMembers, m)
				}

				modified := 0
				for _, m := range targetMembers {
					select {
					case <-cancelCh:
						_, _ = ctx.Session.ChannelMessageSend(ctx.ChanID(), fmt.Sprintf("[-] Bulk role update cancelled. Updated %d members.", modified))
						return
					default:
					}

					hasRole := false
					for _, r := range m.Roles {
						if r == roleToAdd {
							hasRole = true
							break
						}
					}

					var err error
					if isRemove && hasRole {
						err = ctx.Session.GuildMemberRoleRemove(gid, m.User.ID, roleToAdd)
					} else if !isRemove && !hasRole {
						err = ctx.Session.GuildMemberRoleAdd(gid, m.User.ID, roleToAdd)
					}

					if err == nil {
						modified++
					}
					time.Sleep(150 * time.Millisecond)
				}

				actionVerb := "added"
				if isRemove {
					actionVerb = "removed"
				}
				_, _ = ctx.Session.ChannelMessageSend(ctx.ChanID(), fmt.Sprintf("[+] Completed bulk role update. Successfully %s role to %d member(s).", actionVerb, modified))
			}()
		}

		return nil
	},
}

func resolveRole(s *discordgo.Session, gid, query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}
	if m := rxRoleRole.FindStringSubmatch(q); len(m) > 1 {
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

func generateGradientPNG(c1, c2 string) (string, error) {
	c1 = strings.TrimPrefix(c1, "#")
	c2 = strings.TrimPrefix(c2, "#")
	val1, err := strconv.ParseInt(c1, 16, 32)
	if err != nil {
		return "", err
	}
	val2, err := strconv.ParseInt(c2, 16, 32)
	if err != nil {
		return "", err
	}
	r1, g1, b1 := uint8(val1>>16), uint8((val1>>8)&0xff), uint8(val1&0xff)
	r2, g2, b2 := uint8(val2>>16), uint8((val2>>8)&0xff), uint8(val2&0xff)

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		ratio := float64(y) / 63.0
		r := uint8(float64(r1)*(1.0-ratio) + float64(r2)*ratio)
		g := uint8(float64(g1)*(1.0-ratio) + float64(g2)*ratio)
		b := uint8(float64(b1)*(1.0-ratio) + float64(b2)*ratio)
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes())), nil
}
