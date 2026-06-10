package general

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"skyvern/internal/manager"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxBoosterRole = regexp.MustCompile(`^<@&(\d+)>$`)
var rxBoosterUser = regexp.MustCompile(`^<@!?(\d+)>$`)
var rxCustomEmoji = regexp.MustCompile(`^<a?:[a-zA-Z0-9_]+:(\d+)>$`)

var BoosterRole = &manager.Command{
	Trigger:     "boosterrole",
	Aliases:     []string{"br"},
	Name:        "boosterrole",
	Description: "Booster Role management system",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage:\n" +
				"`.boosterrole base <@role>`\n" +
				"`.boosterrole create <name>`\n" +
				"`.boosterrole rename <name>`\n" +
				"`.boosterrole icon <emoji/URL>`\n" +
				"`.boosterrole remove`\n" +
				"`.boosterrole list`\n" +
				"`.boosterrole cleanup`\n" +
				"`.boosterrole award <@user> <@role>`")
		}

		gid := ctx.GuildID()
		sub := strings.ToLower(ctx.Args[0])

		switch sub {
		case "base":
			if !checkPerm(ctx, discordgo.PermissionManageRoles) {
				return ctx.Reply("[!] You need Manage Roles permission.")
			}
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.boosterrole base <@role/ID>`")
			}
			roleArg := ctx.Args[1]
			rid := ""
			if m := rxBoosterRole.FindStringSubmatch(roleArg); len(m) > 1 {
				rid = m[1]
			} else {
				rid = roleArg
			}

			roles, err := ctx.Session.GuildRoles(gid)
			if err != nil {
				return ctx.Reply("[!] Failed to fetch guild roles.")
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

			_ = ctx.DB.SaveBoosterBase(gid, rid)
			return ctx.Reply(fmt.Sprintf("[+] Set base booster role to <@&%s>.", rid))

		case "create":
			mem, err := ctx.Session.GuildMember(gid, ctx.AuthorID())
			if err != nil {
				return ctx.Reply("[!] Failed to check your boosting status.")
			}
			if mem.PremiumSince == nil {
				return ctx.Reply("[!] You must be boosting this server to create a booster role.")
			}

			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err == nil && existing != "" {
				return ctx.Reply(fmt.Sprintf("[!] You already have a booster role: <@&%s>", existing))
			}

			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.boosterrole create <name>`")
			}
			name := strings.Join(ctx.Args[1:], " ")

			newRole, err := ctx.Session.GuildRoleCreate(gid, &discordgo.RoleParams{
				Name: name,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to create role: %v", err))
			}

			err = ctx.Session.GuildMemberRoleAdd(gid, ctx.AuthorID(), newRole.ID)
			if err != nil {
				_ = ctx.Session.GuildRoleDelete(gid, newRole.ID)
				return ctx.Reply(fmt.Sprintf("[!] Failed to assign role: %v", err))
			}

			_ = ctx.DB.SaveUserBoosterRole(gid, ctx.AuthorID(), newRole.ID)

			if baseID, err := ctx.DB.GetBoosterBase(gid); err == nil && baseID != "" {
				_ = positionRoleBelow(ctx.Session, gid, newRole.ID, baseID)
			}

			return ctx.Reply(fmt.Sprintf("[+] Created and assigned booster role <@&%s>.", newRole.ID))

		case "rename":
			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err != nil || existing == "" {
				return ctx.Reply("[!] You do not have a custom booster role. Create one first.")
			}

			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.boosterrole rename <name>`")
			}
			name := strings.Join(ctx.Args[1:], " ")

			_, err = ctx.Session.GuildRoleEdit(gid, existing, &discordgo.RoleParams{
				Name: name,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to rename role: %v", err))
			}

			return ctx.Reply(fmt.Sprintf("[+] Renamed your booster role to `%s`.", name))

		case "color", "colour":
			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err != nil || existing == "" {
				return ctx.Reply("[!] You do not have a custom booster role. Create one first.")
			}

			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.boosterrole color <hex>`")
			}
			hexStr := strings.TrimPrefix(ctx.Args[1], "#")
			colVal, err := strconv.ParseInt(hexStr, 16, 32)
			if err != nil {
				return ctx.Reply("[!] Invalid hex color code. Example: `#ff0000` or `ff0000`.")
			}
			colorInt := int(colVal)

			_, err = ctx.Session.GuildRoleEdit(gid, existing, &discordgo.RoleParams{
				Color: &colorInt,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to update role color: %v", err))
			}

			return ctx.Reply(fmt.Sprintf("[+] Updated your booster role color to `#%s`.", hexStr))

		case "icon":
			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err != nil || existing == "" {
				return ctx.Reply("[!] You do not have a custom booster role.")
			}

			var iconBase64 string
			var unicodeEmoji string

			if len(ctx.Message.Attachments) > 0 {
				url := ctx.Message.Attachments[0].URL
				b64, err := downloadAndBase64(url)
				if err != nil {
					return ctx.Reply(fmt.Sprintf("[!] Failed to process attachment: %v", err))
				}
				iconBase64 = b64
			} else if len(ctx.Args) >= 2 {
				arg := ctx.Args[1]
				if m := rxCustomEmoji.FindStringSubmatch(arg); len(m) > 1 {
					emojiURL := fmt.Sprintf("https://cdn.discordapp.com/emojis/%s.png", m[1])
					b64, err := downloadAndBase64(emojiURL)
					if err != nil {
						return ctx.Reply(fmt.Sprintf("[!] Failed to process custom emoji: %v", err))
					}
					iconBase64 = b64
				} else if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
					b64, err := downloadAndBase64(arg)
					if err != nil {
						return ctx.Reply(fmt.Sprintf("[!] Failed to process image URL: %v", err))
					}
					iconBase64 = b64
				} else {
					unicodeEmoji = arg
				}
			} else {
				return ctx.Reply("Usage: `.boosterrole icon <emoji/URL>` (or upload an image with the command)")
			}

			params := &discordgo.RoleParams{}
			if iconBase64 != "" {
				params.Icon = &iconBase64
			} else if unicodeEmoji != "" {
				params.UnicodeEmoji = &unicodeEmoji
			}

			_, err = ctx.Session.GuildRoleEdit(gid, existing, params)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to set role icon (Server may lack Boost Level 2 for role icons): %v", err))
			}

			return ctx.Reply("[+] Booster role icon updated successfully.")

		case "remove":
			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err != nil || existing == "" {
				return ctx.Reply("[!] You do not have a custom booster role.")
			}

			err = ctx.Session.GuildRoleDelete(gid, existing)
			if err != nil {
				_ = ctx.DB.DeleteUserBoosterRole(gid, ctx.AuthorID())
				return ctx.Reply("[!] Role was already deleted or could not be found. Entry removed from database.")
			}

			_ = ctx.DB.DeleteUserBoosterRole(gid, ctx.AuthorID())
			return ctx.Reply("[+] Custom booster role removed.")

		case "list":
			m, _ := ctx.DB.ListUserBoosterRoles(gid)
			if len(m) == 0 {
				return ctx.Reply("[*] No custom booster roles registered.")
			}
			var sb strings.Builder
			sb.WriteString("Custom Booster Roles:\n\n")
			for uid, rid := range m {
				sb.WriteString(fmt.Sprintf("- <@%s>: <@&%s> (`%s`)\n", uid, rid, rid))
			}
			return ctx.Reply(sb.String())

		case "cleanup":
			if !checkPerm(ctx, discordgo.PermissionManageRoles) {
				return ctx.Reply("[!] You need Manage Roles permission.")
			}
			m, _ := ctx.DB.ListUserBoosterRoles(gid)
			if len(m) == 0 {
				return ctx.Reply("[*] No custom booster roles to clean up.")
			}

			cleaned := 0
			for uid, rid := range m {
				mem, err := ctx.Session.GuildMember(gid, uid)
				if err != nil || mem.PremiumSince == nil {
					_ = ctx.Session.GuildRoleDelete(gid, rid)
					_ = ctx.DB.DeleteUserBoosterRole(gid, uid)
					cleaned++
				}
			}

			return ctx.Reply(fmt.Sprintf("[+] Cleaned up %d booster roles.", cleaned))

		case "award":
			if !checkPerm(ctx, discordgo.PermissionManageRoles) {
				return ctx.Reply("[!] You need Manage Roles permission.")
			}
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.boosterrole award <@user> <@role>`")
			}
			userArg := ctx.Args[1]
			roleArg := ctx.Args[2]

			uid := ""
			if m := rxBoosterUser.FindStringSubmatch(userArg); len(m) > 1 {
				uid = m[1]
			} else {
				uid = userArg
			}

			rid := ""
			if m := rxBoosterRole.FindStringSubmatch(roleArg); len(m) > 1 {
				rid = m[1]
			} else {
				rid = roleArg
			}

			_, err := ctx.Session.GuildMember(gid, uid)
			if err != nil {
				return ctx.Reply("[!] Member not found in server.")
			}

			roles, err := ctx.Session.GuildRoles(gid)
			if err != nil {
				return ctx.Reply("[!] Failed to fetch guild roles.")
			}
			found := false
			for _, r := range roles {
				if r.ID == rid {
					found = true
					break
				}
			}
			if !found {
				return ctx.Reply("[!] Role not found in server.")
			}

			_ = ctx.DB.SaveUserBoosterRole(gid, uid, rid)
			return ctx.Reply(fmt.Sprintf("[+] Associated booster role <@&%s> to <@%s>.", rid, uid))

		default:
			return ctx.Reply("Unknown subcommand. Use base, award, create, rename, icon, remove, list, cleanup.")
		}
		return nil
	},
}

func positionRoleBelow(s *discordgo.Session, gid, roleID, baseRoleID string) error {
	roles, err := s.GuildRoles(gid)
	if err != nil {
		return err
	}
	var basePos int
	var roleToMove *discordgo.Role
	for _, r := range roles {
		if r.ID == baseRoleID {
			basePos = r.Position
		}
		if r.ID == roleID {
			roleToMove = r
		}
	}
	if roleToMove == nil || basePos <= 1 {
		return nil
	}
	roleToMove.Position = basePos - 1
	_, err = s.GuildRoleReorder(gid, []*discordgo.Role{roleToMove})
	return err
}

func downloadAndBase64(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	contentType := http.DetectContentType(data)
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", contentType, encoded), nil
}
