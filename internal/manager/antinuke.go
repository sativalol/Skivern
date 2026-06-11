package manager

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"skyvern/internal/config"
)

var (
	anMu      sync.Mutex
	anActions = make(map[string]map[string]map[string][]time.Time) // guildID -> actorID -> actionType -> timestamps
)

func sfTimeConv(id string) (time.Time, error) {
	var sf int64
	var v uint64
	for _, r := range id {
		if r >= '0' && r <= '9' {
			v = v*10 + uint64(r-'0')
		}
	}
	sf = int64(v)
	t := (sf >> 22) + 1420070400000
	return time.Unix(0, t*int64(time.Millisecond)), nil
}

func (m *Manager) TrackAntinuke(s *discordgo.Session, gid, targetID string, act discordgo.AuditLogAction) {
	cfg, err := m.db.GetAntinukeCfg(gid)
	if err != nil || !cfg.Enabled {
		return
	}

	// Wait a brief moment for Discord to populate the audit log
	time.Sleep(800 * time.Millisecond)

	// Fetch recent audit logs for this action
	log, err := s.GuildAuditLog(gid, "", "", int(act), 5)
	if err != nil {
		return
	}

	bid := s.State.User.ID
	var actorID string
	now := time.Now()

	for _, ent := range log.AuditLogEntries {
		if ent.TargetID != targetID {
			continue
		}
		if ent.UserID == bid {
			return // Ignore actions taken by the bot itself
		}
		// Check timestamp
		t, err := sfTimeConv(ent.ID)
		if err == nil && now.Sub(t) < 8*time.Second {
			actorID = ent.UserID
			break
		}
	}

	// If targetID is empty or we couldn't match targetID perfectly, fallback to first recent entry of this action type
	if actorID == "" && len(log.AuditLogEntries) > 0 {
		first := log.AuditLogEntries[0]
		if first.UserID != bid {
			t, err := sfTimeConv(first.ID)
			if err == nil && now.Sub(t) < 8*time.Second {
				actorID = first.UserID
			}
		}
	}

	if actorID == "" {
		return
	}

	// Bypass check
	if m.db.HasBypass(gid, actorID) {
		return
	}
	g, err := s.State.Guild(gid)
	if err == nil && g.OwnerID == actorID {
		return
	}

	// Determine type, limit, and window
	var actType string
	var limit, secs int

	switch act {
	case discordgo.AuditLogActionChannelCreate:
		actType = "chan_create"
		limit = cfg.ChanLimit
		secs = cfg.ChanSecs
	case discordgo.AuditLogActionChannelDelete:
		actType = "chan_delete"
		limit = cfg.ChanLimit
		secs = cfg.ChanSecs
	case discordgo.AuditLogActionRoleCreate:
		actType = "role_create"
		limit = cfg.RoleLimit
		secs = cfg.RoleSecs
	case discordgo.AuditLogActionRoleDelete:
		actType = "role_delete"
		limit = cfg.RoleLimit
		secs = cfg.RoleSecs
	case discordgo.AuditLogActionMemberBanAdd:
		actType = "ban"
		limit = cfg.BanLimit
		secs = cfg.BanSecs
	case discordgo.AuditLogActionMemberKick:
		actType = "kick"
		limit = cfg.KickLimit
		secs = cfg.KickSecs
	case discordgo.AuditLogActionBotAdd:
		actType = "bot"
		limit = cfg.BotLimit
		secs = cfg.BotSecs
	default:
		return
	}

	anMu.Lock()
	if anActions[gid] == nil {
		anActions[gid] = make(map[string]map[string][]time.Time)
	}
	if anActions[gid][actorID] == nil {
		anActions[gid][actorID] = make(map[string][]time.Time)
	}

	var active []time.Time
	cutoff := now.Add(-time.Duration(secs) * time.Second)
	for _, ts := range anActions[gid][actorID][actType] {
		if ts.After(cutoff) {
			active = append(active, ts)
		}
	}
	active = append(active, now)
	anActions[gid][actorID][actType] = active
	anMu.Unlock()

	if len(active) > limit {
		m.punishAdmin(s, gid, actorID, actType, len(active), limit, cfg.Action)
	}
}

func (m *Manager) punishAdmin(s *discordgo.Session, gid, actorID, actType string, count, limit int, punish string) {
	mem, err := s.GuildMember(gid, actorID)
	if err != nil {
		return
	}

	reason := fmt.Sprintf("[Skyvern Antinuke] Triggered %s limit (%d/%d actions)", actType, count, limit)

	// Log the alert to the modlog first
	m.logAntinukeAlert(s, gid, actorID, actType, count, limit, punish)

	switch strings.ToLower(punish) {
	case "ban":
		_ = s.GuildBanCreateWithReason(gid, actorID, reason, 7)
	case "kick":
		_ = s.GuildMemberDeleteWithReason(gid, actorID, reason)
	default: // "strip"
		for _, rid := range mem.Roles {
			_ = s.GuildMemberRoleRemove(gid, actorID, rid)
		}
	}
}

func (m *Manager) logAntinukeAlert(s *discordgo.Session, gid, actorID, actType string, count, limit int, punish string) {
	mCfg, err := m.db.GetModlog(gid)
	if err != nil || mCfg.ChannelID == "" {
		return
	}

	inst, err := m.db.GetBot(s.State.User.ID)
	var resolved config.ResCfg
	if err == nil {
		resolved = config.Resolve(config.GetGlobal(), inst)
	} else {
		resolved = config.Resolve(config.GetGlobal(), config.BotInst{})
	}

	ef := []*discordgo.MessageEmbedField{
		config.Field("Administrator", fmt.Sprintf("<@%s> (`%s`)", actorID, actorID), true),
		config.Field("Action Type", actType, true),
		config.Field("Actions count", fmt.Sprintf("%d/%d", count, limit), true),
		config.Field("Punishment Applied", punish, true),
	}

	emb := config.Build(resolved, config.EmbedOpt{
		Title:  "🛡️ Antinuke Alert - Protection Triggered",
		Fields: ef,
	})
	emb.Color = 0xff0000 // Red color for critical alerts

	_, _ = s.ChannelMessageSendEmbed(mCfg.ChannelID, emb)
}
