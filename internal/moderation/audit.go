package moderation

import (
	"skyvern/internal/storage"
	"time"

	"github.com/bwmarrin/discordgo"
)

func ProcAudit(s *discordgo.Session, db *storage.DB, gid, tid string, act discordgo.AuditLogAction) {
	cfg, err := db.GetModlog(gid)
	if err != nil || cfg.ChannelID == "" || !cfg.LogDiscord {
		return
	}

	time.Sleep(800 * time.Millisecond)

	log, err := s.GuildAuditLog(gid, "", "", int(act), 5)
	if err != nil {
		return
	}

	bid := s.State.User.ID

	for _, ent := range log.AuditLogEntries {
		if ent.TargetID != tid {
			continue
		}
		if ent.UserID == bid {
			return
		}
		t, err := sfTimeConv(ent.ID)
		if err != nil || time.Since(t) > 5*time.Second {
			continue
		}
		LogAction(s, db, gid, actName(act)+" (Manual)", ent.UserID, tid, ent.Reason)
		return
	}
}

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

func actName(act discordgo.AuditLogAction) string {
	switch act {
	case discordgo.AuditLogActionMemberBanAdd:
		return "Ban"
	case discordgo.AuditLogActionMemberBanRemove:
		return "Unban"
	case discordgo.AuditLogActionMemberKick:
		return "Kick"
	case discordgo.AuditLogActionMemberUpdate:
		return "Update"
	}
	return "Action"
}
