package manager

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"skyvern/internal/config"
	"skyvern/internal/moderation"
	"skyvern/internal/storage"

	"github.com/bwmarrin/discordgo"
	bolt "go.etcd.io/bbolt"
	"regexp"
	"runtime"
	"runtime/debug"
)

type CommandContext struct {
	Session  *discordgo.Session
	Message  *discordgo.Message
	Interact *discordgo.Interaction
	Args     []string
	Cfg      config.ResCfg
	DB       *storage.DB
	ClientID string
	Mgr      *Manager
}

type Command struct {
	Trigger     string
	Aliases     []string
	Name        string
	Description string
	Category    string
	Execute     func(ctx *CommandContext) error
}

func (ctx *CommandContext) Respond(embed *discordgo.MessageEmbed) error {
	if ctx.Interact != nil {
		return ctx.Session.InteractionRespond(ctx.Interact, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}},
		})
	}
	_, err := ctx.Session.ChannelMessageSendEmbed(ctx.Message.ChannelID, embed)
	return err
}

func (ctx *CommandContext) GuildID() string {
	if ctx.Interact != nil {
		return ctx.Interact.GuildID
	}
	if ctx.Message != nil {
		return ctx.Message.GuildID
	}
	return ""
}

func (ctx *CommandContext) ChanID() string {
	if ctx.Interact != nil {
		return ctx.Interact.ChannelID
	}
	if ctx.Message != nil {
		return ctx.Message.ChannelID
	}
	return ""
}

func (ctx *CommandContext) AuthorID() string {
	if ctx.Interact != nil && ctx.Interact.Member != nil && ctx.Interact.Member.User != nil {
		return ctx.Interact.Member.User.ID
	}
	if ctx.Message != nil && ctx.Message.Author != nil {
		return ctx.Message.Author.ID
	}
	return ""
}

func (ctx *CommandContext) AuthorTag() string {
	if ctx.Interact != nil && ctx.Interact.Member != nil && ctx.Interact.Member.User != nil {
		return ctx.Interact.Member.User.Username
	}
	if ctx.Message != nil && ctx.Message.Author != nil {
		return ctx.Message.Author.Username
	}
	return "Unknown"
}

func (ctx *CommandContext) Reply(text string) error {
	return ctx.Respond(config.Wrap(ctx.Cfg, text))
}

func (ctx *CommandContext) Ban(uid, reason string, days int) error {
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), reason)
	return ctx.Session.GuildBanCreateWithReason(ctx.GuildID(), uid, auditReason, days)
}

func (ctx *CommandContext) Unban(uid string, reason ...string) error {
	r := "Manual unban"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.GuildBanDelete(ctx.GuildID(), uid, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) Kick(uid string, reason ...string) error {
	r := "Manual kick"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.GuildMemberDelete(ctx.GuildID(), uid, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) Timeout(uid string, until *time.Time, reason ...string) error {
	r := "Manual timeout"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.GuildMemberTimeout(ctx.GuildID(), uid, until, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) Nick(uid, nick string, reason ...string) error {
	r := "Manual nickname update"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.GuildMemberNickname(ctx.GuildID(), uid, nick, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) ChannelPermissionSet(chID, targetID string, targetType discordgo.PermissionOverwriteType, allowVal, denyVal int64, reason ...string) error {
	r := "Update channel permissions"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.ChannelPermissionSet(chID, targetID, targetType, allowVal, denyVal, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) ChannelPermissionDelete(chID, targetID string, reason ...string) error {
	r := "Delete channel permissions override"
	if len(reason) > 0 && reason[0] != "" {
		r = reason[0]
	}
	auditReason := fmt.Sprintf("Forced by %s (%s) | Reason: %s", ctx.AuthorTag(), ctx.AuthorID(), r)
	return ctx.Session.ChannelPermissionDelete(chID, targetID, discordgo.WithAuditLogReason(auditReason))
}

func (ctx *CommandContext) Delete(msgID string) error {
	return ctx.Session.ChannelMessageDelete(ctx.ChanID(), msgID)
}

func (ctx *CommandContext) BulkDelete(ids []string) error {
	return ctx.Session.ChannelMessagesBulkDelete(ctx.ChanID(), ids)
}

type instState struct {
	cfg      config.ResCfg
	session  *discordgo.Session
	clientID string
	running  bool
}

type tracker struct {
	mu   sync.RWMutex
	data map[string]*storage.Analytics
}

func (t *tracker) get(id string) storage.Analytics {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if a, ok := t.data[id]; ok {
		return *a
	}
	return storage.Analytics{}
}

func (t *tracker) incPrefix(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	a := t.ensure(id)
	a.PrefixCmds++
	a.TotalCmds++
}

func (t *tracker) incSlash(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	a := t.ensure(id)
	a.SlashCmds++
	a.TotalCmds++
}

func (t *tracker) setGuilds(id string, n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensure(id).GuildCount = n
}

func (t *tracker) totals() storage.Analytics {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out storage.Analytics
	for _, a := range t.data {
		out.TotalCmds += a.TotalCmds
		out.PrefixCmds += a.PrefixCmds
		out.SlashCmds += a.SlashCmds
		out.GuildCount += a.GuildCount
	}
	return out
}

func (t *tracker) ensure(id string) *storage.Analytics {
	if t.data == nil {
		t.data = make(map[string]*storage.Analytics)
	}
	if t.data[id] == nil {
		t.data[id] = &storage.Analytics{}
	}
	return t.data[id]
}

type Manager struct {
	db       *storage.DB
	commands []*Command

	mu        sync.RWMutex
	instances map[string]*instState

	stats     *tracker
	stopFlush chan struct{}

	compHandlers map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)

	lastDailyQuestionDate map[string]string
	lastDailyQuoteDate    map[string]string
	dailyMu               sync.Mutex

	lastBoostLogged       map[string]time.Time
	boostMu               sync.Mutex

	reminders             []storage.Reminder
	schedules             []storage.ScheduledMsg
	remindersMu           sync.RWMutex
	schedulesMu           sync.RWMutex

	antispamTracker       map[string][]time.Time
	antispamMu            sync.Mutex

	palantirDB            *bolt.DB
	antispamCache         map[string]storage.AntispamCfg
	filterCache           map[string]storage.FilterCfg
	antilinkCache         map[string]storage.AntilinkCfg
	prefixCache           map[string]string
	palantirCache         storage.PalantirCfg
	palantirCacheInit     bool
	configMu              sync.RWMutex

	palantirChan          chan *PalantirLog
	palantirWG            sync.WaitGroup
}

func New(db *storage.DB, cmds []*Command) *Manager {
	rems, _ := db.ListAllReminders()
	schs, _ := db.ListAllSchedules()
	m := &Manager{
		db:                    db,
		commands:              cmds,
		instances:             make(map[string]*instState),
		stats:                 &tracker{},
		stopFlush:             make(chan struct{}),
		compHandlers:          make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)),
		lastDailyQuestionDate: make(map[string]string),
		lastDailyQuoteDate:    make(map[string]string),
		lastBoostLogged:       make(map[string]time.Time),
		antispamTracker:       make(map[string][]time.Time),
		antispamCache:         make(map[string]storage.AntispamCfg),
		filterCache:           make(map[string]storage.FilterCfg),
		antilinkCache:         make(map[string]storage.AntilinkCfg),
		prefixCache:           make(map[string]string),
		palantirChan:          make(chan *PalantirLog, 1000),
		reminders:             rems,
		schedules:             schs,
	}
	palDb, err := bolt.Open(config.ResolvePath("palantir.db"), 0600, nil)
	if err == nil {
		_ = palDb.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte("AuditLogs"))
			return err
		})
		m.palantirDB = palDb
	}
	go m.palantirWriterLoop()
	go m.flushLoop()
	go m.tempRoleLoop()
	go m.dailySchedulerLoop()
	go m.birthdayLoop()
	go m.remindLoop()
	go m.scheduleLoop()
	go m.gcLoop()
	return m
}

