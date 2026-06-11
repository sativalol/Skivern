package manager

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	bolt "go.etcd.io/bbolt"
	"skyvern/internal/storage"
)

type PalantirLog struct {
	Timestamp time.Time `json:"timestamp"`
	GuildID   string    `json:"guild_id"`
	Category  string    `json:"category"`
	Title     string    `json:"title"`
	Desc      string    `json:"description"`
	UserID    string    `json:"user_id"`
	ChannelID string    `json:"channel_id"`
}

func (m *Manager) checkAntispam(s *discordgo.Session, msg *discordgo.MessageCreate) bool {
	if msg.GuildID == "" || msg.Author == nil {
		return false
	}
	cfg, err := m.GetAntispamCfg(msg.GuildID)
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
	cfg, err := m.GetFilterCfg(msg.GuildID)
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

func (m *Manager) LogPalantir(guildID, category, title, desc, userID, channelID string) {
	if pCfg, err := m.GetPalantirCfg(); err == nil && pCfg.Enabled {
		allowed := true
		for _, g := range pCfg.BlockedGuilds {
			if g == guildID {
				allowed = false
				break
			}
		}
		if allowed && channelID != "" {
			for _, c := range pCfg.BlockedChannels {
				if c == channelID {
					allowed = false
					break
				}
			}
		}
		if allowed && userID != "" {
			for _, u := range pCfg.BlockedUsers {
				if u == userID {
					allowed = false
					break
				}
			}
		}
		if allowed && category != "" {
			for _, e := range pCfg.BlockedEvents {
				if strings.EqualFold(e, category) {
					allowed = false
					break
				}
			}
		}
		if !allowed {
			return
		}

		select {
		case m.palantirChan <- &PalantirLog{
			Timestamp: time.Now(),
			GuildID:   guildID,
			Category:  category,
			Title:     title,
			Desc:      desc,
			UserID:    userID,
			ChannelID: channelID,
		}:
		default:
			// queue full, drop
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

var rxLinkFull = regexp.MustCompile(`(?i)(?:https?://|//)[^\s<>"']+`)
var rxLinkWWW = regexp.MustCompile(`(?i)\bwww\.[^\s<>"']+`)
var rxLinkBare = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9\-]{0,61}[a-z0-9])?\.)+(?:com|net|org|io|gg|tv|me|app|dev|xyz|info|link|click|ly|to|sh|cc|tk|ml|ga|cf|gq|pw|online|site|biz|edu|gov|co|uk|us|ca|au|de|fr|ru|jp|cn|br|nl|se|no|fi|dk|pl|es|it|pt|be|ch|at|nz|sg|hk|in|kr|mx|ar|za|ng|pk|vn|th|id|eg|club|icu|top|vip|live|stream|news|store|shop|tech|art|pro|media)(?:/[^\s<>"']*)?`)
var rxInvite = regexp.MustCompile(`(?i)(?:discord\.gg/|discord(?:app)?\.com/invite/)[^\s<>"']+`)

func extractLinks(content string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(matches []string) {
		for _, m := range matches {
			m = strings.TrimRight(m, ".,;:!?)>")
			if m != "" && !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	add(rxLinkFull.FindAllString(content, -1))
	add(rxInvite.FindAllString(content, -1))
	add(rxLinkWWW.FindAllString(content, -1))
	add(rxLinkBare.FindAllString(content, -1))
	return out
}

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
	links := extractLinks(msg.Content)
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
