package moderation

import (
	"fmt"
	"skyvern/internal/config"
	"skyvern/internal/storage"

	"github.com/bwmarrin/discordgo"
)

func LogAction(s *discordgo.Session, db *storage.DB, gid, action, mid, tid, reason string, f ...*discordgo.MessageEmbedField) {
	cfg, err := db.GetModlog(gid)
	if err != nil || cfg.ChannelID == "" {
		return
	}

	inst, err := db.GetBot(s.State.User.ID)
	var resolved config.ResCfg
	if err == nil {
		resolved = config.Resolve(config.GetGlobal(), inst)
	} else {
		resolved = config.Resolve(config.GetGlobal(), config.BotInst{})
	}

	ef := []*discordgo.MessageEmbedField{
		config.Field("Target User", fmt.Sprintf("<@%s> (`%s`)", tid, tid), true),
		config.Field("Moderator", fmt.Sprintf("<@%s> (`%s`)", mid, mid), true),
	}

	if reason != "" {
		ef = append(ef, config.Field("Reason", reason, false))
	}
	ef = append(ef, f...)

	emb := config.Build(resolved, config.EmbedOpt{
		Title:  "Moderation Action: " + action,
		Fields: ef,
	})

	_, _ = s.ChannelMessageSendEmbed(cfg.ChannelID, emb)
}