func (m *Manager) RegisterComponentHandler(customID string, fn func(s *discordgo.Session, i *discordgo.InteractionCreate)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.compHandlers == nil {
		m.compHandlers = make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate))
	}
	m.compHandlers[customID] = fn
}

func (m *Manager) Close() {
	close(m.stopFlush)
	if m.palantirChan != nil {
		close(m.palantirChan)
	}
	m.palantirWG.Wait()
	if m.palantirDB != nil {
		_ = m.palantirDB.Close()
	}
}

func (m *Manager) AddCommands(cmds []*Command) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands = append(m.commands, cmds...)
}

func (m *Manager) Start(clientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.instances[clientID]; ok && s.running {
		return nil
	}

	inst, err := m.db.GetBot(clientID)
	if err != nil {
		return fmt.Errorf("load bot: %w", err)
	}
	cfg := config.Resolve(config.GetGlobal(), inst)

	sess, err := discordgo.New("Bot " + inst.Token)
	if err != nil {
		return fmt.Errorf("discordgo init: %w", err)
	}
	sess.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuilds | discordgo.IntentMessageContent | discordgo.IntentsGuildMembers | discordgo.IntentsGuildMessageReactions
	sess.State.MaxMessageCount = 1000

	state := &instState{cfg: cfg, session: sess, clientID: clientID}
	m.instances[clientID] = state
	m.attachHandlers(sess, state)

	if err := sess.Open(); err != nil {
		delete(m.instances, clientID)
		return fmt.Errorf("session open: %w", err)
	}

	state.running = true

	if cfg.AvatarURL != "" {
		go m.updateAvatar(sess, cfg.AvatarURL, clientID)
	}
	go m.registerSlashCmds(sess, clientID)

	m.stats.setGuilds(clientID, len(sess.State.Guilds))
	return nil
}

func (m *Manager) Stop(clientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.instances[clientID]
	if !ok || !s.running {
		return nil
	}
	if err := s.session.Close(); err != nil {
		return fmt.Errorf("session close: %w", err)
	}
	s.running = false
	return nil
}

func (m *Manager) IsRunning(clientID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.instances[clientID]
	return ok && s.running
}

