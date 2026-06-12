package utility

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/manager"
	"skyvern/internal/storage"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	manager.RegisterHelp("voicemaster", []manager.HelpPage{
		{
			Command:     "voicemaster setup",
			Syntax:      ".voicemaster setup",
			Description: "Create Voice Master category, hub channel, and interface channel.",
		},
		{
			Command:     "voicemaster interface",
			Syntax:      ".voicemaster interface [channel]",
			Description: "Deploy the control panel interface.",
		},
	})
}

var VoiceMaster = &manager.Command{
	Trigger:     "voicemaster",
	Aliases:     []string{"vm", "tempvoice", "tempvc"},
	Name:        "voicemaster",
	Description: "Advanced temporary voice channel management system",
	Category:    "utility",
	Execute: func(ctx *manager.CommandContext) error {
		p, err := ctx.Session.UserChannelPermissions(ctx.AuthorID(), ctx.ChanID())
		if err != nil || (p&discordgo.PermissionManageChannels) == 0 {
			return ctx.Reply("[!] You need Manage Channels permission to configure Voice Master.")
		}

		if len(ctx.Args) == 0 {
			return ctx.SendHelp("voicemaster")
		}

		sub := strings.ToLower(ctx.Args[0])
		gid := ctx.GuildID()

		cfg, err := ctx.DB.GetTempVoiceCfg(gid)
		if err != nil {
			cfg = storage.TempVoiceCfg{Enabled: false}
		}

		switch sub {
		case "setup":
			cat, err := ctx.Session.GuildChannelCreateComplex(gid, discordgo.GuildChannelCreateData{
				Name: "Voice Master",
				Type: discordgo.ChannelTypeGuildCategory,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to create category: %v", err))
			}

			hub, err := ctx.Session.GuildChannelCreateComplex(gid, discordgo.GuildChannelCreateData{
				Name:     "Join to create",
				Type:     discordgo.ChannelTypeGuildVoice,
				ParentID: cat.ID,
				PermissionOverwrites: []*discordgo.PermissionOverwrite{
					{
						ID:    gid,
						Type:  discordgo.PermissionOverwriteTypeRole,
						Allow: discordgo.PermissionVoiceConnect | discordgo.PermissionViewChannel,
					},
				},
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to create hub voice channel: %v", err))
			}

			interfaceChan, err := ctx.Session.GuildChannelCreateComplex(gid, discordgo.GuildChannelCreateData{
				Name:     "voice-master-interface",
				Type:     discordgo.ChannelTypeGuildText,
				ParentID: cat.ID,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to create interface channel: %v", err))
			}

			cfg.ParentChannelID = hub.ID
			cfg.CategoryID = cat.ID
			cfg.InterfaceChanID = interfaceChan.ID
			cfg.Enabled = true

			emb, comps := buildInterfaceUI(ctx.Session, gid)
			res, err := ctx.Session.ChannelMessageSendComplex(interfaceChan.ID, &discordgo.MessageSend{
				Embeds:     []*discordgo.MessageEmbed{emb},
				Components: comps,
			})
			if err == nil && res != nil {
				cfg.InterfaceMsgID = res.ID
			}

			_ = ctx.DB.SaveTempVoiceCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Voice Master setup completed! Hub: <#%s>, Interface: <#%s>.", hub.ID, interfaceChan.ID))

		case "interface", "panel":
			chanID := ctx.ChanID()
			if len(ctx.Args) > 1 {
				chanID = strings.Trim(ctx.Args[1], "<#>")
			}

			emb, comps := buildInterfaceUI(ctx.Session, gid)
			res, err := ctx.Session.ChannelMessageSendComplex(chanID, &discordgo.MessageSend{
				Embeds:     []*discordgo.MessageEmbed{emb},
				Components: comps,
			})
			if err != nil {
				return ctx.Reply(fmt.Sprintf("[!] Failed to send interface panel: %v", err))
			}

			cfg.InterfaceChanID = chanID
			if res != nil {
				cfg.InterfaceMsgID = res.ID
			}
			_ = ctx.DB.SaveTempVoiceCfg(gid, cfg)
			return ctx.Reply(fmt.Sprintf("[+] Sent control panel to <#%s>.", chanID))

		default:
			return ctx.SendHelp("voicemaster")
		}
	},
}

func buildInterfaceUI(s *discordgo.Session, gid string) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	emojiServerID := config.GetGlobal().EmojiServerID
	if emojiServerID == "" {
		emojiServerID = "1411452931915645032"
	}

	var homeEmojis []*discordgo.Emoji
	if ems, err := s.GuildEmojis(emojiServerID); err == nil {
		homeEmojis = ems
	}

	var localEmojis []*discordgo.Emoji
	if ems, err := s.GuildEmojis(gid); err == nil {
		localEmojis = ems
	}

	getEmoji := func(name, fallback string) *discordgo.ComponentEmoji {
		normalizedName := strings.ReplaceAll(name, "-", "_")

		for _, e := range homeEmojis {
			if e.Name == normalizedName {
				return &discordgo.ComponentEmoji{
					Name: e.Name,
					ID:   e.ID,
				}
			}
		}

		for _, e := range localEmojis {
			if e.Name == normalizedName {
				return &discordgo.ComponentEmoji{
					Name: e.Name,
					ID:   e.ID,
				}
			}
		}

		for _, gState := range s.State.Guilds {
			for _, e := range gState.Emojis {
				if e.Name == normalizedName {
					return &discordgo.ComponentEmoji{
						Name: e.Name,
						ID:   e.ID,
					}
				}
			}
		}

		return &discordgo.ComponentEmoji{Name: fallback}
	}

	emojiLock := getEmoji("vm_lock", "🔒")
	emojiUnlock := getEmoji("vm_unlock", "🔓")
	emojiGhost := getEmoji("vm_ghost", "👻")
	emojiReveal := getEmoji("vm_reveal", "👁️")
	emojiRename := getEmoji("vm_rename", "✏️")
	emojiIncrease := getEmoji("vm_increase", "➕")
	emojiDecrease := getEmoji("vm_decrease", "➖")
	emojiInfo := getEmoji("vm_info", "ℹ️")
	emojiDisconnect := getEmoji("vm_disconnect", "👋")
	emojiActivity := getEmoji("vm_activity", "🎮")
	emojiClaim := getEmoji("vm_claim", "👑")

	emb := &discordgo.MessageEmbed{
		Color:       0x808080,
		Title:       "Voice Master",
		Description: "Manage your private voice channel using the interactive buttons below. Each action applies instantly to your current room.",
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Privacy & Access",
				Value: fmt.Sprintf(
					"> %s **Lock** - Prevent others from joining\n"+
						"> %s **Unlock** - Allow everyone to join\n"+
						"> %s **Hide** - Hide channel from the list\n"+
						"> %s **Show** - Make channel visible again",
					formatEmbedEmoji(emojiLock), formatEmbedEmoji(emojiUnlock), formatEmbedEmoji(emojiGhost), formatEmbedEmoji(emojiReveal),
				),
				Inline: false,
			},
			{
				Name: "Configuration",
				Value: fmt.Sprintf(
					"> %s **Rename** - Change your channel name\n"+
						"> %s **Limit +** - Increase player slots\n"+
						"> %s **Limit -** - Decrease player slots\n"+
						"> %s **Info** - View room statistics",
					formatEmbedEmoji(emojiRename), formatEmbedEmoji(emojiIncrease), formatEmbedEmoji(emojiDecrease), formatEmbedEmoji(emojiInfo),
				),
				Inline: false,
			},
			{
				Name: "Utility & Ownership",
				Value: fmt.Sprintf(
					"> %s **Kick** - Remove a user from the room\n"+
						"> %s **Activity** - Start a Discord activity\n"+
						"> %s **Claim** - Take ownership of an empty room",
					formatEmbedEmoji(emojiDisconnect), formatEmbedEmoji(emojiActivity), formatEmbedEmoji(emojiClaim),
				),
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Esoterica Voice • Click a button to execute the action",
		},
	}

	row1 := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "vm_lock", Label: "Lock", Style: discordgo.SecondaryButton, Emoji: emojiLock},
			discordgo.Button{CustomID: "vm_unlock", Label: "Unlock", Style: discordgo.SecondaryButton, Emoji: emojiUnlock},
			discordgo.Button{CustomID: "vm_ghost", Label: "Hide", Style: discordgo.SecondaryButton, Emoji: emojiGhost},
			discordgo.Button{CustomID: "vm_reveal", Label: "Show", Style: discordgo.SecondaryButton, Emoji: emojiReveal},
		},
	}

	row2 := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "vm_rename", Label: "Rename", Style: discordgo.SecondaryButton, Emoji: emojiRename},
			discordgo.Button{CustomID: "vm_limit_inc", Label: "Limit +", Style: discordgo.SecondaryButton, Emoji: emojiIncrease},
			discordgo.Button{CustomID: "vm_limit_dec", Label: "Limit -", Style: discordgo.SecondaryButton, Emoji: emojiDecrease},
			discordgo.Button{CustomID: "vm_info", Label: "Info", Style: discordgo.SecondaryButton, Emoji: emojiInfo},
		},
	}

	row3 := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "vm_disconnect", Label: "Kick", Style: discordgo.SecondaryButton, Emoji: emojiDisconnect},
			discordgo.Button{CustomID: "vm_activity", Label: "Activity", Style: discordgo.SecondaryButton, Emoji: emojiActivity},
			discordgo.Button{CustomID: "vm_claim", Label: "Claim Room", Style: discordgo.SecondaryButton, Emoji: emojiClaim},
		},
	}

	return emb, []discordgo.MessageComponent{row1, row2, row3}
}

