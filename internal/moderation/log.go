package moderation

import (
	"fmt"
	"regexp"
	"skyvern/internal/config"
	"skyvern/internal/storage"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var rxCase = regexp.MustCompile(`(?i)\(Case\s*#?(\d+)[^)]*\)`)

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

	var caseID string
	actionClean := action
	if m := rxCase.FindStringSubmatch(action); len(m) > 1 {
		caseID = m[1]
		actionClean = strings.TrimSpace(rxCase.ReplaceAllString(action, ""))
	}

	targetName := "Unknown User"
	if targetUser, err := s.User(tid); err == nil && targetUser != nil {
		targetName = targetUser.Username
	}

	modName := "Unknown User"
	if modUser, err := s.User(mid); err == nil && modUser != nil {
		modName = modUser.Username
	}

	var duration string
	var otherFields []*discordgo.MessageEmbedField
	for _, field := range f {
		if strings.ToLower(field.Name) == "duration" {
			duration = field.Value
		} else {
			otherFields = append(otherFields, field)
		}
	}

	var lines []string
	if caseID != "" {
		lines = append(lines, fmt.Sprintf("**Case #%s | %s**", caseID, actionClean))
	} else {
		lines = append(lines, fmt.Sprintf("**Action: %s**", actionClean))
	}
	lines = append(lines, fmt.Sprintf("**User:** %s ( %s )", targetName, tid))
	lines = append(lines, fmt.Sprintf("**Moderator:** %s ( %s )", modName, mid))
	if duration != "" {
		lines = append(lines, fmt.Sprintf("**Duration:** %s", duration))
	}
	if reason != "" {
		lines = append(lines, fmt.Sprintf("**Reason:** %s", reason))
	}
	for _, field := range otherFields {
		lines = append(lines, fmt.Sprintf("**%s:** %s", field.Name, field.Value))
	}

	emb := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    "Modlog Entry",
			IconURL: resolved.FooterIcon,
		},
		Title:       "Information",
		Description: strings.Join(lines, "\n"),
		Color:       0x808080,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	_, _ = s.ChannelMessageSendEmbed(cfg.ChannelID, emb)
}

func DMUserAction(s *discordgo.Session, gid, action, targetID, modID, reason string) {
	// Clean up action name
	actPast := strings.ToLower(action)
	switch actPast {
	case "ban", "hardban", "softban":
		actPast = "banned"
	case "kick":
		actPast = "kicked"
	case "timeout":
		actPast = "timed out"
	case "warn":
		actPast = "warned"
	case "jail":
		actPast = "jailed"
	case "unjail":
		actPast = "unjailed"
	case "unwarn":
		actPast = "unwarned"
	case "unban":
		actPast = "unbanned"
	case "untimeout":
		actPast = "untimeouted"
	}

	guildName := "this server"
	if g, err := s.State.Guild(gid); err == nil && g != nil {
		guildName = g.Name
	}

	channel, err := s.UserChannelCreate(targetID)
	if err != nil {
		return
	}

	emb := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("You have been %s", actPast),
		Description: fmt.Sprintf("You were %s in **%s**.", actPast, guildName),
		Color:       0x808080,
		Fields: []*discordgo.MessageEmbedField{
			config.Field("Moderator", fmt.Sprintf("<@%s>", modID), true),
			config.Field("Reason", reason, false),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, _ = s.ChannelMessageSendEmbed(channel.ID, emb)
}