func (m *Manager) Stats(clientID string) storage.Analytics    { return m.stats.get(clientID) }
func (m *Manager) GlobalStats() storage.Analytics             { return m.stats.totals() }

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
		if gp, err := m.db.GetPrefix(msg.GuildID); err == nil && gp != "" {
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
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildBanRemove) {
		go m.LogMemberUnban(s, e)
		go moderation.ProcAudit(s, m.db, e.GuildID, e.User.ID, discordgo.AuditLogActionMemberBanRemove)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildMemberRemove) {
		go m.LogMemberLeave(s, e)
		go moderation.ProcAudit(s, m.db, e.GuildID, e.Member.User.ID, discordgo.AuditLogActionMemberKick)
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
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.GuildRoleDelete) {
		go m.LogRoleDelete(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ChannelCreate) {
		go m.LogChannelCreate(s, e)
	})

	sess.AddHandler(func(s *discordgo.Session, e *discordgo.ChannelDelete) {
		go m.LogChannelDelete(s, e)
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

func (m *Manager) registerSlashCmds(sess *discordgo.Session, clientID string) {
	time.Sleep(500 * time.Millisecond)
	appID := sess.State.User.ID
	cmds := make([]*discordgo.ApplicationCommand, 0, len(m.commands))
	for _, c := range m.commands {
		cmds = append(cmds, &discordgo.ApplicationCommand{
			Name:        c.Name,
			Description: c.Description,
		})
	}
	if _, err := sess.ApplicationCommandBulkOverwrite(appID, "", cmds); err != nil {
		fmt.Printf("[%s] slash bulk reg: %v\n", clientID, err)
	}
}

func (m *Manager) updateAvatar(sess *discordgo.Session, url, clientID string) {
	// url comes from admin config panel, not user input — still, don't love this
	resp, err := http.Get(url) // #nosec G107
	if err != nil {
		fmt.Printf("[%s] avatar: %v\n", clientID, err)
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[%s] avatar read: %v\n", clientID, err)
		return
	}
	ct := http.DetectContentType(data)
	if _, err := sess.UserUpdate("", "data:"+ct+";base64,"+base64.StdEncoding.EncodeToString(data), ""); err != nil {
		fmt.Printf("[%s] avatar set: %v\n", clientID, err)
	}
}

func (m *Manager) flushLoop() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			m.flushAnalytics()
		case <-m.stopFlush:
			m.flushAnalytics()
			return
		}
	}
}

func (m *Manager) flushAnalytics() {
	m.stats.mu.RLock()
	snap := make(map[string]storage.Analytics, len(m.stats.data))
	for k, v := range m.stats.data {
		snap[k] = *v
	}
	m.stats.mu.RUnlock()

	for id, a := range snap {
		if err := m.db.SaveAnalytics(id, a); err != nil {
			fmt.Printf("flush %q: %v\n", id, err)
		}
	}
}

func (m *Manager) findByTrigger(trigger string) *Command {
	for _, c := range m.commands {
		if strings.EqualFold(c.Trigger, trigger) {
			return c
		}
		for _, a := range c.Aliases {
			if strings.EqualFold(a, trigger) {
				return c
			}
		}
	}
	return nil
}

func (m *Manager) findByName(name string) *Command {
	for _, c := range m.commands {
		if strings.EqualFold(c.Name, name) {
			return c
		}
	}
	return nil
}

func (m *Manager) UpdateInstance(cid string) error {
	run := m.IsRunning(cid)
	if run {
		if err := m.Stop(cid); err != nil {
			return err
		}
	}
	m.mu.Lock()
	delete(m.instances, cid)
	m.mu.Unlock()

	if run {
		return m.Start(cid)
	}
	return nil
}

func (m *Manager) Snapshot() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]bool, len(m.instances))
	for id, s := range m.instances {
		out[id] = s.running
	}
	return out
}

func (m *Manager) ResolvedCfgFor(cid string) (config.ResCfg, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.instances[cid]; ok {
		return s.cfg, true
	}
	return config.ResCfg{}, false
}

func (m *Manager) Commands() []*Command {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Command, len(m.commands))
	copy(out, m.commands)
	return out
}

func renderTemplate(tmpl string, msg *discordgo.Message, args []string) string {
	var pairs []string
	if msg.Author != nil {
		pairs = append(pairs, "{user.name}", msg.Author.Username, "{user.mention}", "<@"+msg.Author.ID+">")
	}
	pairs = append(pairs, "{channel.mention}", "<#"+msg.ChannelID+">")
	if strings.Contains(tmpl, "{args}") {
		pairs = append(pairs, "{args}", strings.Join(args, " "))
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}

func (m *Manager) tempRoleLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			roles, err := m.db.GetExpiredTempRoles()
			if err != nil || len(roles) == 0 {
				continue
			}
			m.mu.RLock()
			var activeSess *discordgo.Session
			for _, inst := range m.instances {
				if inst.running {
					activeSess = inst.session
					break
				}
			}
			m.mu.RUnlock()

			if activeSess == nil {
				continue
			}

			for _, tr := range roles {
				_ = activeSess.GuildMemberRoleRemove(tr.GuildID, tr.UserID, tr.RoleID)
				_ = m.db.DeleteTempRole(tr.GuildID, tr.UserID, tr.RoleID)
			}
		case <-m.stopFlush:
			return
		}
	}
}

var dailyQuestions = []string{
	"What is the most interesting thing you read or watched this week?",
	"If you could have any superpower, what would it be and why?",
	"What is your go-to comfort food?",
	"What's the best piece of advice you've ever received?",
	"If you could travel back in time, which decade would you visit?",
	"What's your favorite hobby or way to pass the time?",
	"What is one goal you want to achieve this month?",
	"What is the last song you listened to?",
	"What is your favorite book or movie of all time?",
	"If you could have dinner with any historical figure, who would it be?",
}

