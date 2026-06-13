package general

import (
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math/rand"
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

func init() {
	manager.RegisterHelp("boosterrole", []manager.HelpPage{
		{
			Command:     "BoosterRole Create",
			Syntax:      ".boosterrole create <hex> <name>",
			Description: "Create a custom color booster role.",
		},
		{
			Command:     "BoosterRole Rename",
			Syntax:      ".boosterrole rename <name>",
			Description: "Change the name of your booster role.",
		},
		{
			Command:     "BoosterRole Color",
			Syntax:      ".boosterrole color <hex>",
			Description: "Change the color of your booster role.",
		},
		{
			Command:     "BoosterRole Share",
			Syntax:      ".boosterrole share <@member>",
			Description: "Share (or stop sharing) your booster role with a member.",
		},
		{
			Command:     "BoosterRole Dominant",
			Syntax:      ".boosterrole dominant",
			Description: "Set your booster role color to the dominant color of your avatar.",
		},
		{
			Command:     "BoosterRole Random",
			Syntax:      ".boosterrole random",
			Description: "Set your booster role to a random color.",
		},
		{
			Command:     "BoosterRole Remove",
			Syntax:      ".boosterrole remove",
			Description: "Delete your booster role.",
		},
	})
}

var BoosterRole = &manager.Command{
	Trigger:     "boosterrole",
	Aliases:     []string{"br", "boostrole"},
	Name:        "boosterrole",
	Description: "Booster Role management system",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage:\n" +
				"`.br base <@role>`\n" +
				"`.br award <@user> <@role>`\n" +
				"`.br create <hex> <name>`\n" +
				"`.br rename <name>`\n" +
				"`.br color <hex>`\n" +
				"`.br share <@user>`\n" +
				"`.br dominant`\n" +
				"`.br random`\n" +
				"`.br filter <word>`\n" +
				"`.br remove`")
		}

		gid := ctx.GuildID()
		sub := strings.ToLower(ctx.Args[0])

		isAdmin := checkPerm(ctx, discordgo.PermissionManageRoles)

		switch sub {
		case "base":
			if !isAdmin {
				return ctx.Reply("[!] You need Manage Roles permission.")
			}
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.br base <@role/ID>`")
			}
			roleArg := ctx.Args[1]
			rid := getCleanRoleID(roleArg)

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
			if mem.PremiumSince == nil && !checkPerm(ctx, discordgo.PermissionAdministrator) {
				return ctx.Reply("[!] You must be boosting this server to create a booster role.")
			}

			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err == nil && existing != "" {
				return ctx.Reply(fmt.Sprintf("[!] You already have a booster role: <@&%s>", existing))
			}

			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.br create <hex> <name>`")
			}
			hexStr := strings.TrimPrefix(ctx.Args[1], "#")
			colVal, err := strconv.ParseInt(hexStr, 16, 32)
			if err != nil {
				return ctx.Reply("[!] Invalid hex color code.")
			}
			colorInt := int(colVal)

			name := "Booster Role"
			if len(ctx.Args) >= 3 {
				name = strings.Join(ctx.Args[2:], " ")
			}

			filters, _ := ctx.DB.ListBoosterFilters(gid)
			lowerName := strings.ToLower(name)
			for _, f := range filters {
				if strings.Contains(lowerName, f) {
					return ctx.Reply("[!] Role name contains blocked word.")
				}
			}


			newRole, err := ctx.Session.GuildRoleCreate(gid, &discordgo.RoleParams{
				Name:  name,
				Color: &colorInt,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to create role: %v", err))
			}

			_ = ctx.Session.GuildMemberRoleAdd(gid, ctx.AuthorID(), newRole.ID)
			_ = ctx.DB.SaveUserBoosterRole(gid, ctx.AuthorID(), newRole.ID)

			if baseID, err := ctx.DB.GetBoosterBase(gid); err == nil && baseID != "" {
				_ = positionRoleBelow(ctx.Session, gid, newRole.ID, baseID)
			}

			return ctx.Reply(fmt.Sprintf("[+] Created and assigned booster role <@&%s>.", newRole.ID))

		case "rename":
			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err != nil || existing == "" {
				return ctx.Reply("[!] You do not have a custom booster role.")
			}

			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.br rename <name>`")
			}
			name := strings.Join(ctx.Args[1:], " ")

			filters, _ := ctx.DB.ListBoosterFilters(gid)
			lowerName := strings.ToLower(name)
			for _, f := range filters {
				if strings.Contains(lowerName, f) {
					return ctx.Reply("[!] Role name contains blocked word.")
				}
			}

			_, err = ctx.Session.GuildRoleEdit(gid, existing, &discordgo.RoleParams{
				Name: name,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to rename role: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Renamed booster role to `%s`.", name))

		case "color", "colour":
			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err != nil || existing == "" {
				return ctx.Reply("[!] You do not have a custom booster role.")
			}

			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.br color <hex>`")
			}
			hexStr := strings.TrimPrefix(ctx.Args[1], "#")
			colVal, err := strconv.ParseInt(hexStr, 16, 32)
			if err != nil {
				return ctx.Reply("[!] Invalid hex color.")
			}
			colorInt := int(colVal)

			_, err = ctx.Session.GuildRoleEdit(gid, existing, &discordgo.RoleParams{
				Color: &colorInt,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to update role color: %v", err))
			}
			return ctx.Reply(fmt.Sprintf("[+] Updated booster role color to `#%s`.", hexStr))

		case "share":
			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err != nil || existing == "" {
				return ctx.Reply("[!] You do not have a booster role to share.")
			}

			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.br share <@member>` or `.br share list/max/limit`")
			}

			subAction := strings.ToLower(ctx.Args[1])

			switch subAction {
			case "max", "limit":
				if !isAdmin {
					return ctx.Reply("[!] Manage Roles permission required.")
				}
				if len(ctx.Args) < 3 {
					return ctx.Reply("Usage: `.br share max <number>`")
				}
				limitVal, err := strconv.Atoi(ctx.Args[2])
				if err != nil || limitVal <= 0 {
					return ctx.Reply("[!] Invalid limit number.")
				}
				gCfg, _ := ctx.DB.GetGuildSettings(gid)
				gCfg.MaxShares = limitVal
				_ = ctx.DB.SaveGuildSettings(gid, gCfg)
				return ctx.Reply(fmt.Sprintf("[+] Maximum booster role shares configured to %d.", limitVal))

			case "list":
				// If admin, list all sharing in the server. If user, list user's shares.
				if isAdmin {
					shares, _ := ctx.DB.ListAllBoosterShares(gid)
					if len(shares) == 0 {
						return ctx.Reply("[*] No custom booster roles are currently shared.")
					}
					var sb strings.Builder
					sb.WriteString("Shared Booster Roles:\n\n")
					for k, roleID := range shares {
						parts := strings.Split(k, ":")
						if len(parts) >= 3 {
							sb.WriteString(fmt.Sprintf("- Owner <@%s> shares <@&%s> with <@%s>\n", parts[1], roleID, parts[2]))
						}
					}
					return ctx.Reply(sb.String())
				}
				shares, _ := ctx.DB.ListBoosterSharesForOwner(gid, ctx.AuthorID())
				if len(shares) == 0 {
					return ctx.Reply("[*] You are not sharing your booster role with anyone.")
				}
				var sb strings.Builder
				sb.WriteString("Shared Users:\n\n")
				for targetID := range shares {
					sb.WriteString(fmt.Sprintf("- <@%s>\n", targetID))
				}
				return ctx.Reply(sb.String())

			case "remove":
				if len(ctx.Args) < 3 {
					return ctx.Reply("Usage: `.br share remove <@member>`")
				}
				targetUser := ctx.Args[2]
				targetID := getCleanUserID(targetUser)

				_ = ctx.DB.DeleteBoosterShare(gid, ctx.AuthorID(), targetID)
				_ = ctx.Session.GuildMemberRoleRemove(gid, targetID, existing)
				return ctx.Reply(fmt.Sprintf("[+] Stopped sharing role with <@%s>.", targetID))

			default:
				targetID := getCleanUserID(ctx.Args[1])
				targetMember, err := ctx.Session.GuildMember(gid, targetID)
				if err != nil {
					return ctx.Reply("[!] Could not find that member.")
				}

				shares, _ := ctx.DB.ListBoosterSharesForOwner(gid, ctx.AuthorID())
				if _, alreadyShared := shares[targetID]; alreadyShared {
					_ = ctx.DB.DeleteBoosterShare(gid, ctx.AuthorID(), targetID)
					_ = ctx.Session.GuildMemberRoleRemove(gid, targetID, existing)
					return ctx.Reply(fmt.Sprintf("[+] Removed custom role from **%s**.", targetMember.User.Username))
				}

				gCfg, _ := ctx.DB.GetGuildSettings(gid)
				if len(shares) >= gCfg.MaxShares {
					return ctx.Reply(fmt.Sprintf("[!] Share limit reached (max: %d).", gCfg.MaxShares))
				}

				_ = ctx.DB.SaveBoosterShare(gid, ctx.AuthorID(), targetID, existing)
				_ = ctx.Session.GuildMemberRoleAdd(gid, targetID, existing)
				return ctx.Reply(fmt.Sprintf("[+] Shared custom role with **%s**.", targetMember.User.Username))
			}

		case "dominant":
			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err != nil || existing == "" {
				return ctx.Reply("[!] You do not have a custom booster role.")
			}

			author, err := ctx.Session.GuildMember(gid, ctx.AuthorID())
			if err != nil {
				return ctx.Reply("[!] Failed to get your member profile.")
			}

			avatarURL := author.User.AvatarURL("128")
			if avatarURL == "" {
				return ctx.Reply("[!] You do not have an avatar to extract color from.")
			}

			colorInt, hex, err := sampleDominantColor(avatarURL)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Dominant color extraction failed: %v", err))
			}

			_, err = ctx.Session.GuildRoleEdit(gid, existing, &discordgo.RoleParams{
				Color: &colorInt,
			})
			if err != nil {
				return ctx.Reply("[!] Failed to update booster role color.")
			}

			return ctx.Reply(fmt.Sprintf("[+] Booster role color set to avatar dominant color: `#%s`.", hex))

		case "random":
			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err != nil || existing == "" {
				return ctx.Reply("[!] You do not have a custom booster role.")
			}

			colVal := rand.Intn(16777216)
			_, err = ctx.Session.GuildRoleEdit(gid, existing, &discordgo.RoleParams{
				Color: &colVal,
			})
			if err != nil {
				return ctx.Reply("[!] Failed to set role color.")
			}
			return ctx.Reply(fmt.Sprintf("[+] Random booster role color set: `#%06x`.", colVal))

		case "filter":
			if !isAdmin {
				return ctx.Reply("[!] You need Manage Roles permission.")
			}
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.br filter <word>` or `.br filter list`")
			}
			filterSub := strings.ToLower(ctx.Args[1])
			if filterSub == "list" {
				filters, _ := ctx.DB.ListBoosterFilters(gid)
				if len(filters) == 0 {
					return ctx.Reply("[*] No custom booster role filters set.")
				}
				return ctx.Reply(fmt.Sprintf("Filtered words:\n`%s`", strings.Join(filters, "`, `")))
			}
			word := strings.Join(ctx.Args[1:], " ")
			_ = ctx.DB.SaveBoosterFilter(gid, word)
			return ctx.Reply(fmt.Sprintf("[+] Word `%s` added to name filters.", word))

		case "link":
			if !isAdmin {
				return ctx.Reply("[!] You need Manage Roles permission.")
			}
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.br link <@member> <@role>`")
			}
			targetID := getCleanUserID(ctx.Args[1])
			roleID := getCleanRoleID(ctx.Args[2])

			_ = ctx.DB.SaveUserBoosterRole(gid, targetID, roleID)
			_ = ctx.Session.GuildMemberRoleAdd(gid, targetID, roleID)
			return ctx.Reply(fmt.Sprintf("[+] Linked booster role <@&%s> to <@%s>.", roleID, targetID))

		case "limit":
			if !isAdmin {
				return ctx.Reply("[!] You need Manage Roles permission.")
			}
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.br limit <limit>`")
			}
			lim, err := strconv.Atoi(ctx.Args[1])
			if err != nil || lim <= 0 {
				return ctx.Reply("[!] Invalid limit number.")
			}
			gCfg, _ := ctx.DB.GetGuildSettings(gid)
			gCfg.BoosterLimit = lim
			_ = ctx.DB.SaveGuildSettings(gid, gCfg)
			return ctx.Reply(fmt.Sprintf("[+] Booster role limit set to %d.", lim))

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
				return ctx.Reply("Usage: `.br icon <emoji/URL>` (or upload an image)")
			}

			params := &discordgo.RoleParams{}
			if iconBase64 != "" {
				params.Icon = &iconBase64
			} else if unicodeEmoji != "" {
				params.UnicodeEmoji = &unicodeEmoji
			}

			_, err = ctx.Session.GuildRoleEdit(gid, existing, params)
			if err != nil {
				return ctx.Reply("[!] Failed to set role icon (check server boost tier).")
			}
			return ctx.Reply("[+] Booster role icon updated.")

		case "remove", "delete":
			existing, err := ctx.DB.GetUserBoosterRole(gid, ctx.AuthorID())
			if err != nil || existing == "" {
				return ctx.Reply("[!] You do not have a custom booster role.")
			}

			_ = ctx.Session.GuildRoleDelete(gid, existing)
			_ = ctx.DB.DeleteUserBoosterRole(gid, ctx.AuthorID())
			return ctx.Reply("[+] Booster role removed.")

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
			if !isAdmin {
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
			if !isAdmin {
				return ctx.Reply("[!] You need Manage Roles permission.")
			}
			if len(ctx.Args) < 3 {
				return ctx.Reply("Usage: `.br award <@user> <@role>`")
			}
			uid := getCleanUserID(ctx.Args[1])
			rid := getCleanRoleID(ctx.Args[2])

			_ = ctx.DB.SaveUserBoosterRole(gid, uid, rid)
			_ = ctx.Session.GuildMemberRoleAdd(gid, uid, rid)
			return ctx.Reply(fmt.Sprintf("[+] Associated booster role <@&%s> to <@%s>.", rid, uid))

		default:
			return ctx.Reply("Unknown subcommand. Use base, award, create, rename, color, share, dominant, random, filter, link, limit, icon, remove, list, cleanup.")
		}
		return nil
	},
}

func getCleanRoleID(arg string) string {
	if m := rxBoosterRole.FindStringSubmatch(arg); len(m) > 1 {
		return m[1]
	}
	return arg
}

func getCleanUserID(arg string) string {
	if m := rxBoosterUser.FindStringSubmatch(arg); len(m) > 1 {
		return m[1]
	}
	return arg
}

func sampleDominantColor(avatarURL string) (int, string, error) {
	resp, err := http.Get(avatarURL)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return 0, "", err
	}

	bounds := img.Bounds()
	counts := make(map[string]int)

	for i := 0; i < 500; i++ {
		x := rand.Intn(bounds.Max.X-bounds.Min.X) + bounds.Min.X
		y := rand.Intn(bounds.Max.Y-bounds.Min.Y) + bounds.Min.Y
		r, g, b, _ := img.At(x, y).RGBA()
		rVal := r >> 8
		gVal := g >> 8
		bVal := b >> 8

		rRounded := (rVal / 16) * 16
		gRounded := (gVal / 16) * 16
		bRounded := (bVal / 16) * 16

		hex := fmt.Sprintf("%02x%02x%02x", rRounded, gRounded, bRounded)
		counts[hex]++
	}

	maxHex := "ffffff"
	maxCount := -1
	for hex, count := range counts {
		if count > maxCount {
			maxCount = count
			maxHex = hex
		}
	}

	colVal, _ := strconv.ParseInt(maxHex, 16, 32)
	return int(colVal), maxHex, nil
}

func positionRoleBelow(s *discordgo.Session, guildID, roleID, baseRoleID string) error {
	roles, err := s.GuildRoles(guildID)
	if err != nil {
		return err
	}
	var baseRole *discordgo.Role
	for _, r := range roles {
		if r.ID == baseRoleID {
			baseRole = r
			break
		}
	}
	if baseRole == nil {
		return fmt.Errorf("base role not found")
	}

	_, err = s.GuildRoleReorder(guildID, []*discordgo.Role{
		{ID: roleID, Position: baseRole.Position},
	})
	return err
}

func downloadAndBase64(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/png"
	}

	b64 := base64.StdEncoding.EncodeToString(body)
	return fmt.Sprintf("data:%s;base64,%s", mime, b64), nil
}
