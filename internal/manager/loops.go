package manager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/bwmarrin/discordgo"
	"skyvern/internal/storage"
)

func (m *Manager) flushLoop() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			m.flushAnalytics()
		case <-m.stopFlush:
			m.flushAnalytics()
			return
		}
	}
}

func (m *Manager) flushAnalytics() {
	m.stats.mu.Lock()
	if m.stats.lastFlushed == nil {
		m.stats.lastFlushed = make(map[string]storage.Analytics)
	}
	dirty := make(map[string]storage.Analytics)
	for k, v := range m.stats.data {
		last := m.stats.lastFlushed[k]
		if v.TotalCmds != last.TotalCmds || v.PrefixCmds != last.PrefixCmds || v.SlashCmds != last.SlashCmds || v.GuildCount != last.GuildCount {
			dirty[k] = *v
			m.stats.lastFlushed[k] = *v
		}
	}
	m.stats.mu.Unlock()

	for id, a := range dirty {
		if err := m.db.SaveAnalytics(id, a); err != nil {
			fmt.Printf("flush %q: %v\n", id, err)
		}
	}
}

func (m *Manager) tempRoleLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			roles, err := m.db.GetExpiredTempRoles()
			if err != nil || len(roles) == 0 {
				continue
			}
			m.mu.RLock()
			var activeSess *discordgo.Session
			for _, inst := range m.instances {
				if inst.running {
					activeSess = inst.session
					break
				}
			}
			m.mu.RUnlock()

			if activeSess == nil {
				continue
			}

			for _, tr := range roles {
				_ = activeSess.GuildMemberRoleRemove(tr.GuildID, tr.UserID, tr.RoleID)
				_ = m.db.DeleteTempRole(tr.GuildID, tr.UserID, tr.RoleID)
			}
		case <-m.stopFlush:
			return
		}
	}
}

var dailyQuestions = []string{
	"What is the most interesting thing you read or watched this week?",
	"If you could have any superpower, what would it be and why?",
	"What is your go-to comfort food?",
	"What's the best piece of advice you've ever received?",
	"If you could travel back in time, which decade would you visit?",
	"What's your favorite hobby or way to pass the time?",
	"What is one goal you want to achieve this month?",
	"What is the last song you listened to?",
	"What is your favorite book or movie of all time?",
	"If you could have dinner with any historical figure, who would it be?",
}

var dailyQuotes = []string{
	"The only way to do great work is to love what you do. - Steve Jobs",
	"Believe you can and you're halfway there. - Theodore Roosevelt",
	"It always seems impossible until it's done. - Nelson Mandela",
	"Success is not final, failure is not fatal: it is the courage to continue that counts. - Winston Churchill",
	"Act as if what you do makes a difference. It does. - William James",
	"The future belongs to those who believe in the beauty of their dreams. - Eleanor Roosevelt",
	"Do what you can, with what you have, where you are. - Theodore Roosevelt",
	"Keep your face always toward the sunshine - and shadows will fall behind you. - Walt Whitman",
	"You miss 100% of the shots you don't take. - Wayne Gretzky",
	"The only limit to our realization of tomorrow will be our doubts of today. - Franklin D. Roosevelt",
}

func (m *Manager) dailySchedulerLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			timeStr := now.Format("15:04")
			dateStr := now.Format("2006-01-02")

			qCfgs, err := m.db.ListDailyQuestions()
			if err == nil {
				for gid, cfg := range qCfgs {
					if !cfg.Enabled || cfg.ChannelID == "" || cfg.Time != timeStr {
						continue
					}
					m.dailyMu.Lock()
					lastDate := m.lastDailyQuestionDate[gid]
					if lastDate == dateStr {
						m.dailyMu.Unlock()
						continue
					}
					m.lastDailyQuestionDate[gid] = dateStr
					m.dailyMu.Unlock()

					m.mu.RLock()
					var s *discordgo.Session
					for _, inst := range m.instances {
						if inst.running {
							s = inst.session
							break
						}
					}
					m.mu.RUnlock()

					if s != nil {
						q := fetchDailyQuestion()
						_, _ = s.ChannelMessageSend(cfg.ChannelID, fmt.Sprintf("**Daily Question:** %s", q))
					}
				}
			}

			qList, err := m.db.ListDailyQuotes()
			if err == nil {
				for gid, cfg := range qList {
					if !cfg.Enabled || cfg.ChannelID == "" || cfg.Time != timeStr {
						continue
					}
					m.dailyMu.Lock()
					lastDate := m.lastDailyQuoteDate[gid]
					if lastDate == dateStr {
						m.dailyMu.Unlock()
						continue
					}
					m.lastDailyQuoteDate[gid] = dateStr
					m.dailyMu.Unlock()

					m.mu.RLock()
					var s *discordgo.Session
					for _, inst := range m.instances {
						if inst.running {
							s = inst.session
							break
						}
					}
					m.mu.RUnlock()

					if s != nil {
						quote := fetchDailyQuote()
						_, _ = s.ChannelMessageSend(cfg.ChannelID, fmt.Sprintf("**Daily Quote:** %s", quote))
					}
				}
			}

		case <-m.stopFlush:
			return
		}
	}
}