var dailyQuotes = []string{
	"The only way to do great work is to love what you do. - Steve Jobs",
	"Believe you can and you're halfway there. - Theodore Roosevelt",
	"It always seems impossible until it's done. - Nelson Mandela",
	"Success is not final, failure is not fatal: it is the courage to continue that counts. - Winston Churchill",
	"Act as if what you do makes a difference. It does. - William James",
	"The future belongs to those who believe in the beauty of their dreams. - Eleanor Roosevelt",
	"Do what you can, with what you have, where you are. - Theodore Roosevelt",
	"Keep your face always toward the sunshine - and shadows will fall behind you. - Walt Whitman",
	"You miss 100% of the shots you don't take. - Wayne Gretzky",
	"The only limit to our realization of tomorrow will be our doubts of today. - Franklin D. Roosevelt",
}

func (m *Manager) dailySchedulerLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			timeStr := now.Format("15:04")
			dateStr := now.Format("2006-01-02")

			qCfgs, err := m.db.ListDailyQuestions()
			if err == nil {
				for gid, cfg := range qCfgs {
					if !cfg.Enabled || cfg.ChannelID == "" || cfg.Time != timeStr {
						continue
					}
					m.dailyMu.Lock()
					lastDate := m.lastDailyQuestionDate[gid]
					if lastDate == dateStr {
						m.dailyMu.Unlock()
						continue
					}
					m.lastDailyQuestionDate[gid] = dateStr
					m.dailyMu.Unlock()

					m.mu.RLock()
					var s *discordgo.Session
					for _, inst := range m.instances {
						if inst.running {
							s = inst.session
							break
						}
					}
					m.mu.RUnlock()

					if s != nil {
						q := fetchDailyQuestion()
						_, _ = s.ChannelMessageSend(cfg.ChannelID, fmt.Sprintf("**Daily Question:** %s", q))
					}
				}
			}

			qList, err := m.db.ListDailyQuotes()
			if err == nil {
				for gid, cfg := range qList {
					if !cfg.Enabled || cfg.ChannelID == "" || cfg.Time != timeStr {
						continue
					}
					m.dailyMu.Lock()
					lastDate := m.lastDailyQuoteDate[gid]
					if lastDate == dateStr {
						m.dailyMu.Unlock()
						continue
					}
					m.lastDailyQuoteDate[gid] = dateStr
					m.dailyMu.Unlock()

					m.mu.RLock()
					var s *discordgo.Session
					for _, inst := range m.instances {
						if inst.running {
							s = inst.session
							break
						}
					}
					m.mu.RUnlock()

					if s != nil {
						quote := fetchDailyQuote()
						_, _ = s.ChannelMessageSend(cfg.ChannelID, fmt.Sprintf("**Daily Quote:** %s", quote))
					}
				}
			}

		case <-m.stopFlush:
			return
		}
	}
}

func fetchDailyQuestion() string {
	resp, err := http.Get("https://api.truthordarebot.xyz/api/truth")
	if err == nil {
		defer resp.Body.Close()
		var res struct {
			Question string `json:"question"`
		}
		if json.NewDecoder(resp.Body).Decode(&res) == nil && res.Question != "" {
			return res.Question
		}
	}
	return dailyQuestions[time.Now().UnixNano()%int64(len(dailyQuestions))]
}

func fetchDailyQuote() string {
	resp, err := http.Get("https://zenquotes.io/api/random")
	if err == nil {
		defer resp.Body.Close()
		var res []struct {
			Q string `json:"q"`
			A string `json:"a"`
		}
		if json.NewDecoder(resp.Body).Decode(&res) == nil && len(res) > 0 && res[0].Q != "" {
			return fmt.Sprintf("*\"%s\"* - %s", res[0].Q, res[0].A)
		}
	}
	return fmt.Sprintf("*\"%s\"*", dailyQuotes[time.Now().UnixNano()%int64(len(dailyQuotes))])
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

func (m *Manager) birthdayLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.mu.RLock()
			var sess *discordgo.Session
			for _, inst := range m.instances {
				if inst.running {
					sess = inst.session
					break
				}
			}
			m.mu.RUnlock()

			if sess == nil {
				continue
			}

			sess.State.RLock()
			var gids []string
			for _, g := range sess.State.Guilds {
				gids = append(gids, g.ID)
			}
			sess.State.RUnlock()

			for _, gid := range gids {
				chID, err := m.db.GetBirthdayChannel(gid)
				if err != nil || chID == "" {
					continue
				}

				birthdays, err := m.db.ListBirthdays(gid)
				if err != nil || len(birthdays) == 0 {
					continue
				}

				for uid, bday := range birthdays {
					tzName, _ := m.db.GetTimezone(uid)
					var loc *time.Location
					if tzName != "" {
						if l, err := time.LoadLocation(tzName); err == nil {
							loc = l
						}
					}
					if loc == nil {
						loc = time.Local
					}

					nowInTZ := time.Now().In(loc)
					if nowInTZ.Format("01/02") == bday {
						lastWished, err := m.db.GetLastBirthdayWished(gid, uid)
						if err != nil || lastWished != nowInTZ.Year() {
							_ = m.db.SaveLastBirthdayWished(gid, uid, nowInTZ.Year())
							_, _ = sess.ChannelMessageSend(chID, fmt.Sprintf("🎂 Happy Birthday <@%s>! Wishing you a fantastic day! 🎉", uid))
						}
					}
				}
			}

		case <-m.stopFlush:
			return
		}
	}
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

