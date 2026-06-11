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

	if len(q) >= 17 && len(q) <= 20 {
		if _, err := s.User(q); err == nil {
			if m, err := s.State.Member(gid, q); err == nil {
				return m, nil
			}
			if m, err := s.GuildMember(gid, q); err == nil {
				return m, nil
			}
		}
	}

	ql := strings.ToLower(q)

	// query state cache first to avoid API call
	if g, err := s.State.Guild(gid); err == nil {
		for _, m := range g.Members {
			if m.User == nil {
				continue
			}
			uName := strings.ToLower(m.User.Username)
			nick := strings.ToLower(m.Nick)

			if uName == ql || nick == ql ||
				strings.HasPrefix(uName, ql) || strings.HasPrefix(nick, ql) ||
				strings.HasSuffix(uName, ql) || strings.HasSuffix(nick, ql) ||
				strings.Contains(uName, ql) || strings.Contains(nick, ql) {
				return m, nil
			}
		}
	}

	lst, err := s.GuildMembers(gid, "", 1000)
	if err != nil {
		return nil, err
	}

	for _, m := range lst {
		if m.User == nil {
			continue
		}
		uName := strings.ToLower(m.User.Username)
		nick := strings.ToLower(m.Nick)

		if uName == ql || nick == ql ||
			strings.HasPrefix(uName, ql) || strings.HasPrefix(nick, ql) ||
			strings.HasSuffix(uName, ql) || strings.HasSuffix(nick, ql) ||
			strings.Contains(uName, ql) || strings.Contains(nick, ql) {
			return m, nil
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
