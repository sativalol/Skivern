package utility

import (
	"encoding/json"
	"fmt"
	"regexp"
	"skyvern/internal/manager"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxEmbedChan = regexp.MustCompile(`^<#(\d+)>$`)

type EmbedJson struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       int    `json:"color"`
	Thumbnail   string `json:"thumbnail"`
	Image       string `json:"image"`
	Footer      struct {
		Text string `json:"text"`
		Icon string `json:"icon"`
	} `json:"footer"`
	Fields []struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Inline bool   `json:"inline"`
	} `json:"fields"`
}

var Embed = &manager.Command{
	Trigger:     "embed",
	Aliases:     []string{"say", "announce", "echo"},
	Name:        "embed",
	Description: "Post a custom embed to a channel using JSON syntax",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionManageMessages) == 0 {
			return ctx.Reply("[!] You need Manage Messages permission to use this command.")
		}

		if len(ctx.Args) == 0 {
			return ctx.Reply("Usage: `.embed <#channel> <json_payload>` or `.embed <json_payload>`")
		}

		targetChanID := ctx.ChanID()
		jsonArgIdx := 0

		if m := rxEmbedChan.FindStringSubmatch(ctx.Args[0]); len(m) > 1 {
			targetChanID = m[1]
			jsonArgIdx = 1
		}

		if len(ctx.Args) <= jsonArgIdx {
			return ctx.Reply("[!] Missing JSON payload.")
		}

		jsonPayload := strings.Join(ctx.Args[jsonArgIdx:], " ")
		var payload EmbedJson
		if err := json.Unmarshal([]byte(jsonPayload), &payload); err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Invalid JSON payload: %v", err))
		}

		embed := &discordgo.MessageEmbed{
			Title:       payload.Title,
			Description: payload.Description,
			Color:       payload.Color,
		}

		if payload.Thumbnail != "" {
			embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: payload.Thumbnail}
		}

		if payload.Image != "" {
			embed.Image = &discordgo.MessageEmbedImage{URL: payload.Image}
		}

		if payload.Footer.Text != "" {
			embed.Footer = &discordgo.MessageEmbedFooter{
				Text:    payload.Footer.Text,
				IconURL: payload.Footer.Icon,
			}
		}

		for _, f := range payload.Fields {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}

		_, err = ctx.Session.ChannelMessageSendEmbed(targetChanID, embed)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("[!] Failed to send embed: %v", err))
		}

		if targetChanID != ctx.ChanID() {
			return ctx.Reply("[+] Embed posted successfully.")
		}
		return nil
	},
}