func (m *Manager) DB() *storage.DB { return m.db }

func (m *Manager) remindLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.remindersMu.Lock()
			if len(m.reminders) == 0 {
				m.remindersMu.Unlock()
				continue
			}
			m.mu.RLock()
			var sess *discordgo.Session
			for _, inst := range m.instances {
				if inst.running {
					sess = inst.session
					break
				}
			}
			m.mu.RUnlock()
			if sess == nil {
				m.remindersMu.Unlock()
				continue
			}
			now := time.Now()
			var active []storage.Reminder
			for _, r := range m.reminders {
				if r.Time.Before(now) {
					go func(rem storage.Reminder, s *discordgo.Session) {
						dm, err := s.UserChannelCreate(rem.UserID)
						if err == nil {
							_, _ = s.ChannelMessageSend(dm.ID, fmt.Sprintf("⏰ **Reminder:** %s", rem.Message))
						}
						_ = m.db.DeleteReminder(rem.UserID, rem.ID)
					}(r, sess)
				} else {
					active = append(active, r)
				}
			}
			m.reminders = active
			m.remindersMu.Unlock()
		case <-m.stopFlush:
			return
		}
	}
}

func (m *Manager) scheduleLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.schedulesMu.Lock()
			if len(m.schedules) == 0 {
				m.schedulesMu.Unlock()
				continue
			}
			m.mu.RLock()
			var sess *discordgo.Session
			for _, inst := range m.instances {
				if inst.running {
					sess = inst.session
					break
				}
			}
			m.mu.RUnlock()
			if sess == nil {
				m.schedulesMu.Unlock()
				continue
			}
			now := time.Now()
			var active []storage.ScheduledMsg
			for _, sch := range m.schedules {
				if sch.Time.Before(now) {
					go func(sc storage.ScheduledMsg, ss *discordgo.Session) {
						_, _ = ss.ChannelMessageSend(sc.ChannelID, sc.Message)
						_ = m.db.DeleteSchedule(sc.GuildID, sc.ID)
					}(sch, sess)
				} else {
					active = append(active, sch)
				}
			}
			m.schedules = active
			m.schedulesMu.Unlock()
		case <-m.stopFlush:
			return
		}
	}
}

func (m *Manager) gcLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			runtime.GC()
			debug.FreeOSMemory()
		case <-m.stopFlush:
			return
		}
	}
}

func (m *Manager) ListReminders(uid string) []storage.Reminder {
	m.remindersMu.RLock()
	defer m.remindersMu.RUnlock()
	var out []storage.Reminder
	for _, r := range m.reminders {
		if r.UserID == uid {
			out = append(out, r)
		}
	}
	return out
}

func (m *Manager) SaveReminder(r storage.Reminder) error {
	if err := m.db.SaveReminder(r); err != nil {
		return err
	}
	m.remindersMu.Lock()
	defer m.remindersMu.Unlock()
	for i, x := range m.reminders {
		if x.UserID == r.UserID && x.ID == r.ID {
			m.reminders[i] = r
			return nil
		}
	}
	m.reminders = append(m.reminders, r)
	return nil
}

func (m *Manager) DeleteReminder(uid, id string) error {
	if err := m.db.DeleteReminder(uid, id); err != nil {
		return err
	}
	m.remindersMu.Lock()
	defer m.remindersMu.Unlock()
	for i, x := range m.reminders {
		if x.UserID == uid && x.ID == id {
			m.reminders = append(m.reminders[:i], m.reminders[i+1:]...)
			break
		}
	}
	return nil
}

func (m *Manager) ListSchedules(gid string) []storage.ScheduledMsg {
	m.schedulesMu.RLock()
	defer m.schedulesMu.RUnlock()
	var out []storage.ScheduledMsg
	for _, s := range m.schedules {
		if s.GuildID == gid {
			out = append(out, s)
		}
	}
	return out
}

func (m *Manager) SaveSchedule(s storage.ScheduledMsg) error {
	if err := m.db.SaveSchedule(s); err != nil {
		return err
	}
	m.schedulesMu.Lock()
	defer m.schedulesMu.Unlock()
	for i, x := range m.schedules {
		if x.GuildID == s.GuildID && x.ID == s.ID {
			m.schedules[i] = s
			return nil
		}
	}
	m.schedules = append(m.schedules, s)
	return nil
}

