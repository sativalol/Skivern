package fun

import (
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var rxVouchUser = regexp.MustCompile(`^<@!?(\d+)>$`)

func resolveVouchUser(s *discordgo.Session, gid, query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}
	if m := rxVouchUser.FindStringSubmatch(q); len(m) > 1 {
		return m[1]
	}
	members, err := s.GuildMembers(gid, "", 1000)
	if err != nil {
		return ""
	}
	for _, m := range members {
		if m.User.ID == q {
			return m.User.ID
		}
	}
	ql := strings.ToLower(q)
	for _, m := range members {
		if strings.ToLower(m.User.Username) == ql {
			return m.User.ID
		}
	}
	return ""
}
