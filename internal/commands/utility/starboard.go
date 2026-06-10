package utility

import (
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("starboard", []manager.HelpPage{
		{
			Command:     "Starboard Channel",
			Syntax:      ".starboard channel <channel>",
			Description: "Configure the starboard log channel.",
		},
		{
			Command:     "Starboard Threshold",
			Syntax:      ".starboard threshold <count>",
			Description: "Set the reaction threshold for the starboard.",
		},
		{
			Command:     "Starboard Enable",
			Syntax:      ".starboard enable",
			Description: "Enable the starboard system.",
		},
		{
			Command:     "Starboard Disable",
			Syntax:      ".starboard disable",
			Description: "Disable the starboard system.",
		},
	})
}

var rxStarChan = regexp.MustCompile(`^<#(\d+)>$`)

var Starboard = &manager.Command{
	Trigger:     "starboard",
	Aliases:     []string{"sb", "star"},
	Name:        "starboard",
	Description: "Configure server starboard system",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionManageGuild) == 0 {
			return ctx.Reply("[!] You need Manage Guild permission to configure the starboard.")
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("starboard")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		cfg, err := ctx.DB.GetStarboardCfg(gid)
		if err != nil {
			cfg = storage.StarboardCfg{
				Threshold: 3,
				Enabled:   false,
			}
		}

		switch sub {
		case "channel", "chan":
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("starboard")
			}
			chanArg := ctx.Args[1]
			cid := ""
			if m := rxStarChan.FindStringSubmatch(chanArg); len(m) > 1 {
				cid = m[1]
			} else {
				cid = chanArg
			}

			ch, err := ctx.Session.Channel(cid)
			if err != nil || ch.GuildID != gid {
				return ctx.Reply("[!] Could not resolve text channel.")
			}

			cfg.ChannelID = cid
			_ = ctx.DB.SaveStarboardCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Starboard channel configured to <#%s>.", cid))

		case "threshold", "limit":
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("starboard")
			}
			count, err := strconv.Atoi(ctx.Args[1])
			if err != nil || count <= 0 {
				return ctx.Reply("[!] Invalid threshold count. Must be a positive number.")
			}

			cfg.Threshold = count
			_ = ctx.DB.SaveStarboardCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Starboard reaction threshold set to %d.", count))

		case "enable":
			if cfg.ChannelID == "" {
				return ctx.Reply("[!] Please configure a channel first using `.starboard channel <#channel>`.")
			}
			cfg.Enabled = true
			_ = ctx.DB.SaveStarboardCfg(gid, cfg)
			return ctx.Reply("[+] Starboard system enabled.")

		case "disable":
			cfg.Enabled = false
			_ = ctx.DB.SaveStarboardCfg(gid, cfg)
			return ctx.Reply("[+] Starboard system disabled.")

		default:
			return ctx.SendHelp("starboard")
		}
	},
}
