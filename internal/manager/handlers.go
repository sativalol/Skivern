package manager

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"skyvern/internal/moderation"
	"skyvern/internal/storage"
)

func (m *Manager) attachHandlers(sess *discordgo.Session, state *instState) {
	sess.AddHandler(func(s *discordgo.Session, msg *discordgo.MessageCreate) {
		if msg.Author == nil || msg.Author.Bot {
			return
		}
		if m.checkAntispam(s, msg) {
			return
		}
		if m.checkFilter(s, msg) {
			return
		}
		if m.checkAntilink(s, msg) {
			return
		}
		s.State.MessageAdd(msg.Message)
		_ = m.db.IncrementUserMessages(msg.GuildID, msg.Author.ID)

		prefix := state.cfg.Prefix
		if gp, err := m.GetPrefix(msg.GuildID); err == nil && gp != "" {
			prefix = gp
		}

		isAFKCmd := false
		if strings.HasPrefix(msg.Content, prefix) {
			pFields := strings.Fields(strings.TrimPrefix(msg.Content, prefix))
			if len(pFields) > 0 {
				cmdLower := strings.ToLower(pFields[0])
				if cmdLower == "afk" || cmdLower == "brb" || cmdLower == "away" {
					isAFKCmd = true
				}
			}
		}
		if !isAFKCmd {
			if status, err := m.db.GetAFK(msg.GuildID, msg.Author.ID); err == nil {
				_ = m.db.DeleteAFK(msg.GuildID, msg.Author.ID)
				dur := time.Since(status.Time).Round(time.Second)
				_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Welcome back <@%s>, I removed your AFK. You were away for %v and were pinged %d times.", msg.Author.ID, dur, status.Pings))
			}
		}

		afkChecked := make(map[string]bool)
		var uids []string
		for _, mention := range msg.Mentions {
			if mention.ID != msg.Author.ID && !mention.Bot && !afkChecked[mention.ID] {
				afkChecked[mention.ID] = true
				uids = append(uids, mention.ID)
			}
		}
		if msg.ReferencedMessage != nil && msg.ReferencedMessage.Author != nil {
			refAuthor := msg.ReferencedMessage.Author.ID
			if refAuthor != msg.Author.ID && !msg.ReferencedMessage.Author.Bot && !afkChecked[refAuthor] {
				afkChecked[refAuthor] = true
				uids = append(uids, refAuthor)
			}
		}
		for _, uid := range uids {
			if status, err := m.db.GetAFK(msg.GuildID, uid); err == nil {
				status.Pings++
				_ = m.db.SaveAFK(msg.GuildID, uid, status)
				dur := time.Since(status.Time).Round(time.Second)
				_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> is AFK: %s (%v ago) - Mentioned %d times.", uid, status.Reason, dur, status.Pings))
			}
		}

		if reacts, err := m.db.ListAutoreact(msg.GuildID); err == nil && len(reacts) > 0 {
			lowerContent := strings.ToLower(msg.Content)
			for trigger, emoji := range reacts {
				if strings.Contains(lowerContent, trigger) {
					_ = s.MessageReactionAdd(msg.ChannelID, msg.ID, emoji)
				}
			}
		}

		if responders, err := m.db.ListAutoresponder(msg.GuildID); err == nil && len(responders) > 0 {
			lowerContent := strings.ToLower(msg.Content)
			for trigger, response := range responders {
				if strings.Contains(lowerContent, trigger) {
					if strings.HasSuffix(response, "-embed") {
						cleanedText := strings.TrimSpace(strings.TrimSuffix(response, "-embed"))
						embed := &discordgo.MessageEmbed{
							Description: cleanedText,
							Color:       0x7289da,
						}
						_, _ = s.ChannelMessageSendEmbed(msg.ChannelID, embed)
					} else {
						_, _ = s.ChannelMessageSend(msg.ChannelID, response)
					}
				}
			}
		}

		if msg.Author.ID == "302050872383242240" {
			hasBumpWord := false
			for _, embed := range msg.Embeds {
				desc := strings.ToLower(embed.Description)
				if strings.Contains(desc, "bump done") || strings.Contains(desc, "page bumped") {
					hasBumpWord = true
					break
				}
			}
			if hasBumpWord {
				if bumpCfg, err := m.db.GetBumpCfg(msg.GuildID); err == nil && bumpCfg.Enabled && bumpCfg.ChannelID != "" {
					go func(g string, b storage.BumpCfg) {
						time.Sleep(2 * time.Hour)
						m.mu.RLock()
						var activeSess *discordgo.Session
						for _, inst := range m.instances {
							if inst.running {
								activeSess = inst.session
								break
							}
						}
						m.mu.RUnlock()

						if activeSess != nil {
							if currentCfg, err := m.db.GetBumpCfg(g); err == nil && currentCfg.Enabled {
								_, _ = activeSess.ChannelMessageSend(currentCfg.ChannelID, currentCfg.Message)
							}
						}
					}(msg.GuildID, bumpCfg)
				}
			}
		}

		if !strings.HasPrefix(msg.Content, prefix) {
			return
		}
		parts := strings.Fields(strings.TrimPrefix(msg.Content, prefix))
		if len(parts) == 0 {
			return
		}
		cmd := m.findByTrigger(strings.ToLower(parts[0]))
		if cmd == nil {
			if tmpl, err := m.db.GetInvoke(msg.GuildID, parts[0]); err == nil && tmpl != "" {
				resText := renderTemplate(tmpl, msg.Message, parts[1:])
				_, _ = s.ChannelMessageSend(msg.ChannelID, resText)
			}
			return
		}
		cfg := state.cfg
		cfg.Prefix = prefix
		ctx := &CommandContext{
			Session:  s,
			Message:  msg.Message,
			Args:     parts[1:],
			Cfg:      cfg,
			DB:       m.db,
			ClientID: state.clientID,
			Mgr:      m,
		}
		m.stats.incPrefix(state.clientID)
		go m.LogCommandUsage(s, cmd, ctx)
		go func() {
			if err := cmd.Execute(ctx); err != nil {
				fmt.Printf("[%s] %q: %v\n", state.clientID, parts[0], err)
			}
		}()
	})

	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}
		name := i.ApplicationCommandData().Name
		cmd := m.findByName(name)
		if cmd == nil {
			return
		}
		ctx := &CommandContext{
			Session:  s,
			Interact: i.Interaction,
			Cfg:      state.cfg,
			DB:       m.db,
			ClientID: state.clientID,
			Mgr:      m,
		}
		m.stats.incSlash(state.clientID)
		go func() {
			if err := cmd.Execute(ctx); err != nil {
				fmt.Printf("[%s] /%s: %v\n", state.clientID, name, err)
			}
		}()
	})

	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionMessageComponent {
			id := i.MessageComponentData().CustomID

			if strings.HasPrefix(id, "btnrole_") {
				go func() {
					roleID, err := m.db.GetButtonRole(i.GuildID, i.Message.ID, id)
					if err != nil || roleID == "" {
						_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: "[!] This button role is not registered or has expired.",
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						})
						return
					}

					if !m.checkRoleSafety(s, i.GuildID, roleID) {
						_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: "[!] Security Check Failed: This role cannot be self-assigned due to dangerous permissions or hierarchy constraint.",
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						})
						return
					}

					var content string
					hasRole := false
					for _, r := range i.Member.Roles {
						if r == roleID {
							hasRole = true
							break
						}
					}

					if hasRole {
						err = s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, roleID)
						if err == nil {
							content = fmt.Sprintf("[+] Removed role <@&%s>.", roleID)
						} else {
							content = fmt.Sprintf("[!] Failed to remove role: %v", err)
						}
					} else {
						err = s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, roleID)
						if err == nil {
							content = fmt.Sprintf("[+] Assigned role <@&%s>.", roleID)
						} else {
							content = fmt.Sprintf("[!] Failed to assign role: %v", err)
						}
					}

					_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: content,
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
				}()
				return
			}

			m.mu.RLock()
			handler, ok := m.compHandlers[id]
			if !ok {
				for k, fn := range m.compHandlers {
					if strings.HasSuffix(k, "*") && strings.HasPrefix(id, strings.TrimSuffix(k, "*")) {
						handler = fn
						ok = true
						break
					}
				}
			}
			m.mu.RUnlock()
			if ok {
				go handler(s, i)
			}
		}
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildMemberAdd) {
		if e.Member == nil || e.Member.User == nil {
			return
		}
		go m.LogMemberJoin(s, e)
		go m.TrackAntiraidJoin(s, e.GuildID, e.Member)
		if e.Member.User.Bot {
			go m.TrackAntinuke(s, e.GuildID, e.Member.User.ID, discordgo.AuditLogActionBotAdd)
		}
		entries, err := m.db.ListStickyRoles(e.GuildID)
		if err == nil {
			for _, entry := range entries {
				if entry.UserID == e.Member.User.ID || entry.UserID == "everyone" {
					_ = s.GuildMemberRoleAdd(e.GuildID, e.Member.User.ID, entry.RoleID)
				}
			}
		}
		ar, err := m.db.GetAutoroles(e.GuildID)
		if err == nil {
			for _, rid := range ar {
				_ = s.GuildMemberRoleAdd(e.GuildID, e.Member.User.ID, rid)
			}
		}
	})

	sess.AddHandler(func(s *discordgo.Session, _ *discordgo.GuildCreate) {
		m.stats.setGuilds(state.clientID, len(s.State.Guilds))
	})

	sess.AddHandler(func(s *discordgo.Session, u *discordgo.GuildMemberUpdate) {
		if u.Member == nil || u.Member.User == nil {
			return
		}
		go m.LogMemberUpdate(s, u)
		if locked, err := m.db.GetNicklock(u.GuildID, u.Member.User.ID); err == nil {
			if u.Member.Nick != locked {
				_ = s.GuildMemberNickname(u.GuildID, u.Member.User.ID, locked)
			}
		}
		if u.Member.PremiumSince != nil && !u.Member.PremiumSince.IsZero() {
			m.boostMu.Lock()
			lastLog, exists := m.lastBoostLogged[u.GuildID+":"+u.Member.User.ID]
			isNew := !exists || time.Since(lastLog) > 10*time.Minute
			if isNew && time.Since(*u.Member.PremiumSince) < 2*time.Minute {
				m.lastBoostLogged[u.GuildID+":"+u.Member.User.ID] = time.Now()
				m.boostMu.Unlock()
				go m.triggerBoostMsg(s, u.GuildID, u.Member)
			} else {
				m.boostMu.Unlock()
			}
		}
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildBanAdd) {
		go m.LogMemberBan(s, e)
		go moderation.ProcAudit(s, m.db, e.GuildID, e.User.ID, discordgo.AuditLogActionMemberBanAdd)
		go m.TrackAntinuke(s, e.GuildID, e.User.ID, discordgo.AuditLogActionMemberBanAdd)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildBanRemove) {
		go m.LogMemberUnban(s, e)
		go moderation.ProcAudit(s, m.db, e.GuildID, e.User.ID, discordgo.AuditLogActionMemberBanRemove)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildMemberRemove) {
		go m.LogMemberLeave(s, e)
		go moderation.ProcAudit(s, m.db, e.GuildID, e.Member.User.ID, discordgo.AuditLogActionMemberKick)
		go m.TrackAntinuke(s, e.GuildID, e.Member.User.ID, discordgo.AuditLogActionMemberKick)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.MessageDelete) {
		go m.LogMessageDelete(s, e)
		if e.BeforeDelete != nil {
			if e.BeforeDelete.Author == nil || e.BeforeDelete.Author.Bot {
				return
			}
			AddDeleted(e.ChannelID, DeletedMsg{
				Author:    e.BeforeDelete.Author,
				Content:   e.BeforeDelete.Content,
				ChannelID: e.ChannelID,
				Time:      time.Now(),
			})
		}
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.MessageUpdate) {
		go m.LogMessageUpdate(s, e)
		if e.BeforeUpdate != nil {
			if e.BeforeUpdate.Author == nil || e.BeforeUpdate.Author.Bot {
				return
			}
			if e.BeforeUpdate.Content == e.Content {
				return
			}
			AddEdited(e.ChannelID, EditedMsg{
				Author:    e.BeforeUpdate.Author,
				Old:       e.BeforeUpdate.Content,
				New:       e.Content,
				ChannelID: e.ChannelID,
				Time:      time.Now(),
			})
		}
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.MessageReactionRemove) {
		go m.LogReactionRemove(s, e)
		usr, err := s.User(e.UserID)
		if err != nil || usr.Bot {
			return
		}

		emojiQuery := e.Emoji.Name
		if e.Emoji.ID != "" {
			emojiQuery = e.Emoji.APIName()
		}
		roleID, err := m.db.GetReactRole(e.GuildID, e.MessageID, emojiQuery)
		if err == nil && roleID != "" {
			if m.checkRoleSafety(s, e.GuildID, roleID) {
				_ = s.GuildMemberRoleRemove(e.GuildID, e.UserID, roleID)
			}
		}

		AddReact(e.ChannelID, DeletedReact{
			Author:    usr,
			Emoji:     &e.Emoji,
			ChannelID: e.ChannelID,
			Time:      time.Now(),
		})
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.MessageReactionAdd) {
		if e.UserID == s.State.User.ID {
			return
		}
		go m.LogReactionAdd(s, e)
		go m.handleReactionAdd(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.VoiceStateUpdate) {
		go m.LogVoiceStateUpdate(s, e)
		go m.handleVoiceStateUpdate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.PresenceUpdate) {
		go m.handlePresenceUpdate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildRoleCreate) {
		go m.LogRoleCreate(s, e)
		go m.TrackAntinuke(s, e.GuildID, e.Role.ID, discordgo.AuditLogActionRoleCreate)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildRoleDelete) {
		go m.LogRoleDelete(s, e)
		go m.TrackAntinuke(s, e.GuildID, e.RoleID, discordgo.AuditLogActionRoleDelete)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ChannelCreate) {
		go m.LogChannelCreate(s, e)
		go m.TrackAntinuke(s, e.GuildID, e.Channel.ID, discordgo.AuditLogActionChannelCreate)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ChannelDelete) {
		go m.LogChannelDelete(s, e)
		go m.TrackAntinuke(s, e.GuildID, e.Channel.ID, discordgo.AuditLogActionChannelDelete)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.MessageDeleteBulk) {
		go m.LogMessageDeleteBulk(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.UserUpdate) {
		go m.LogUserUpdate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildRoleUpdate) {
		go m.LogRoleUpdate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ChannelUpdate) {
		go m.LogChannelUpdate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildUpdate) {
		go m.LogGuildUpdate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.InviteCreate) {
		go m.LogInviteCreate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.InviteDelete) {
		go m.LogInviteDelete(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.AutoModerationActionExecution) {
		go m.LogAutoModExecution(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildScheduledEventCreate) {
		go m.LogScheduledEventCreate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildScheduledEventDelete) {
		go m.LogScheduledEventDelete(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildScheduledEventUpdate) {
		go m.LogScheduledEventUpdate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ThreadCreate) {
		go m.LogThreadCreate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ThreadDelete) {
		go m.LogThreadDelete(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ThreadUpdate) {
		go m.LogThreadUpdate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.WebhooksUpdate) {
		go m.LogWebhooksUpdate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildEmojisUpdate) {
		go m.LogGuildEmojisUpdate(s, e)
	})
}

func (m *Manager) triggerBoostMsg(s *discordgo.Session, gid string, mem *discordgo.Member) {
	cfg, err := m.db.GetBoostCfg(gid)
	if err != nil || cfg.ChannelID == "" || cfg.Message == "" {
		return
	}
	text := cfg.Message
	text = strings.ReplaceAll(text, "{user}", mem.User.Username)
	text = strings.ReplaceAll(text, "{user.mention}", mem.User.Mention())
	text = strings.ReplaceAll(text, "{user.name}", mem.User.Username)

	gName := gid
	g, err := s.State.Guild(gid)
	if err == nil {
		gName = g.Name
	} else {
		if g, err = s.Guild(gid); err == nil {
			gName = g.Name
		}
	}
	text = strings.ReplaceAll(text, "{guild.name}", gName)

	_, _ = s.ChannelMessageSend(cfg.ChannelID, text)
}

func (m *Manager) handleReactionAdd(s *discordgo.Session, e *discordgo.MessageReactionAdd) {
	emojiQuery := e.Emoji.Name
	if e.Emoji.ID != "" {
		emojiQuery = e.Emoji.APIName()
	}
	roleID, err := m.db.GetReactRole(e.GuildID, e.MessageID, emojiQuery)
	if err == nil && roleID != "" {
		if m.checkRoleSafety(s, e.GuildID, roleID) {
			_ = s.GuildMemberRoleAdd(e.GuildID, e.UserID, roleID)
		}
		return
	}

	if e.Emoji.Name == "⭐" {
		if sbCfg, err := m.db.GetStarboardCfg(e.GuildID); err == nil && sbCfg.Enabled && sbCfg.ChannelID != "" {
			if msg, err := s.ChannelMessage(e.ChannelID, e.MessageID); err == nil {
				stars := 0
				for _, r := range msg.Reactions {
					if r.Emoji.Name == "⭐" {
						stars = r.Count
						break
					}
				}
				if stars >= sbCfg.Threshold {
					m.postToStarboard(s, sbCfg.ChannelID, msg, stars)
				}
			}
		}
	}

	cfg, err := m.db.GetHallCfg(e.GuildID)
	if err != nil {
		return
	}

	isFame := e.Emoji.Name == "⭐" || e.Emoji.Name == "👍"
	isShame := e.Emoji.Name == "👎" || e.Emoji.Name == "🤡" || e.Emoji.Name == "💩"

	if !isFame && !isShame {
		return
	}

	msg, err := s.ChannelMessage(e.ChannelID, e.MessageID)
	if err != nil {
		return
	}

	fameCount := 0
	shameCount := 0
	for _, r := range msg.Reactions {
		if r.Emoji.Name == "⭐" || r.Emoji.Name == "👍" {
			fameCount += r.Count
		}
		if r.Emoji.Name == "👎" || r.Emoji.Name == "🤡" || r.Emoji.Name == "💩" {
			shameCount += r.Count
		}
	}

	if isFame && cfg.FameChannelID != "" && fameCount >= cfg.FameThreshold {
		posted, _ := m.db.IsHallPosted(e.GuildID, e.MessageID, "fame")
		if !posted {
			_ = m.db.SetHallPosted(e.GuildID, e.MessageID, "fame")
			m.postToHall(s, cfg.FameChannelID, msg, "Hall of Fame", 0xffd700)
		}
	}

	if isShame && cfg.ShameChannelID != "" && shameCount >= cfg.ShameThreshold {
		posted, _ := m.db.IsHallPosted(e.GuildID, e.MessageID, "shame")
		if !posted {
			_ = m.db.SetHallPosted(e.GuildID, e.MessageID, "shame")
			m.postToHall(s, cfg.ShameChannelID, msg, "Hall of Shame", 0x964b00)
		}
	}
}

func (m *Manager) postToHall(s *discordgo.Session, targetChanID string, msg *discordgo.Message, title string, color int) {
	authorName := "Unknown"
	authorAvatar := ""
	if msg.Author != nil {
		authorName = msg.Author.Username
		authorAvatar = msg.Author.AvatarURL("")
	}

	content := msg.Content
	if content == "" {
		content = "*(No text content)*"
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: content,
		Color:       color,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    authorName,
			IconURL: authorAvatar,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Original Message",
				Value:  fmt.Sprintf("[Jump to Message](https://discord.com/channels/%s/%s/%s)", msg.GuildID, msg.ChannelID, msg.ID),
				Inline: true,
			},
		},
	}

	if len(msg.Attachments) > 0 {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: msg.Attachments[0].URL,
		}
	}

	_, _ = s.ChannelMessageSendEmbed(targetChanID, embed)
}

func (m *Manager) checkRoleSafety(s *discordgo.Session, gid string, roleID string) bool {
	botMember, err := s.GuildMember(gid, s.State.User.ID)
	if err != nil {
		return false
	}

	roles, err := s.GuildRoles(gid)
	if err != nil {
		return false
	}

	botMaxPos := -1
	var targetRole *discordgo.Role
	for _, r := range roles {
		if r.ID == roleID {
			targetRole = r
		}
		for _, botRoleID := range botMember.Roles {
			if r.ID == botRoleID && r.Position > botMaxPos {
				botMaxPos = r.Position
			}
		}
	}

	if targetRole == nil {
		return false
	}

	if targetRole.Position >= botMaxPos {
		return false
	}

	dangerousPerms := int64(discordgo.PermissionAdministrator |
		discordgo.PermissionManageRoles |
		discordgo.PermissionManageGuild |
		discordgo.PermissionBanMembers |
		discordgo.PermissionKickMembers |
		discordgo.PermissionManageWebhooks |
		discordgo.PermissionManageChannels)

	if (targetRole.Permissions & dangerousPerms) != 0 {
		return false
	}

	return true
}

func (m *Manager) handleVoiceStateUpdate(s *discordgo.Session, e *discordgo.VoiceStateUpdate) {
	cfg, err := m.db.GetTempVoiceCfg(e.GuildID)
	if err != nil || !cfg.Enabled || cfg.ParentChannelID == "" {
		return
	}

	if e.ChannelID == cfg.ParentChannelID {
		mName := "Temp VC"
		mInfo, err := s.GuildMember(e.GuildID, e.UserID)
		if err == nil && mInfo.User != nil {
			mName = fmt.Sprintf("%s's Channel", mInfo.User.Username)
		}

		newCh, err := s.GuildChannelCreateComplex(e.GuildID, discordgo.GuildChannelCreateData{
			Name:     mName,
			Type:     discordgo.ChannelTypeGuildVoice,
			ParentID: cfg.CategoryID,
		})
		if err == nil {
			_ = m.db.SaveTempVoiceChan(newCh.ID, e.UserID)
			_ = s.GuildMemberMove(e.GuildID, e.UserID, &newCh.ID)
		}
		return
	}

	if e.BeforeUpdate != nil && e.BeforeUpdate.ChannelID != "" && e.BeforeUpdate.ChannelID != e.ChannelID {
		m.cleanTempVC(s, e.GuildID, e.BeforeUpdate.ChannelID)
	}
}

func (m *Manager) cleanTempVC(s *discordgo.Session, gid, chanID string) {
	owner, err := m.db.GetTempVoiceChan(chanID)
	if err != nil || owner == "" {
		return
	}

	g, err := s.State.Guild(gid)
	if err != nil {
		g, err = s.Guild(gid)
	}
	if err != nil {
		return
	}

	count := 0
	for _, vs := range g.VoiceStates {
		if vs.ChannelID == chanID {
			count++
		}
	}

	if count == 0 {
		_, _ = s.ChannelDelete(chanID)
		_ = m.db.DeleteTempVoiceChan(chanID)
	}
}

func (m *Manager) handlePresenceUpdate(s *discordgo.Session, e *discordgo.PresenceUpdate) {
	if e.User == nil {
		return
	}
	cfg, err := m.db.GetVanityCfg(e.GuildID)
	if err != nil || !cfg.Enabled || cfg.Text == "" || cfg.RoleID == "" {
		return
	}

	hasVanity := false
	for _, act := range e.Activities {
		if act.Type == discordgo.ActivityTypeCustom {
			if strings.Contains(act.State, cfg.Text) {
				hasVanity = true
				break
			}
		}
	}

	mem, err := s.State.Member(e.GuildID, e.User.ID)
	if err != nil {
		mem, err = s.GuildMember(e.GuildID, e.User.ID)
	}
	if err != nil {
		return
	}

	hasRole := false
	for _, r := range mem.Roles {
		if r == cfg.RoleID {
			hasRole = true
			break
		}
	}

	if hasVanity && !hasRole {
		if m.checkRoleSafety(s, e.GuildID, cfg.RoleID) {
			_ = s.GuildMemberRoleAdd(e.GuildID, e.User.ID, cfg.RoleID)
		}
	} else if !hasVanity && hasRole {
		if m.checkRoleSafety(s, e.GuildID, cfg.RoleID) {
			_ = s.GuildMemberRoleRemove(e.GuildID, e.User.ID, cfg.RoleID)
		}
	}
}

func (m *Manager) postToStarboard(s *discordgo.Session, targetChanID string, msg *discordgo.Message, stars int) {
	sbMsgID, err := m.db.GetStarboardMsg(msg.ID)

	authorName := "Unknown"
	authorAvatar := ""
	if msg.Author != nil {
		authorName = msg.Author.Username
		authorAvatar = msg.Author.AvatarURL("")
	}

	content := msg.Content
	if content == "" {
		content = "*(No text content)*"
	}

	embed := &discordgo.MessageEmbed{
		Description: content,
		Color:       0xffac33,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    authorName,
			IconURL: authorAvatar,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Original Message",
				Value:  fmt.Sprintf("[Jump to Message](https://discord.com/channels/%s/%s/%s)", msg.GuildID, msg.ChannelID, msg.ID),
				Inline: true,
			},
		},
	}

	if len(msg.Attachments) > 0 {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: msg.Attachments[0].URL,
		}
	}

	contentStr := fmt.Sprintf("⭐ **%d** | <#%s>", stars, msg.ChannelID)

	if err == nil && sbMsgID != "" {
		_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:      sbMsgID,
			Channel: targetChanID,
			Content: &contentStr,
			Embeds:  &[]*discordgo.MessageEmbed{embed},
		})
	} else {
		newMsg, err := s.ChannelMessageSendComplex(targetChanID, &discordgo.MessageSend{
			Content: contentStr,
			Embeds:  []*discordgo.MessageEmbed{embed},
		})
		if err == nil {
			_ = m.db.SaveStarboardMsg(msg.ID, newMsg.ID)
		}
	}
}
