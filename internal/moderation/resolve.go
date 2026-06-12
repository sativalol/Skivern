package moderation

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func ResolveMember(s *discordgo.Session, gid, query string) (*discordgo.Member, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	if strings.HasPrefix(q, "<@") && strings.HasSuffix(q, ">") {
		id := q[2 : len(q)-1]
		if strings.HasPrefix(id, "!") {
			id = id[1:]
		}
		if strings.HasPrefix(id, "&") {
			id = id[1:]
		}
		isDigit := true
		for _, r := range id {
			if r < '0' || r > '9' {
				isDigit = false
				break
			}
		}
		if isDigit && id != "" {
			if m, err := s.State.Member(gid, id); err == nil {
				return m, nil
			}
			if m, err := s.GuildMember(gid, id); err == nil {
				return m, nil
			}
		}
	}

	isID := true
	if len(q) >= 17 && len(q) <= 21 {
		for _, r := range q {
			if r < '0' || r > '9' {
				isID = false
				break
			}
		}
	} else {
		isID = false
	}
	if isID {
		if m, err := s.State.Member(gid, q); err == nil {
			return m, nil
		}
		if m, err := s.GuildMember(gid, q); err == nil {
			return m, nil
		}
	}

	ql := strings.ToLower(q)

	if ms, err := s.GuildMembersSearch(gid, q, 100); err == nil && len(ms) > 0 {
		for _, m := range ms {
			if m.User != nil && (strings.EqualFold(m.User.Username, q) || strings.EqualFold(m.Nick, q) || strings.EqualFold(m.User.GlobalName, q)) {
				return m, nil
			}
		}
		for _, m := range ms {
			if m.User != nil {
				uName := strings.ToLower(m.User.Username)
				nick := strings.ToLower(m.Nick)
				gName := strings.ToLower(m.User.GlobalName)
				if strings.Contains(uName, ql) || strings.Contains(nick, ql) || strings.Contains(gName, ql) {
					return m, nil
				}
			}
		}
	}

	if g, err := s.State.Guild(gid); err == nil {
		for _, m := range g.Members {
			if m.User == nil {
				continue
			}
			uName := strings.ToLower(m.User.Username)
			nick := strings.ToLower(m.Nick)
			gName := strings.ToLower(m.User.GlobalName)

			if uName == ql || nick == ql || gName == ql ||
				strings.HasPrefix(uName, ql) || strings.HasPrefix(nick, ql) || strings.HasPrefix(gName, ql) ||
				strings.Contains(uName, ql) || strings.Contains(nick, ql) || strings.Contains(gName, ql) {
				return m, nil
			}
		}
	}

	if lst, err := s.GuildMembers(gid, "", 1000); err == nil {
		for _, m := range lst {
			if m.User == nil {
				continue
			}
			uName := strings.ToLower(m.User.Username)
			nick := strings.ToLower(m.Nick)
			gName := strings.ToLower(m.User.GlobalName)

			if uName == ql || nick == ql || gName == ql ||
				strings.HasPrefix(uName, ql) || strings.HasPrefix(nick, ql) || strings.HasPrefix(gName, ql) ||
				strings.Contains(uName, ql) || strings.Contains(nick, ql) || strings.Contains(gName, ql) {
				return m, nil
			}
		}
	}
	return nil, nil
}

func ResolveChannel(s *discordgo.Session, gid, query string) (*discordgo.Channel, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	if strings.HasPrefix(q, "<#") && strings.HasSuffix(q, ">") {
		id := q[2 : len(q)-1]
		if ch, err := s.State.Channel(id); err == nil && ch.GuildID == gid {
			return ch, nil
		}
		if ch, err := s.Channel(id); err == nil && ch.GuildID == gid {
			return ch, nil
		}
	}

	if len(q) >= 17 && len(q) <= 20 {
		isDigit := true
		for _, r := range q {
			if r < '0' || r > '9' {
				isDigit = false
				break
			}
		}
		if isDigit {
			if ch, err := s.State.Channel(q); err == nil && ch.GuildID == gid {
				return ch, nil
			}
			if ch, err := s.Channel(q); err == nil && ch.GuildID == gid {
				return ch, nil
			}
		}
	}

	ql := strings.ToLower(q)
	if strings.HasPrefix(ql, "#") {
		ql = ql[1:]
	}

	channels, err := s.GuildChannels(gid)
	if err != nil {
		return nil, err
	}

	for _, ch := range channels {
		cName := strings.ToLower(ch.Name)
		if cName == ql || strings.Contains(cName, ql) {
			return ch, nil
		}
	}

	return nil, nil
}
