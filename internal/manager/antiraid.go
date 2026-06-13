package manager

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"skyvern/internal/config"
	"skyvern/internal/storage"
)

var (
	arMu    sync.Mutex
	arJoins = make(map[string][]time.Time) // guildID -> join timestamps
)

func (m *Manager) TrackAntiraidJoin(s *discordgo.Session, gid string, mem *discordgo.Member) {
	cfg, err := m.GetAntiraidCfg(gid)
	if err != nil || !cfg.Enabled {
		return
	}

	arMu.Lock()
	now := time.Now()
	cutoff := now.Add(-time.Duration(cfg.Seconds) * time.Second)

	var active []time.Time
	for _, t := range arJoins[gid] {
		if t.After(cutoff) {
			active = append(active, t)
		}
	}
	active = append(active, now)
	arJoins[gid] = active
	arMu.Unlock()

	// If join count exceeds limit, trigger protection
	if len(active) > cfg.JoinLimit {
		m.mitigateRaid(s, gid, mem, len(active), cfg)
	}
}

func (m *Manager) mitigateRaid(s *discordgo.Session, gid string, mem *discordgo.Member, count int, cfg storage.AntiraidCfg) {
	reason := fmt.Sprintf("[Skyvern Antiraid] Join flood detected (%d joins in %d seconds)", count, cfg.Seconds)

	// Action dispatch
	actionTaken := "Alerted / Monitored"
	switch strings.ToLower(cfg.Action) {
	case "ban":
		actionTaken = "Banned User"
		_ = s.GuildBanCreateWithReason(gid, mem.User.ID, reason, 0)
	case "kick":
		actionTaken = "Kicked User"
		_ = s.GuildMemberDeleteWithReason(gid, mem.User.ID, reason)
	case "lockdown":
		actionTaken = "Channel Lockdown Engaged"
		go m.lockdownGuild(s, gid)
	}

	// Log raid alert
	m.logAntiraidAlert(s, gid, mem.User.Username, mem.User.ID, count, cfg, actionTaken)
}

func (m *Manager) lockdownGuild(s *discordgo.Session, gid string) {
	chans, err := s.GuildChannels(gid)
	if err != nil {
		return
	}
	for _, c := range chans {
		if c.Type == discordgo.ChannelTypeGuildText {
			_ = s.ChannelPermissionSet(c.ID, gid, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionSendMessages, discordgo.WithAuditLogReason("Antiraid Lockdown"))
		}
	}
}

func (m *Manager) logAntiraidAlert(s *discordgo.Session, gid, username, uid string, count int, cfg storage.AntiraidCfg, action string) {
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
		config.Field("Raid Trigger User", fmt.Sprintf("%s (`%s`)", username, uid), true),
		config.Field("Join Rate", fmt.Sprintf("%d joins / %d secs", count, cfg.Seconds), true),
		config.Field("Configured Action", cfg.Action, true),
		config.Field("Applied Mitigation", action, true),
	}

	emb := config.Build(resolved, config.EmbedOpt{
		Title:  "⚠️ Antiraid Triggered - Raid Detected",
		Fields: ef,
	})
	emb.Color = 0xffa500 // Orange alert color for raids

	_, _ = s.ChannelMessageSendEmbed(mCfg.ChannelID, emb)
}