func (m *Manager) DeleteSchedule(gid, id string) error {
	if err := m.db.DeleteSchedule(gid, id); err != nil {
		return err
	}
	m.schedulesMu.Lock()
	defer m.schedulesMu.Unlock()
	for i, x := range m.schedules {
		if x.GuildID == gid && x.ID == id {
			m.schedules = append(m.schedules[:i], m.schedules[i+1:]...)
			break
		}
	}
	return nil
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

func (m *Manager) checkAntispam(s *discordgo.Session, msg *discordgo.MessageCreate) bool {
	if msg.GuildID == "" || msg.Author == nil {
		return false
	}
	cfg, err := m.db.GetAntispamCfg(msg.GuildID)
	if err != nil || !cfg.Enabled || cfg.Limit <= 0 || cfg.Seconds <= 0 {
		return false
	}
	p, err := s.UserChannelPermissions(msg.Author.ID, msg.ChannelID)
	if err == nil && cfg.BypassPerms && (p&(discordgo.PermissionManageGuild|discordgo.PermissionAdministrator)) != 0 {
		return false
	}
	for _, id := range cfg.Whitelist {
		if id == msg.Author.ID || id == msg.ChannelID {
			return false
		}
		if msg.Member != nil {
			for _, rID := range msg.Member.Roles {
				if rID == id {
					return false
				}
			}
		}
	}
	now := time.Now()
	m.antispamMu.Lock()
	if m.antispamTracker == nil {
		m.antispamTracker = make(map[string][]time.Time)
	}
	key := msg.GuildID + ":" + msg.Author.ID
	timestamps := m.antispamTracker[key]
	cutoff := now.Add(-time.Duration(cfg.Seconds) * time.Second)
	var active []time.Time
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			active = append(active, ts)
		}
	}
	active = append(active, now)
	m.antispamTracker[key] = active
	m.antispamMu.Unlock()
	if len(active) > cfg.Limit {
		reason := fmt.Sprintf("Anti-spam: exceeded limit of %d messages in %d seconds", cfg.Limit, cfg.Seconds)
		switch cfg.Action {
		case "timeout":
			dur := time.Duration(cfg.TimeoutSecs) * time.Second
			until := now.Add(dur)
			auditReason := fmt.Sprintf("Anti-Spam | Reason: %s", reason)
			_ = s.GuildMemberTimeout(msg.GuildID, msg.Author.ID, &until, discordgo.WithAuditLogReason(auditReason))
			_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("[!] <@%s> has been timed out for %s for spamming.", msg.Author.ID, dur.String()))
		case "kick":
			auditReason := fmt.Sprintf("Anti-Spam | Reason: %s", reason)
			_ = s.GuildMemberDelete(msg.GuildID, msg.Author.ID, discordgo.WithAuditLogReason(auditReason))
			_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("[!] <@%s> has been kicked for spamming.", msg.Author.ID))
		case "ban":
			auditReason := fmt.Sprintf("Anti-Spam | Reason: %s", reason)
			_ = s.GuildBanCreateWithReason(msg.GuildID, msg.Author.ID, auditReason, 0)
			_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("[!] <@%s> has been banned for spamming.", msg.Author.ID))
		}
		return true
	}
	return false
}

func (m *Manager) checkFilter(s *discordgo.Session, msg *discordgo.MessageCreate) bool {
	if msg.GuildID == "" || msg.Author == nil {
		return false
	}
	cfg, err := m.db.GetFilterCfg(msg.GuildID)
	if err != nil || !cfg.Enabled {
		return false
	}
	p, err := s.UserChannelPermissions(msg.Author.ID, msg.ChannelID)
	if err == nil && cfg.BypassPerms && (p&(discordgo.PermissionManageMessages|discordgo.PermissionManageGuild|discordgo.PermissionAdministrator)) != 0 {
		return false
	}
	for _, id := range cfg.Whitelist {
		if id == msg.Author.ID || id == msg.ChannelID {
			return false
		}
		if msg.Member != nil {
			for _, rID := range msg.Member.Roles {
				if rID == id {
					return false
				}
			}
		}
	}
	content := strings.ToLower(msg.Content)
	hasViolation := false
	for _, w := range cfg.BlockedWords {
		if containsBypass(content, w) {
			hasViolation = true
			break
		}
	}
	if !hasViolation {
		for _, rxStr := range cfg.Regexes {
			if rxStr != "" {
				if re, err := regexp.Compile(rxStr); err == nil {
					if re.MatchString(msg.Content) {
						hasViolation = true
						break
					}
				}
			}
		}
	}
	if hasViolation {
		for _, aw := range cfg.AllowedWords {
			lowAW := strings.ToLower(aw)
			if lowAW != "" && strings.Contains(content, lowAW) {
				return false
			}
		}
		_ = s.ChannelMessageDelete(msg.ChannelID, msg.ID)
		_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("[!] <@%s>, your message contained blocked content and was removed.", msg.Author.ID))
		return true
	}
	return false
}

type PalantirLog struct {
	Timestamp time.Time `json:"timestamp"`
	GuildID   string    `json:"guild_id"`
	Category  string    `json:"category"`
	Title     string    `json:"title"`
	Desc      string    `json:"description"`
	UserID    string    `json:"user_id"`
	ChannelID string    `json:"channel_id"`
}

func (m *Manager) GetPrefix(gid string) (string, error) {
	m.configMu.RLock()
	p, ok := m.prefixCache[gid]
	m.configMu.RUnlock()
	if ok {
		return p, nil
	}
	p, err := m.db.GetPrefix(gid)
	if err == nil {
		m.configMu.Lock()
		m.prefixCache[gid] = p
		m.configMu.Unlock()
	}
	return p, err
}

func (m *Manager) SavePrefix(gid, prefix string) error {
	err := m.db.SavePrefix(gid, prefix)
	if err == nil {
		m.configMu.Lock()
		m.prefixCache[gid] = prefix
		m.configMu.Unlock()
	}
	return err
}

