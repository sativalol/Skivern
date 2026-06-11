package manager

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	bolt "go.etcd.io/bbolt"
	"skyvern/internal/config"
	"skyvern/internal/storage"
)

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

	lastBoostLogged map[string]time.Time
	boostMu         sync.Mutex

	reminders   []storage.Reminder
	schedules   []storage.ScheduledMsg
	remindersMu sync.RWMutex
	schedulesMu sync.RWMutex

	antispamTracker map[string][]time.Time
	antispamMu      sync.Mutex

	palantirDB        *bolt.DB
	antispamCache     map[string]storage.AntispamCfg
	filterCache       map[string]storage.FilterCfg
	antilinkCache     map[string]storage.AntilinkCfg
	prefixCache       map[string]string
	palantirCache     storage.PalantirCfg
	palantirCacheInit bool
	configMu          sync.RWMutex

	palantirChan chan *PalantirLog
	palantirWG   sync.WaitGroup

	whs  map[string]*discordgo.Webhook
	whMu sync.RWMutex
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
		whs:                   make(map[string]*discordgo.Webhook),
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
	if m.compHandlers == nil {
		m.compHandlers = make(map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate))
	}
	m.compHandlers[customID] = fn
	m.mu.Unlock()
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
	m.commands = append(m.commands, cmds...)
	m.mu.Unlock()
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

func (m *Manager) Stats(clientID string) storage.Analytics { return m.stats.get(clientID) }
func (m *Manager) GlobalStats() storage.Analytics          { return m.stats.totals() }

func (m *Manager) UpdateInstance(cid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, err := m.db.GetBot(cid)
	if err != nil {
		return err
	}
	cfg := config.Resolve(config.GetGlobal(), inst)
	if s, ok := m.instances[cid]; ok {
		s.cfg = cfg
	}
	return nil
}

func (m *Manager) Snapshot() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]bool)
	for cid, inst := range m.instances {
		out[cid] = inst.running
	}
	return out
}

func (m *Manager) ResolvedCfgFor(cid string) (config.ResCfg, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if inst, ok := m.instances[cid]; ok {
		return inst.cfg, true
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

func (m *Manager) updateAvatar(sess *discordgo.Session, url, clientID string) {
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

func (m *Manager) DB() *storage.DB { return m.db }

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
