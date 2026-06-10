package general

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxBday = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})(?:/\d{2,4})?$`)
var rxBdayChan = regexp.MustCompile(`^<#(\d+)>$`)

var Birthday = &manager.Command{
	Trigger:     "birthday",
	Aliases:     []string{"bday"},
	Name:        "birthday",
	Description: "View or set your birthday, or configure the announcements channel",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		gid := ctx.GuildID()
		uid := ctx.AuthorID()

		if len(ctx.Args) == 0 {
			bday, err := ctx.DB.GetBirthday(gid, uid)
			if err != nil || bday == "" {
				return ctx.Reply("[*] You haven't set your birthday yet. Use `.birthday set MM/DD/YYYY` to set it.")
			}
			return ctx.Reply(fmt.Sprintf("[*] Your birthday is set to `%s`.", bday))
		}

		sub := strings.ToLower(ctx.Args[0])

		switch sub {
		case "set":
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.birthday set MM/DD/YYYY` or `.birthday set MM/DD`")
			}
			dateArg := ctx.Args[1]
			m := rxBday.FindStringSubmatch(dateArg)
			if len(m) < 3 {
				return ctx.Reply("[!] Invalid birthday format. Please use MM/DD or MM/DD/YYYY (e.g. 05/20 or 12/31/1998).")
			}

			monthVal, _ := strconv.Atoi(m[1])
			dayVal, _ := strconv.Atoi(m[2])

			if monthVal < 1 || monthVal > 12 {
				return ctx.Reply("[!] Invalid month. Must be between 1 and 12.")
			}
			if dayVal < 1 || dayVal > 31 {
				return ctx.Reply("[!] Invalid day. Must be between 1 and 31.")
			}

			formatted := fmt.Sprintf("%02d/%02d", monthVal, dayVal)

			err := ctx.DB.SaveBirthday(gid, uid, formatted)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save birthday: %v", err))
			}

			return ctx.Reply(fmt.Sprintf("[+] Set your birthday to `%s`.", formatted))

		case "channel":
			if !checkPerm(ctx, discordgo.PermissionManageServer) {
				return ctx.Reply("[!] You need Manage Server permission to configure the birthday channel.")
			}
			if len(ctx.Args) < 2 {
				return ctx.Reply("Usage: `.birthday channel <#channel>`")
			}
			chanArg := ctx.Args[1]
			cid := ""
			if m := rxBdayChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			} else {
				cid = chanArg
			}

			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Could not resolve text channel.")
			}

			err = ctx.DB.SaveBirthdayChannel(gid, cid)
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to save birthday channel: %v", err))
			}

			return ctx.Reply(fmt.Sprintf("[+] Set birthday announcement channel to <#%s>.", cid))

		default:
			if m := rxBday.FindStringSubmatch(ctx.Args[0]); len(m) >= 3 {
				monthVal, _ := strconv.Atoi(m[1])
				dayVal, _ := strconv.Atoi(m[2])
				if monthVal >= 1 && monthVal <= 12 && dayVal >= 1 && dayVal <= 31 {
					formatted := fmt.Sprintf("%02d/%02d", monthVal, dayVal)
					_ = ctx.DB.SaveBirthday(gid, uid, formatted)
					return ctx.Reply(fmt.Sprintf("[+] Set your birthday to `%s`.", formatted))
				}
			}
			return ctx.Reply("Usage:\n" +
				"`.birthday` - View your birthday\n" +
				"`.birthday set MM/DD/YYYY` - Set your birthday\n" +
				"`.birthday channel <#channel>` - Configure announcement channel")
		}
	},
}
