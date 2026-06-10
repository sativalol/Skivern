package general

import (
	"encoding/json"
	"fmt"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type BtnRoleJson struct {
	Embed struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Color       int    `json:"color"`
	} `json:"embed"`
	Buttons []struct {
		Label  string `json:"label"`
		Style  string `json:"style"`
		Emoji  string `json:"emoji"`
		RoleID string `json:"role_id"`
	} `json:"buttons"`
}

var ButtonRole = &manager.Command{
	Trigger:     "buttonrole",
	Aliases:     []string{"btnrole"},
	Name:        "buttonrole",
	Description: "Create a button role panel using JSON payload",
	Category:    "general",
	Execute: func(ctx *manager.CommandContext) error {
		if !checkPerm(ctx, discordgo.PermissionAdministrator) {
			return ctx.Reply("[!] You need Administrator permission to use this command.")
		}

		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: `.buttonrole create <json_payload>`")
		}

		sub := strings.ToLower(ctx.Args[0])
		if sub != "create" {
			return ctx.Reply("[!] Invalid subcommand. Use `.buttonrole create <json_payload>`")
		}

		jsonPayload := strings.Join(ctx.Args[1:], " ")
		var cfg BtnRoleJson
		if err := json.Unmarshal([]byte(jsonPayload), &cfg); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Invalid JSON format: %v", err))
		}

		gid := ctx.GuildID()

		botMember, err := ctx.Session.GuildMember(gid, ctx.ClientID)
		if err != nil {
			return ctx.Reply("[!] Failed to verify bot hierarchy status.")
		}

		roles, err := ctx.Session.GuildRoles(gid)
		if err != nil {
			return ctx.Reply("[!] Failed to fetch guild roles.")
		}

		botMaxPos := -1
		for _, r := range roles {
			for _, botRoleID := range botMember.Roles {
				if r.ID == botRoleID && r.Position > botMaxPos {
					botMaxPos = r.Position
				}
			}
		}

		var components []discordgo.MessageComponent
		var buttons []discordgo.MessageComponent

		for i, btn := range cfg.Buttons {
			var targetRole *discordgo.Role
			for _, r := range roles {
				if r.ID == btn.RoleID {
					targetRole = r
					break
				}
			}
			if targetRole == nil {
				return ctx.Reply(fmt.Sprintf("[!] Role ID `%s` not found in server.", btn.RoleID))
			}

			if targetRole.Position >= botMaxPos {
				return ctx.Reply(fmt.Sprintf("[!] Security Alert: Role <@&%s> is higher than or equal to the bot's own role. Action blocked.", btn.RoleID))
			}

			dangerousPerms := int64(discordgo.PermissionAdministrator |
				discordgo.PermissionManageRoles |
				discordgo.PermissionManageGuild |
				discordgo.PermissionBanMembers |
				discordgo.PermissionKickMembers |
				discordgo.PermissionManageWebhooks |
				discordgo.PermissionManageChannels)
			if (targetRole.Permissions & dangerousPerms) != 0 {
				return ctx.Reply(fmt.Sprintf("[!] Security Alert: Role <@&%s> has administrative/moderation permissions. Cannot be used for public button roles.", btn.RoleID))
			}

			style := discordgo.PrimaryButton
			switch strings.ToLower(btn.Style) {
			case "secondary", "grey":
				style = discordgo.SecondaryButton
			case "success", "green":
				style = discordgo.SuccessButton
			case "danger", "red":
				style = discordgo.DangerButton
			case "primary", "blue":
				style = discordgo.PrimaryButton
			}

			customID := fmt.Sprintf("btnrole_%s_%d", btn.RoleID, i)

			btnObj := discordgo.Button{
				Label:    btn.Label,
				Style:    style,
				CustomID: customID,
			}
			if btn.Emoji != "" {
				btnObj.Emoji = &discordgo.ComponentEmoji{
					Name: btn.Emoji,
				}
			}

			buttons = append(buttons, btnObj)
		}

		if len(buttons) == 0 {
			return ctx.Reply("[!] No buttons specified.")
		}

		components = append(components, discordgo.ActionsRow{
			Components: buttons,
		})

		embed := &discordgo.MessageEmbed{
			Title:       cfg.Embed.Title,
			Description: cfg.Embed.Description,
			Color:       cfg.Embed.Color,
		}
		if embed.Color == 0 {
			embed.Color = 0x7289da
		}

		sentMsg, err := ctx.Session.ChannelMessageSendComplex(ctx.ChanID(), &discordgo.MessageSend{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		})
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to send button role panel: %v", err))
		}

		for i, btn := range cfg.Buttons {
			customID := fmt.Sprintf("btnrole_%s_%d", btn.RoleID, i)
			_ = ctx.DB.SaveButtonRole(gid, sentMsg.ID, customID, btn.RoleID)
		}

		return ctx.Reply("[+] Button role panel created successfully.")
	},
}