func (m *Manager) DeletePrefix(gid string) error {
	err := m.db.DeletePrefix(gid)
	if err == nil {
		m.configMu.Lock()
		delete(m.prefixCache, gid)
		m.configMu.Unlock()
	}
	return err
}

func (m *Manager) GetAntispamCfg(gid string) (storage.AntispamCfg, error) {
	m.configMu.RLock()
	cfg, ok := m.antispamCache[gid]
	m.configMu.RUnlock()
	if ok {
		return cfg, nil
	}
	cfg, err := m.db.GetAntispamCfg(gid)
	if err == nil {
		m.configMu.Lock()
		m.antispamCache[gid] = cfg
		m.configMu.Unlock()
	}
	return cfg, err
}

func (m *Manager) SaveAntispamCfg(gid string, cfg storage.AntispamCfg) error {
	err := m.db.SaveAntispamCfg(gid, cfg)
	if err == nil {
		m.configMu.Lock()
		m.antispamCache[gid] = cfg
		m.configMu.Unlock()
	}
	return err
}

func (m *Manager) GetFilterCfg(gid string) (storage.FilterCfg, error) {
	m.configMu.RLock()
	cfg, ok := m.filterCache[gid]
	m.configMu.RUnlock()
	if ok {
		return cfg, nil
	}
	cfg, err := m.db.GetFilterCfg(gid)
	if err == nil {
		m.configMu.Lock()
		m.filterCache[gid] = cfg
		m.configMu.Unlock()
	}
	return cfg, err
}

func (m *Manager) SaveFilterCfg(gid string, cfg storage.FilterCfg) error {
	err := m.db.SaveFilterCfg(gid, cfg)
	if err == nil {
		m.configMu.Lock()
		m.filterCache[gid] = cfg
		m.configMu.Unlock()
	}
	return err
}

func (m *Manager) GetAntilinkCfg(gid string) (storage.AntilinkCfg, error) {
	m.configMu.RLock()
	cfg, ok := m.antilinkCache[gid]
	m.configMu.RUnlock()
	if ok {
		return cfg, nil
	}
	cfg, err := m.db.GetAntilinkCfg(gid)
	if err == nil {
		m.configMu.Lock()
		m.antilinkCache[gid] = cfg
		m.configMu.Unlock()
	}
	return cfg, err
}

func (m *Manager) SaveAntilinkCfg(gid string, cfg storage.AntilinkCfg) error {
	err := m.db.SaveAntilinkCfg(gid, cfg)
	if err == nil {
		m.configMu.Lock()
		m.antilinkCache[gid] = cfg
		m.configMu.Unlock()
	}
	return err
}

func (m *Manager) GetPalantirCfg() (storage.PalantirCfg, error) {
	m.configMu.RLock()
	cfg := m.palantirCache
	hasInit := m.palantirCacheInit
	m.configMu.RUnlock()
	if hasInit {
		return cfg, nil
	}
	cfg, err := m.db.GetPalantirCfg()
	if err == nil {
		m.configMu.Lock()
		m.palantirCache = cfg
		m.palantirCacheInit = true
		m.configMu.Unlock()
	}
	return cfg, err
}

func (m *Manager) SavePalantirCfg(cfg storage.PalantirCfg) error {
	err := m.db.SavePalantirCfg(cfg)
	if err == nil {
		m.configMu.Lock()
		m.palantirCache = cfg
		m.palantirCacheInit = true
		m.configMu.Unlock()
	}
	return err
}