func formatEmbedEmoji(emoji *discordgo.ComponentEmoji) string {
	if emoji.ID != "" {
		return fmt.Sprintf("<:%s:%s>", emoji.Name, emoji.ID)
	}
	return emoji.Name
}

func HandleVoiceMasterComponent(s *discordgo.Session, i *discordgo.InteractionCreate, mgr *manager.Manager) {
	var customID string
	if i.Type == discordgo.InteractionMessageComponent {
		customID = i.MessageComponentData().CustomID
	} else if i.Type == discordgo.InteractionModalSubmit {
		customID = i.ModalSubmitData().CustomID
	}

	if customID == "" {
		return
	}

	db := mgr.DB()
	uid := i.Member.User.ID
	gid := i.GuildID

	if customID == "vm_claim" {
		var voiceChanID string
		g, err := s.State.Guild(gid)
		if err == nil {
			for _, vs := range g.VoiceStates {
				if vs.UserID == uid {
					voiceChanID = vs.ChannelID
					break
				}
			}
		}

		if voiceChanID == "" {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "[!] Join a voice room to claim it.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		owner, err := db.GetTempVoiceChan(voiceChanID)
		if err != nil {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "[!] This is not a Voice Master room.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		ownerInChannel := false
		for _, vs := range g.VoiceStates {
			if vs.UserID == owner && vs.ChannelID == voiceChanID {
				ownerInChannel = true
				break
			}
		}

		if ownerInChannel {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "[!] Owner is still in the room.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		_ = db.SaveTempVoiceChan(voiceChanID, uid)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "[+] Room ownership transferred.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if strings.HasPrefix(customID, "vm_rename_modal_") {
		voiceChanID := strings.TrimPrefix(customID, "vm_rename_modal_")
		var newName string
		for _, row := range i.ModalSubmitData().Components {
			if ar, ok := row.(*discordgo.ActionsRow); ok {
				for _, comp := range ar.Components {
					if ti, ok := comp.(*discordgo.TextInput); ok && ti.CustomID == "name" {
						newName = ti.Value
						break
					}
				}
			}
		}

		if newName == "" {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "[!] Invalid name.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		_, _ = s.ChannelEdit(voiceChanID, &discordgo.ChannelEdit{Name: newName})
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("[+] Room renamed to `%s`.", newName),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if strings.HasPrefix(customID, "vm_disconnect_select_") {
		targetID := i.MessageComponentData().Values[0]
		_ = s.GuildMemberMove(gid, targetID, nil)

		targetName := targetID
		tUser, err := s.User(targetID)
		if err == nil && tUser != nil {
			targetName = tUser.Username
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("[+] %s disconnected.", targetName),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	var voiceChanID string
	g, err := s.State.Guild(gid)
	if err == nil {
		for _, vs := range g.VoiceStates {
			if vs.UserID == uid {
				voiceChanID = vs.ChannelID
				break
			}
		}
	}

	if voiceChanID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "[!] You must be in your voice room to use these controls.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	owner, err := db.GetTempVoiceChan(voiceChanID)
	if err != nil || owner != uid {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "[!] You do not own this voice channel or it is not a temporary room.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	ch, err := s.State.Channel(voiceChanID)
	if err != nil {
		ch, _ = s.Channel(voiceChanID)
	}

	if ch == nil {
		return
	}

	var content string

	switch customID {
	case "vm_lock":
		_ = s.ChannelPermissionSet(voiceChanID, gid, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionVoiceConnect)
		content = "[+] Room locked."

	case "vm_unlock":
		_ = s.ChannelPermissionSet(voiceChanID, gid, discordgo.PermissionOverwriteTypeRole, discordgo.PermissionVoiceConnect, 0)
		content = "[+] Room unlocked."

	case "vm_ghost":
		_ = s.ChannelPermissionSet(voiceChanID, gid, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionViewChannel)
		content = "[+] Room hidden."

	case "vm_reveal":
		_ = s.ChannelPermissionSet(voiceChanID, gid, discordgo.PermissionOverwriteTypeRole, discordgo.PermissionViewChannel, 0)
		content = "[+] Room revealed."

	case "vm_rename":
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: fmt.Sprintf("vm_rename_modal_%s", voiceChanID),
				Title:    "Rename Room",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "name",
								Label:       "New Name",
								Style:       discordgo.TextInputShort,
								Placeholder: ch.Name,
								Required:    true,
								MaxLength:   100,
							},
						},
					},
				},
			},
		})
		return

	case "vm_limit_inc":
		newLimit := ch.UserLimit + 1
		if newLimit > 99 {
			newLimit = 99
		}
		_, _ = s.ChannelEdit(voiceChanID, &discordgo.ChannelEdit{UserLimit: newLimit})
		content = fmt.Sprintf("[+] Limit increased to %d.", newLimit)

	case "vm_limit_dec":
		newLimit := ch.UserLimit - 1
		if newLimit < 0 {
			newLimit = 0
		}
		_, _ = s.ChannelEdit(voiceChanID, &discordgo.ChannelEdit{UserLimit: newLimit})
		content = fmt.Sprintf("[+] Limit decreased to %d.", newLimit)

	case "vm_info":
		membersCount := 0
		for _, vs := range g.VoiceStates {
			if vs.ChannelID == voiceChanID {
				membersCount++
			}
		}

		isLocked := false
		isHidden := false
		for _, o := range ch.PermissionOverwrites {
			if o.ID == gid && o.Type == discordgo.PermissionOverwriteTypeRole {
				if (o.Deny & discordgo.PermissionVoiceConnect) != 0 {
					isLocked = true
				}
				if (o.Deny & discordgo.PermissionViewChannel) != 0 {
					isHidden = true
				}
			}
		}

		statusStr := "🔓 Unlocked"
		if isLocked {
			statusStr = "🔒 Locked"
		}
		visibilityStr := "👁️ Visible"
		if isHidden {
			visibilityStr = "👻 Hidden"
		}
		limitStr := "Unlimited"
		if ch.UserLimit > 0 {
			limitStr = strconv.Itoa(ch.UserLimit)
		}

		emb := &discordgo.MessageEmbed{
			Color: 0x808080,
			Title: "🎤 Voice Room Information",
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Owner", Value: fmt.Sprintf("<@%s>", owner), Inline: true},
				{Name: "Channel", Value: ch.Name, Inline: true},
				{Name: "Limit", Value: limitStr, Inline: true},
				{Name: "Members", Value: strconv.Itoa(membersCount), Inline: true},
				{Name: "Status", Value: statusStr, Inline: true},
				{Name: "Visibility", Value: visibilityStr, Inline: true},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: "Esoterica Voice Master • Room Details",
			},
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{emb},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		return

	case "vm_disconnect":
		var options []discordgo.SelectMenuOption
		for _, vs := range g.VoiceStates {
			if vs.ChannelID == voiceChanID && vs.UserID != uid {
				member, err := s.State.Member(gid, vs.UserID)
				if err != nil {
					member, _ = s.GuildMember(gid, vs.UserID)
				}
				if member != nil && member.User != nil {
					options = append(options, discordgo.SelectMenuOption{
						Label: member.User.Username,
						Value: member.User.ID,
					})
				}
			}
		}

		if len(options) == 0 {
			content = "[!] No other members in room."
		} else {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.SelectMenu{
									CustomID:    fmt.Sprintf("vm_disconnect_select_%s", voiceChanID),
									Placeholder: "Select a member to disconnect",
									Options:     options,
								},
							},
						},
					},
				},
			})
			return
		}

	case "vm_activity":
		content = "Start activities directly from the Discord Activity button in your voice channel."

	default:
		content = "[!] Unknown action."
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
