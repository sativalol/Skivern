package utility

import (
	"fmt"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("tempvoice", []manager.HelpPage{
		{
			Command:     "Tempvoice Channel",
			Syntax:      ".tempvoice channel <voice_channel_id>",
			Description: "Set 'Join to Create' parent voice channel.",
		},
		{
			Command:     "Tempvoice Category",
			Syntax:      ".tempvoice category <category_id>",
			Description: "Set category where temporary VCs are created.",
		},
		{
			Command:     "Tempvoice Enable",
			Syntax:      ".tempvoice enable",
			Description: "Enable temporary voice channel system.",
		},
		{
			Command:     "Tempvoice Disable",
			Syntax:      ".tempvoice disable",
			Description: "Disable temporary voice channel system.",
		},
	})
}

var TempVoice = &manager.Command{
	Trigger:     "tempvoice",
	Aliases:     []string{"tv", "tempvc"},
	Name:        "tempvoice",
	Description: "Configure temporary voice channels",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionManageChannels) == 0 {
			return ctx.Reply("[!] You need Manage Channels permission to configure temporary voice channels.")
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("tempvoice")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		cfg, err := ctx.DB.GetTempVoiceCfg(gid)
		if err != nil {
			cfg = storage.TempVoiceCfg{Enabled: false}
		}

		switch sub {
		case "channel", "chan":
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("tempvoice")
			}
			vcID := ctx.Args[1]

			ch, err := ctx.Session.Channel(vcID)
			if err != nil || ch.GuildID != gid || ch.Type != discordgo.ChannelTypeGuildVoice {
				return ctx.Reply("[!] Invalid voice channel ID.")
			}

			cfg.ParentChannelID = vcID
			if cfg.CategoryID == "" {
				cfg.CategoryID = ch.ParentID
			}

			_ = ctx.DB.SaveTempVoiceCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Set 'Join to Create' voice channel to <#%s>.", vcID))

		case "category", "cat":
			if len(ctx.Args) < 2 {
				return ctx.SendHelp("tempvoice")
			}
			catID := ctx.Args[1]

			ch, err := ctx.Session.Channel(catID)
			if err != nil || ch.GuildID != gid || ch.Type != discordgo.ChannelTypeGuildCategory {
				return ctx.Reply("[!] Invalid category channel ID.")
			}

			cfg.CategoryID = catID
			_ = ctx.DB.SaveTempVoiceCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Temp voice channels will be created in category `%s`.", ch.Name))

		case "enable":
			if cfg.ParentChannelID == "" {
				return ctx.Reply("[!] Configure a parent channel first using `.tempvoice channel <id>`.")
			}
			cfg.Enabled = true
			_ = ctx.DB.SaveTempVoiceCfg(gid, cfg)
			return ctx.Reply("[+] Temporary voice system enabled.")

		case "disable":
			cfg.Enabled = false
			_ = ctx.DB.SaveTempVoiceCfg(gid, cfg)
			return ctx.Reply("[+] Temporary voice system disabled.")

		default:
			return ctx.SendHelp("tempvoice")
		}
	},
}