func fetchDailyQuestion() string {
	resp, err := http.Get("https://api.truthordarebot.xyz/api/truth")
	if err == nil {
		defer resp.Body.Close()
		var res struct {
			Question string `json:"question"`
		}
		if json.NewDecoder(resp.Body).Decode(&res) == nil && res.Question != "" {
			return res.Question
		}
	}
	return dailyQuestions[time.Now().UnixNano()%int64(len(dailyQuestions))]
}

func fetchDailyQuote() string {
	resp, err := http.Get("https://zenquotes.io/api/random")
	if err == nil {
		defer resp.Body.Close()
		var res []struct {
			Q string `json:"q"`
			A string `json:"a"`
		}
		if json.NewDecoder(resp.Body).Decode(&res) == nil && len(res) > 0 && res[0].Q != "" {
			return fmt.Sprintf("*\"%s\"* - %s", res[0].Q, res[0].A)
		}
	}
	return fmt.Sprintf("*\"%s\"*", dailyQuotes[time.Now().UnixNano()%int64(len(dailyQuotes))])
}

func (m *Manager) birthdayLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.mu.RLock()
			var sess *discordgo.Session
			for _, inst := range m.instances {
				if inst.running {
					sess = inst.session
					break
				}
			}
			m.mu.RUnlock()

			if sess == nil {
				continue
			}

			sess.State.RLock()
			var gids []string
			for _, g := range sess.State.Guilds {
				gids = append(gids, g.ID)
			}
			sess.State.RUnlock()

			for _, gid := range gids {
				chID, err := m.db.GetBirthdayChannel(gid)
				if err != nil || chID == "" {
					continue
				}

				birthdays, err := m.db.ListBirthdays(gid)
				if err != nil || len(birthdays) == 0 {
					continue
				}

				for uid, bday := range birthdays {
					tzName, _ := m.db.GetTimezone(uid)
					var loc *time.Location
					if tzName != "" {
						if l, err := time.LoadLocation(tzName); err == nil {
							loc = l
						}
					}
					if loc == nil {
						loc = time.Local
					}

					nowInTZ := time.Now().In(loc)
					if nowInTZ.Format("01/02") == bday {
						lastWished, err := m.db.GetLastBirthdayWished(gid, uid)
						if err != nil || lastWished != nowInTZ.Year() {
							_ = m.db.SaveLastBirthdayWished(gid, uid, nowInTZ.Year())
							_, _ = sess.ChannelMessageSend(chID, fmt.Sprintf("🎂 Happy Birthday <@%s>! Wishing you a fantastic day! 🎉", uid))
						}
					}
				}
			}

		case <-m.stopFlush:
			return
		}
	}
}

func (m *Manager) remindLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.remindersMu.Lock()
			if len(m.reminders) == 0 {
				m.remindersMu.Unlock()
				continue
			}
			m.mu.RLock()
			var sess *discordgo.Session
			for _, inst := range m.instances {
				if inst.running {
					sess = inst.session
					break
				}
			}
			m.mu.RUnlock()
			if sess == nil {
				m.remindersMu.Unlock()
				continue
			}
			now := time.Now()
			var active []storage.Reminder
			for _, r := range m.reminders {
				if r.Time.Before(now) {
					go func(rem storage.Reminder, s *discordgo.Session) {
						dm, err := s.UserChannelCreate(rem.UserID)
						if err == nil {
							_, _ = s.ChannelMessageSend(dm.ID, fmt.Sprintf("⏰ **Reminder:** %s", rem.Message))
						}
						_ = m.db.DeleteReminder(rem.UserID, rem.ID)
					}(r, sess)
				} else {
					active = append(active, r)
				}
			}
			m.reminders = active
			m.remindersMu.Unlock()
		case <-m.stopFlush:
			return
		}
	}
}

func (m *Manager) scheduleLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.schedulesMu.Lock()
			if len(m.schedules) == 0 {
				m.schedulesMu.Unlock()
				continue
			}
			m.mu.RLock()
			var sess *discordgo.Session
			for _, inst := range m.instances {
				if inst.running {
					sess = inst.session
					break
				}
			}
			m.mu.RUnlock()
			if sess == nil {
				m.schedulesMu.Unlock()
				continue
			}
			now := time.Now()
			var active []storage.ScheduledMsg
			for _, sch := range m.schedules {
				if sch.Time.Before(now) {
					go func(sc storage.ScheduledMsg, ss *discordgo.Session) {
						_, _ = ss.ChannelMessageSend(sc.ChannelID, sc.Message)
						_ = m.db.DeleteSchedule(sc.GuildID, sc.ID)
					}(sch, sess)
				} else {
					active = append(active, sch)
				}
			}
			m.schedules = active
			m.schedulesMu.Unlock()
		case <-m.stopFlush:
			return
		}
	}
}

func (m *Manager) gcLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			runtime.GC()
			debug.FreeOSMemory()
		case <-m.stopFlush:
			return
		}
	}
}