func (m *Manager) palantirWriterLoop() {
	m.palantirWG.Add(1)
	defer m.palantirWG.Done()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var batch []*PalantirLog
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if m.palantirDB != nil {
			_ = m.palantirDB.Update(func(tx *bolt.Tx) error {
				bkt := tx.Bucket([]byte("AuditLogs"))
				if bkt == nil {
					return nil
				}
				for _, entry := range batch {
					if data, err := json.Marshal(entry); err == nil {
						seq, _ := bkt.NextSequence()
						_ = bkt.Put([]byte(fmt.Sprintf("%d", seq)), data)
					}
				}
				return nil
			})
		}
		batch = batch[:0]
	}
	for {
		select {
		case entry, ok := <-m.palantirChan:
			if !ok {
				flush()
				return
			}
			batch = append(batch, entry)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func containsBypass(content, blocked string) bool {
	content = strings.ToLower(content)
	blocked = strings.ToLower(blocked)
	if blocked == "" {
		return false
	}
	if strings.Contains(content, blocked) {
		return true
	}
	normContent := normalizeHomoglyphs(content)
	normBlocked := normalizeHomoglyphs(blocked)
	if strings.Contains(normContent, normBlocked) {
		return true
	}
	return scanWithNoise(normContent, normBlocked)
}

func scanWithNoise(content, blocked string) bool {
	if len(blocked) == 0 {
		return false
	}
	runesContent := []rune(content)
	runesBlocked := []rune(blocked)
	for start := 0; start < len(runesContent); start++ {
		bIdx := 0
		cIdx := start
		var lastMatched rune
		for cIdx < len(runesContent) && bIdx < len(runesBlocked) {
			currChar := runesContent[cIdx]
			targetChar := runesBlocked[bIdx]
			if currChar == targetChar {
				lastMatched = currChar
				bIdx++
				cIdx++
			} else if isNoise(currChar) || (bIdx > 0 && currChar == lastMatched) {
				cIdx++
			} else {
				break
			}
		}
		if bIdx == len(runesBlocked) {
			return true
		}
	}
	return false
}

func isNoise(r rune) bool {
	if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
		return false
	}
	return true
}

func normalizeHomoglyphs(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case 'а', 'ä', 'á', 'à', 'â', 'ã', 'å', '@', '4':
			sb.WriteRune('a')
		case 'ß':
			sb.WriteString("ss")
		case 'с', 'ç', 'ć', 'ĉ', '(':
			sb.WriteRune('c')
		case 'ď', 'đ':
			sb.WriteRune('d')
		case 'е', 'ë', 'é', 'è', 'ê', '3':
			sb.WriteRune('e')
		case 'ƒ':
			sb.WriteRune('f')
		case 'ğ', 'ĝ', 'ģ', '9':
			sb.WriteRune('g')
		case 'ï', 'í', 'ì', 'î', '1', '!', '|':
			sb.WriteRune('i')
		case 'ј', 'ĵ':
			sb.WriteRune('j')
		case 'ķ':
			sb.WriteRune('k')
		case 'ľ', 'ļ', 'ĺ':
			sb.WriteRune('l')
		case 'ñ', 'ń', 'ņ', 'ň':
			sb.WriteRune('n')
		case 'о', 'ö', 'ó', 'ò', 'ô', 'õ', '0':
			sb.WriteRune('o')
		case 'ŕ', 'ŗ', 'ř':
			sb.WriteRune('r')
		case 'ś', 'ŝ', 'ş', 'š', '$', '5':
			sb.WriteRune('s')
		case 'ť', 'ţ', '7', '+':
			sb.WriteRune('t')
		case 'ü', 'ú', 'ù', 'û', 'μ':
			sb.WriteRune('u')
		case 'ŵ':
			sb.WriteRune('w')
		case 'ÿ', 'ý', 'ŷ':
			sb.WriteRune('y')
		case 'ź', 'ż', 'ž', '2':
			sb.WriteRune('z')
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

var rxLink = regexp.MustCompile(`(?i)https?://[^\s/$.?#].[^\s]*`)
var rxInvite = regexp.MustCompile(`(?i)(discord\.gg/|discord(app)?\.com/invite/)`)

func (m *Manager) checkAntilink(s *discordgo.Session, msg *discordgo.MessageCreate) bool {
	if msg.GuildID == "" || msg.Author == nil {
		return false
	}
	cfg, err := m.GetAntilinkCfg(msg.GuildID)
	if err != nil || !cfg.Enabled {
		return false
	}
	p, err := s.UserChannelPermissions(msg.Author.ID, msg.ChannelID)
	if err == nil && cfg.BypassPerms && (p&(discordgo.PermissionManageMessages|discordgo.PermissionManageGuild|discordgo.PermissionAdministrator)) != 0 {
		return false
	}
	for _, id := range cfg.Whitelist {
		if id == msg.Author.ID || id == msg.ChannelID {
			return false
		}
		if msg.Member != nil {
			for _, rID := range msg.Member.Roles {
				if rID == id {
					return false
				}
			}
		}
	}
	links := rxLink.FindAllString(msg.Content, -1)
	if len(links) == 0 {
		return false
	}
	hasViolation := false
	for _, link := range links {
		isInvite := rxInvite.MatchString(link)
		if cfg.BlockInvitesOnly {
			if isInvite {
				hasViolation = true
				break
			}
			continue
		}
		lowLink := strings.ToLower(link)
		if len(cfg.BlockedDomains) > 0 {
			blocked := false
			for _, bd := range cfg.BlockedDomains {
				if bd != "" && strings.Contains(lowLink, strings.ToLower(bd)) {
					blocked = true
					break
				}
			}
			if blocked {
				hasViolation = true
				break
			}
			continue
		}
		if len(cfg.AllowedDomains) > 0 {
			allowed := false
			for _, ad := range cfg.AllowedDomains {
				if ad != "" && strings.Contains(lowLink, strings.ToLower(ad)) {
					allowed = true
					break
				}
			}
			if !allowed {
				hasViolation = true
				break
			}
		} else {
			hasViolation = true
			break
		}
	}
	if hasViolation {
		_ = s.ChannelMessageDelete(msg.ChannelID, msg.ID)
		reason := "Anti-Link violation"
		switch cfg.Action {
		case "timeout":
			dur := time.Duration(cfg.TimeoutSecs) * time.Second
			until := time.Now().Add(dur)
			_ = s.GuildMemberTimeout(msg.GuildID, msg.Author.ID, &until, discordgo.WithAuditLogReason("Anti-Link | Punished user"))
			_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("[!] <@%s>, links are not allowed. You have been timed out for %s.", msg.Author.ID, dur.String()))
		case "kick":
			_ = s.GuildMemberDelete(msg.GuildID, msg.Author.ID, discordgo.WithAuditLogReason(reason))
			_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("[!] <@%s> has been kicked for posting links.", msg.Author.ID))
		case "ban":
			_ = s.GuildBanCreateWithReason(msg.GuildID, msg.Author.ID, reason, 0)
			_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("[!] <@%s> has been banned for posting links.", msg.Author.ID))
		default:
			_, _ = s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("[!] <@%s>, links are not allowed here.", msg.Author.ID))
		}
		return true
	}
	return false
}


