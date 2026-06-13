package manager

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"skyvern/internal/moderation"
	"skyvern/internal/storage"
)

func safeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("PANIC RECOVER %v\n%s\n", r, debug.Stack())
			}
		}()
		fn()
	}()
}


func (m *Manager) attachHandlers(sess *discordgo.Session, state *instState) {
	sess.AddHandler(func(s *discordgo.Session, msg *discordgo.MessageCreate) {
		if msg.Author == nil || msg.Author.Bot {
			return
		}

		// Image-only gallery check
		if msg.GuildID != "" {
			if exists, err := m.db.IsImgOnlyChannel(msg.GuildID, msg.ChannelID); err == nil && exists {
				hasImg := false
				if len(msg.Attachments) > 0 {
					hasImg = true
				} else {
					lowerContent := strings.ToLower(msg.Content)
					if strings.Contains(lowerContent, "http://") || strings.Contains(lowerContent, "https://") {
						if strings.Contains(lowerContent, ".png") || strings.Contains(lowerContent, ".jpg") ||
							strings.Contains(lowerContent, ".jpeg") || strings.Contains(lowerContent, ".gif") ||
							strings.Contains(lowerContent, ".webp") || strings.Contains(lowerContent, "tenor.com") ||
							strings.Contains(lowerContent, "giphy.com") || strings.Contains(lowerContent, "cdn.discordapp.com") {
							hasImg = true
						}
					}
				}
				if !hasImg {
					_ = s.ChannelMessageDelete(msg.ChannelID, msg.ID)
					return
				}
			}
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

		var personalPrefix string
		if up, err := m.db.GetUserPrefix(msg.Author.ID); err == nil && up != "" {
			personalPrefix = up
		}

		matchedPrefix := ""
		mentionPrefix := ""
		mentionNickPrefix := ""
		if s.State.User != nil {
			mentionPrefix = "<@" + s.State.User.ID + ">"
			mentionNickPrefix = "<@!" + s.State.User.ID + ">"
		}

		if personalPrefix != "" && strings.HasPrefix(msg.Content, personalPrefix) {
			matchedPrefix = personalPrefix
		} else if strings.HasPrefix(msg.Content, prefix) {
			matchedPrefix = prefix
		} else if mentionPrefix != "" && strings.HasPrefix(msg.Content, mentionPrefix) {
			matchedPrefix = mentionPrefix
		} else if mentionNickPrefix != "" && strings.HasPrefix(msg.Content, mentionNickPrefix) {
			matchedPrefix = mentionNickPrefix
		}

		isAFKCmd := false
		if matchedPrefix != "" {
			pFields := strings.Fields(strings.TrimPrefix(msg.Content, matchedPrefix))
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

		// Sticky message update
		if msg.GuildID != "" {
			if sm, err := m.db.GetStickyMessage(msg.GuildID, msg.ChannelID); err == nil && sm.Message != "" {
				if sm.LastMsgID != "" {
					_ = s.ChannelMessageDelete(msg.ChannelID, sm.LastMsgID)
				}
				text := replacePlaceholders(sm.Message, msg.Author, s, msg.GuildID)
				var newMsg *discordgo.Message
				var err error
				if strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}") {
					var payload struct {
						Title       string `json:"title"`
						Description string `json:"description"`
						Color       int    `json:"color"`
						Thumbnail   string `json:"thumbnail"`
						Image       string `json:"image"`
						Footer      struct {
							Text string `json:"text"`
							Icon string `json:"icon"`
						} `json:"footer"`
					}
					if json.Unmarshal([]byte(text), &payload) == nil {
						embed := &discordgo.MessageEmbed{
							Title:       payload.Title,
							Description: payload.Description,
							Color:       payload.Color,
						}
						if embed.Color == 0 {
							embed.Color = 0x808080
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
						newMsg, err = s.ChannelMessageSendEmbed(msg.ChannelID, embed)
					}
				}
				if newMsg == nil {
					newMsg, err = s.ChannelMessageSend(msg.ChannelID, text)
				}
				if err == nil && newMsg != nil {
					sm.LastMsgID = newMsg.ID
					_ = m.db.SaveStickyMessage(msg.GuildID, msg.ChannelID, sm)
				}
			}
		}

		if matchedPrefix == "" || !strings.HasPrefix(msg.Content, matchedPrefix) {
			return
		}
		parts := strings.Fields(strings.TrimPrefix(msg.Content, matchedPrefix))
		if len(parts) == 0 {
			if s.State.User != nil && (matchedPrefix == "<@"+s.State.User.ID+">" || matchedPrefix == "<@!"+s.State.User.ID+">") {
				_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("My prefix in this server is `%s`", prefix))
			}
			return
		}

		// Alias resolution
		trigger := strings.ToLower(parts[0])
		if alias, err := m.db.GetAlias(msg.GuildID, trigger); err == nil && alias != "" {
			aliasParts := strings.Fields(alias)
			if len(aliasParts) > 0 {
				parts = append(aliasParts, parts[1:]...)
			}
		}

		cmd := m.findByTrigger(strings.ToLower(parts[0]))
		if cmd == nil {
			var tmpl string
			if msg.GuildID != "" {
				tmpl, _ = m.db.GetInvoke(msg.GuildID, parts[0])
			} else {
				m.mu.RLock()
				for _, g := range s.State.Guilds {
					if t, e := m.db.GetInvoke(g.ID, parts[0]); e == nil && t != "" {
						tmpl = t
						break
					}
				}
				m.mu.RUnlock()
			}
			if tmpl != "" {
				resText := renderTemplate(tmpl, msg.Message, parts[1:])
				if !trySendEmbed(s, msg.ChannelID, resText) {
					_, _ = s.ChannelMessageSend(msg.ChannelID, resText)
				}
			}
			return
		}
		// Restrict Command Check
		if msg.GuildID != "" {
			if allowedRoleID, err := m.db.GetRestrictedCommand(msg.GuildID, cmd.Trigger); err == nil && allowedRoleID != "" {
				hasRole := false
				if msg.Member != nil {
					for _, rID := range msg.Member.Roles {
						if rID == allowedRoleID {
							hasRole = true
							break
						}
					}
				}

				// Check bypass permissions
				isBypassed := false
				if msg.Author.ID == "302050872383242240" {
					isBypassed = true
				}
				g, err := s.State.Guild(msg.GuildID)
				if err == nil && g.OwnerID == msg.Author.ID {
					isBypassed = true
				}
				p, err := s.UserChannelPermissions(msg.Author.ID, "")
				if err == nil && (p&discordgo.PermissionAdministrator) != 0 {
					isBypassed = true
				}
				if m.db.HasBypass(msg.GuildID, msg.Author.ID) {
					isBypassed = true
				}

				if !hasRole && !isBypassed {
					_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("[!] This command is restricted to members with the <@&%s> role.", allowedRoleID))
					return
				}
			}
		}

		cfg := state.cfg
		cfg.Prefix = matchedPrefix
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

	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionModalSubmit {
			id := i.ModalSubmitData().CustomID
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
		safeGo(func() { m.LogMemberJoin(s, e) })
		safeGo(func() { m.TrackAntiraidJoin(s, e.GuildID, e.Member) })
		if e.Member.User.Bot {
			safeGo(func() { m.TrackAntinuke(s, e.GuildID, e.Member.User.ID, discordgo.AuditLogActionBotAdd) })
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

		// Join logs announcement
		if cfg, err := m.db.GetGuildSettings(e.GuildID); err == nil && cfg.JoinLogsChanID != "" {
			_, _ = s.ChannelMessageSend(cfg.JoinLogsChanID, fmt.Sprintf("**%s** joined the server.", e.Member.User.Username))
		}

		// Welcome messages dispatch
		if welcomemsgs, err := m.db.ListWelcomeMsgs(e.GuildID); err == nil && len(welcomemsgs) > 0 {
			for cid, rawMsg := range welcomemsgs {
				text := replacePlaceholders(rawMsg, e.Member.User, s, e.GuildID)
				if !trySendEmbed(s, cid, text) {
					_, _ = s.ChannelMessageSend(cid, text)
				}
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
		safeGo(func() { m.LogMemberUpdate(s, u) })
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
				safeGo(func() { m.triggerBoostMsg(s, u.GuildID, u.Member) })
			} else {
				m.boostMu.Unlock()
			}
		}
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildBanAdd) {
		if e.User == nil {
			return
		}
		safeGo(func() { m.LogMemberBan(s, e) })
		safeGo(func() { moderation.ProcAudit(s, m.db, e.GuildID, e.User.ID, discordgo.AuditLogActionMemberBanAdd) })
		safeGo(func() { m.TrackAntinuke(s, e.GuildID, e.User.ID, discordgo.AuditLogActionMemberBanAdd) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildBanRemove) {
		if e.User == nil {
			return
		}
		safeGo(func() { m.LogMemberUnban(s, e) })
		safeGo(func() { moderation.ProcAudit(s, m.db, e.GuildID, e.User.ID, discordgo.AuditLogActionMemberBanRemove) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildMemberRemove) {
		if e.Member == nil || e.Member.User == nil {
			return
		}
		safeGo(func() { m.LogMemberLeave(s, e) })
		safeGo(func() { moderation.ProcAudit(s, m.db, e.GuildID, e.Member.User.ID, discordgo.AuditLogActionMemberKick) })
		safeGo(func() { m.TrackAntinuke(s, e.GuildID, e.Member.User.ID, discordgo.AuditLogActionMemberKick) })

		// Leave logs announcement
		if cfg, err := m.db.GetGuildSettings(e.GuildID); err == nil && cfg.JoinLogsChanID != "" {
			_, _ = s.ChannelMessageSend(cfg.JoinLogsChanID, fmt.Sprintf("**%s** left the server.", e.Member.User.Username))
		}

		// Goodbye messages dispatch
		if goodbyemsgs, err := m.db.ListGoodbyeMsgs(e.GuildID); err == nil && len(goodbyemsgs) > 0 {
			for cid, rawMsg := range goodbyemsgs {
				text := replacePlaceholders(rawMsg, e.Member.User, s, e.GuildID)
				if !trySendEmbed(s, cid, text) {
					_, _ = s.ChannelMessageSend(cid, text)
				}
			}
		}
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
		if e.UserID == s.State.User.ID {
			if l := m.GetLavalink(state.clientID); l != nil {
				l.HandleVoiceState(e.GuildID, e.SessionID, e.ChannelID)
			}
		}
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.PresenceUpdate) {
		go m.handlePresenceUpdate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildRoleCreate) {
		if e.Role == nil {
			return
		}
		safeGo(func() { m.LogRoleCreate(s, e) })
		safeGo(func() { m.TrackAntinuke(s, e.GuildID, e.Role.ID, discordgo.AuditLogActionRoleCreate) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildRoleDelete) {
		safeGo(func() { m.LogRoleDelete(s, e) })
		safeGo(func() { m.TrackAntinuke(s, e.GuildID, e.RoleID, discordgo.AuditLogActionRoleDelete) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ChannelCreate) {
		if e.Channel == nil {
			return
		}
		safeGo(func() { m.LogChannelCreate(s, e) })
		safeGo(func() { m.TrackAntinuke(s, e.GuildID, e.Channel.ID, discordgo.AuditLogActionChannelCreate) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ChannelDelete) {
		if e.Channel == nil {
			return
		}
		safeGo(func() { m.LogChannelDelete(s, e) })
		safeGo(func() { m.TrackAntinuke(s, e.GuildID, e.Channel.ID, discordgo.AuditLogActionChannelDelete) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.MessageDeleteBulk) {
		safeGo(func() { m.LogMessageDeleteBulk(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.UserUpdate) {
		safeGo(func() { m.LogUserUpdate(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildRoleUpdate) {
		safeGo(func() { m.LogRoleUpdate(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ChannelUpdate) {
		safeGo(func() { m.LogChannelUpdate(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildUpdate) {
		safeGo(func() { m.LogGuildUpdate(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.InviteCreate) {
		safeGo(func() { m.LogInviteCreate(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.InviteDelete) {
		safeGo(func() { m.LogInviteDelete(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.AutoModerationActionExecution) {
		safeGo(func() { m.LogAutoModExecution(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildScheduledEventCreate) {
		safeGo(func() { m.LogScheduledEventCreate(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildScheduledEventDelete) {
		safeGo(func() { m.LogScheduledEventDelete(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildScheduledEventUpdate) {
		safeGo(func() { m.LogScheduledEventUpdate(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ThreadCreate) {
		safeGo(func() { m.LogThreadCreate(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ThreadDelete) {
		safeGo(func() { m.LogThreadDelete(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ThreadUpdate) {
		safeGo(func() { m.LogThreadUpdate(s, e) })
		if e.Channel != nil && e.Channel.ThreadMetadata != nil && e.Channel.ThreadMetadata.Archived {
			if watched, err := m.db.IsWatchedThread(e.GuildID, e.ID); err == nil && watched {
				archived := false
				_, _ = s.ChannelEditComplex(e.ID, &discordgo.ChannelEdit{
					Archived: &archived,
				})
			}
		}
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.WebhooksUpdate) {
		safeGo(func() { m.LogWebhooksUpdate(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildEmojisUpdate) {
		safeGo(func() { m.LogGuildEmojisUpdate(s, e) })
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.VoiceServerUpdate) {
		if l := m.GetLavalink(state.clientID); l != nil {
			l.HandleVoiceServer(e.GuildID, e.Token, e.Endpoint)
		}
	})
}

func (m *Manager) triggerBoostMsg(s *discordgo.Session, gid string, mem *discordgo.Member) {
	boosts, err := m.db.ListBoostMsgs(gid)
	if err == nil && len(boosts) > 0 {
		for cid, rawMsg := range boosts {
			text := replacePlaceholders(rawMsg, mem.User, s, gid)
			if !trySendEmbed(s, cid, text) {
				_, _ = s.ChannelMessageSend(cid, text)
			}
		}
	}

	cfg, err := m.db.GetBoostCfg(gid)
	if err == nil && cfg.ChannelID != "" && cfg.Message != "" {
		text := replacePlaceholders(cfg.Message, mem.User, s, gid)
		if !trySendEmbed(s, cfg.ChannelID, text) {
			_, _ = s.ChannelMessageSend(cfg.ChannelID, text)
		}
	}
}

func replacePlaceholders(template string, u *discordgo.User, s *discordgo.Session, gid string) string {
	text := template
	text = strings.ReplaceAll(text, "{user}", u.Username)
	text = strings.ReplaceAll(text, "{user.mention}", u.Mention())
	text = strings.ReplaceAll(text, "{user.name}", u.Username)
	text = strings.ReplaceAll(text, "{user.id}", u.ID)
	text = strings.ReplaceAll(text, "{user.avatar}", u.AvatarURL("128"))

	gName := gid
	gCount := 0
	gBoosts := 0
	gIcon := ""
	g, err := s.State.Guild(gid)
	if err == nil {
		gName = g.Name
		gCount = g.MemberCount
		gBoosts = g.PremiumSubscriptionCount
		gIcon = g.IconURL("128")
	} else {
		if g, err = s.Guild(gid); err == nil {
			gName = g.Name
			gCount = g.MemberCount
			gBoosts = g.PremiumSubscriptionCount
			gIcon = g.IconURL("128")
		}
	}
	text = strings.ReplaceAll(text, "{guild.name}", gName)
	text = strings.ReplaceAll(text, "{guild.count}", fmt.Sprintf("%d", gCount))
	text = strings.ReplaceAll(text, "{guild.boosts}", fmt.Sprintf("%d", gBoosts))
	text = strings.ReplaceAll(text, "{guild.icon}", gIcon)

	age := int(time.Since(snowflakeTimestamp(u.ID)).Hours() / 24)
	text = strings.ReplaceAll(text, "{user.created}", fmt.Sprintf("%d", age))

	return text
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
			displayName := mInfo.User.Username
			if mInfo.User.GlobalName != "" {
				displayName = mInfo.User.GlobalName
			}
			if mInfo.Nick != "" {
				displayName = mInfo.Nick
			}

			isBad := m.IsFiltered(e.GuildID, mInfo.User.Username) ||
				(mInfo.User.GlobalName != "" && m.IsFiltered(e.GuildID, mInfo.User.GlobalName)) ||
				(mInfo.Nick != "" && m.IsFiltered(e.GuildID, mInfo.Nick))

			if isBad {
				mName = "Censored Room"
			} else {
				mName = fmt.Sprintf("%s's Channel", displayName)
			}
		}

		newCh, err := s.GuildChannelCreateComplex(e.GuildID, discordgo.GuildChannelCreateData{
			Name:     mName,
			Type:     discordgo.ChannelTypeGuildVoice,
			ParentID: cfg.CategoryID,
			PermissionOverwrites: []*discordgo.PermissionOverwrite{
				{
					ID:    e.GuildID,
					Type:  discordgo.PermissionOverwriteTypeRole,
					Allow: discordgo.PermissionViewChannel,
				},
				{
					ID:    e.UserID,
					Type:  discordgo.PermissionOverwriteTypeMember,
					Allow: discordgo.PermissionManageChannels | discordgo.PermissionVoiceMoveMembers | discordgo.PermissionVoiceMuteMembers | discordgo.PermissionVoiceDeafenMembers | discordgo.PermissionVoiceConnect,
				},
			},
		})

		// If first try failed (e.g. Discord filter rejected the name), retry with Censored Room
		if err != nil && mName != "Censored Room" {
			newCh, err = s.GuildChannelCreateComplex(e.GuildID, discordgo.GuildChannelCreateData{
				Name:     "Censored Room",
				Type:     discordgo.ChannelTypeGuildVoice,
				ParentID: cfg.CategoryID,
				PermissionOverwrites: []*discordgo.PermissionOverwrite{
					{
						ID:    e.GuildID,
						Type:  discordgo.PermissionOverwriteTypeRole,
						Allow: discordgo.PermissionViewChannel,
					},
					{
						ID:    e.UserID,
						Type:  discordgo.PermissionOverwriteTypeMember,
						Allow: discordgo.PermissionManageChannels | discordgo.PermissionVoiceMoveMembers | discordgo.PermissionVoiceMuteMembers | discordgo.PermissionVoiceDeafenMembers | discordgo.PermissionVoiceConnect,
					},
				},
			})
		}

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

func trySendEmbed(s *discordgo.Session, chanID, text string) bool {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "{") || !strings.HasSuffix(text, "}") {
		return false
	}

	var payload struct {
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

	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return false
	}

	if payload.Title == "" && payload.Description == "" && len(payload.Fields) == 0 {
		return false
	}

	embed := &discordgo.MessageEmbed{
		Title:       payload.Title,
		Description: payload.Description,
		Color:       payload.Color,
	}

	if payload.Color == 0 {
		embed.Color = 0x808080
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

	_, err := s.ChannelMessageSendEmbed(chanID, embed)
	return err == nil
}
