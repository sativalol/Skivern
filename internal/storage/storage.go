package storage

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"skyvern/internal/config"

	bolt "go.etcd.io/bbolt"
)

var (
	bktGlobal    = []byte("GlobalConfig")
	bktBots      = []byte("Bots")
	bktAnalytics = []byte("Analytics")
	bktNicklocks = []byte("Nicklocks")
	bktModlogs   = []byte("ModlogsConfig")
	bktCases     = []byte("Cases")
	bktJails     = []byte("Jails")
	bktInvokes   = []byte("Invokes")
	bktAntinuke  = []byte("AntinukeBypass")
	bktPrefixes  = []byte("GuildPrefixes")
	bktTempRoles = []byte("TempRoles")
	bktStickyRoles = []byte("StickyRoles")
	bktAutoroles = []byte("Autoroles")
	bktDailyQuestions = []byte("DailyQuestions")
	bktDailyQuotes = []byte("DailyQuotes")
	bktUserMessages = []byte("UserMessages")
	bktBoostCfg     = []byte("BoostConfig")
	bktBoosterRoles = []byte("BoosterRoles")
	bktHallCfg      = []byte("HallConfig")
	bktHallMessages = []byte("HallMessages")
	bktAFK          = []byte("AFKStatus")
	bktAutoreact    = []byte("Autoreact")
	bktAutorespond  = []byte("Autoresponder")
	bktBirthdays    = []byte("Birthdays")
	bktTimezones    = []byte("Timezones")
	bktBirthdayAnn  = []byte("BirthdayAnnouncements")
	bktBdayWished   = []byte("BdayWished")
	bktBumpReminder = []byte("BumpReminder")
	bktButtonRoles  = []byte("ButtonRoles")
	bktReactRoles   = []byte("ReactRoles")
	bktReminders    = []byte("Reminders")
	bktSchedules    = []byte("Schedules")
	bktStarboardCfg = []byte("StarboardCfg")
	bktStarboardMsg = []byte("StarboardMsg")
	bktTags         = []byte("Tags")
	bktTempVoiceCfg = []byte("TempVoiceCfg")
	bktTempVoiceChan= []byte("TempVoiceChan")
	bktVanityCfg    = []byte("VanityCfg")
	bktVouches      = []byte("Vouches")
	bktLoggerSubs   = []byte("LoggerSubs")
	bktLoggerIgnores = []byte("LoggerIgnores")
	bktAntispam     = []byte("AntispamCfg")
	bktFilters      = []byte("FilterCfg")
	bktPalantirCfg  = []byte("PalantirCfg")
	bktAntilink     = []byte("AntilinkCfg")

	keyGlobal = []byte("cfg")
)

type Analytics struct {
	TotalCmds  int64 `json:"total_cmds"`
	PrefixCmds int64 `json:"prefix_cmds"`
	SlashCmds  int64 `json:"slash_cmds"`
	GuildCount int   `json:"guild_count"`
}

type ModlogCfg struct {
	ChannelID  string `json:"channel_id"`
	LogDiscord bool   `json:"log_discord"`
}

type DB struct {
	b *bolt.DB
}

func Open(path string) (*DB, error) {
	b, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("bolt open: %w", err)
	}

	err = b.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{
			bktGlobal, bktBots, bktAnalytics, bktNicklocks, bktModlogs,
			bktCases, bktJails, bktInvokes, bktAntinuke, bktPrefixes,
			bktTempRoles, bktStickyRoles, bktAutoroles, bktDailyQuestions,
			bktDailyQuotes, bktUserMessages, bktBoostCfg, bktBoosterRoles,
			bktHallCfg, bktHallMessages, bktAFK, bktAutoreact, bktAutorespond,
			bktBirthdays, bktTimezones, bktBirthdayAnn, bktBdayWished,
			bktBumpReminder, bktButtonRoles, bktReactRoles, bktReminders,
			bktSchedules, bktStarboardCfg, bktStarboardMsg, bktTags,
			bktTempVoiceCfg, bktTempVoiceChan, bktVanityCfg, bktVouches,
			bktLoggerSubs, bktLoggerIgnores, bktAntispam, bktFilters, bktPalantirCfg, bktAntilink,
		} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bucket init: %w", err)
	}

	db := &DB{b: b}
	if err := db.seedGlobal(); err != nil {
		return nil, err
	}
	return db, nil
}

func (d *DB) Close() error { return d.b.Close() }

func (d *DB) seedGlobal() error {
	return d.b.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktGlobal)
		if bkt.Get(keyGlobal) != nil {
			return nil
		}
		return putJSON(bkt, keyGlobal, config.DefGlobal())
	})
}

func (d *DB) GetGlobal() (config.GlobalCfg, error) {
	var cfg config.GlobalCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktGlobal).Get(keyGlobal)
		if v == nil {
			cfg = config.DefGlobal()
			return nil
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
}

func (d *DB) SaveGlobal(cfg config.GlobalCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktGlobal), keyGlobal, cfg)
	})
}

func (d *DB) GetBot(cid string) (config.BotInst, error) {
	var inst config.BotInst
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktBots).Get([]byte(cid))
		if v == nil {
			return fmt.Errorf("bot %q not found", cid)
		}
		return json.Unmarshal(v, &inst)
	})
	return inst, err
}

func (d *DB) ListBots() ([]config.BotInst, error) {
	var bots []config.BotInst
	err := d.b.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBots).ForEach(func(_, v []byte) error {
			var inst config.BotInst
			if err := json.Unmarshal(v, &inst); err != nil {
				return err
			}
			bots = append(bots, inst)
			return nil
		})
	})
	return bots, err
}

func (d *DB) SaveBot(inst config.BotInst) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktBots), []byte(inst.ClientID), inst)
	})
}

func (d *DB) DeleteBot(cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(bktBots).Delete([]byte(cid)); err != nil {
			return err
		}
		return tx.Bucket(bktAnalytics).Delete([]byte(cid))
	})
}

func (d *DB) GetAnalytics(cid string) (Analytics, error) {
	var a Analytics
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktAnalytics).Get([]byte(cid))
		if v == nil {
			return nil
		}
		return json.Unmarshal(v, &a)
	})
	return a, err
}

func (d *DB) SaveAnalytics(cid string, a Analytics) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktAnalytics), []byte(cid), a)
	})
}

func (d *DB) AllAnalytics() (map[string]Analytics, error) {
	out := make(map[string]Analytics)
	err := d.b.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAnalytics).ForEach(func(k, v []byte) error {
			var a Analytics
			if err := json.Unmarshal(v, &a); err != nil {
				return err
			}
			out[string(k)] = a
			return nil
		})
	})
	return out, err
}

func putJSON(bkt *bolt.Bucket, key []byte, val any) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return bkt.Put(key, b)
}

func (d *DB) SaveNicklock(gid, uid, nickname string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid)
		return tx.Bucket(bktNicklocks).Put(k, []byte(nickname))
	})
}

func (d *DB) GetNicklock(gid, uid string) (string, error) {
	var nick string
	err := d.b.View(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid)
		v := tx.Bucket(bktNicklocks).Get(k)
		if v == nil {
			return fmt.Errorf("no lock found")
		}
		nick = string(v)
		return nil
	})
	return nick, err
}

func (d *DB) DeleteNicklock(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid)
		return tx.Bucket(bktNicklocks).Delete(k)
	})
}

func (d *DB) SaveModlog(gid string, cfg ModlogCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktModlogs), []byte(gid), cfg)
	})
}

func (d *DB) GetModlog(gid string) (ModlogCfg, error) {
	var cfg ModlogCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktModlogs).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("no config found")
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
}

type Case struct {
	ID        int       `json:"id"`
	GuildID   string    `json:"guild_id"`
	UserID    string    `json:"user_id"`
	ModID     string    `json:"mod_id"`
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

func (d *DB) AddCase(gid string, c Case) (int, error) {
	var id int
	err := d.b.Update(func(tx *bolt.Tx) error {
		gbkt, err := tx.Bucket(bktCases).CreateBucketIfNotExists([]byte(gid))
		if err != nil {
			return err
		}
		seq, err := gbkt.NextSequence()
		if err != nil {
			return err
		}
		id = int(seq)
		c.ID = id
		c.GuildID = gid
		key := []byte(fmt.Sprintf("%06d", id))
		return putJSON(gbkt, key, c)
	})
	return id, err
}

func (d *DB) GetCase(gid string, id int) (Case, error) {
	var c Case
	err := d.b.View(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktCases).Bucket([]byte(gid))
		if gbkt == nil {
			return fmt.Errorf("case not found")
		}
		key := []byte(fmt.Sprintf("%06d", id))
		v := gbkt.Get(key)
		if v == nil {
			return fmt.Errorf("case not found")
		}
		return json.Unmarshal(v, &c)
	})
	return c, err
}

func (d *DB) DeleteCase(gid string, id int) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktCases).Bucket([]byte(gid))
		if gbkt == nil {
			return fmt.Errorf("case not found")
		}
		key := []byte(fmt.Sprintf("%06d", id))
		if gbkt.Get(key) == nil {
			return fmt.Errorf("case not found")
		}
		return gbkt.Delete(key)
	})
}

func (d *DB) DeleteAllCases(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktCases).Bucket([]byte(gid))
		if gbkt == nil {
			return nil
		}
		return gbkt.ForEach(func(k, v []byte) error {
			var c Case
			if err := json.Unmarshal(v, &c); err == nil && c.UserID == uid {
				if err := gbkt.Delete(k); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func (d *DB) ListCases(gid, uid string) ([]Case, error) {
	var out []Case
	err := d.b.View(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktCases).Bucket([]byte(gid))
		if gbkt == nil {
			return nil
		}
		return gbkt.ForEach(func(_, v []byte) error {
			var c Case
			if err := json.Unmarshal(v, &c); err == nil {
				if uid == "" || c.UserID == uid {
					out = append(out, c)
				}
			}
			return nil
		})
	})
	return out, err
}

func (d *DB) SaveJailed(gid, uid string, roles []string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		gbkt, err := tx.Bucket(bktJails).CreateBucketIfNotExists([]byte(gid))
		if err != nil {
			return err
		}
		return putJSON(gbkt, []byte(uid), roles)
	})
}

func (d *DB) GetJailed(gid, uid string) ([]string, error) {
	var roles []string
	err := d.b.View(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktJails).Bucket([]byte(gid))
		if gbkt == nil {
			return fmt.Errorf("not jailed")
		}
		v := gbkt.Get([]byte(uid))
		if v == nil {
			return fmt.Errorf("not jailed")
		}
		return json.Unmarshal(v, &roles)
	})
	return roles, err
}

func (d *DB) DeleteJailed(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktJails).Bucket([]byte(gid))
		if gbkt == nil {
			return nil
		}
		return gbkt.Delete([]byte(uid))
	})
}

func (d *DB) SaveInvoke(gid, trigger, template string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		gbkt, err := tx.Bucket(bktInvokes).CreateBucketIfNotExists([]byte(gid))
		if err != nil {
			return err
		}
		return gbkt.Put([]byte(strings.ToLower(trigger)), []byte(template))
	})
}

func (d *DB) GetInvoke(gid, trigger string) (string, error) {
	var template string
	err := d.b.View(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktInvokes).Bucket([]byte(gid))
		if gbkt == nil {
			return fmt.Errorf("invoke not found")
		}
		v := gbkt.Get([]byte(strings.ToLower(trigger)))
		if v == nil {
			return fmt.Errorf("invoke not found")
		}
		template = string(v)
		return nil
	})
	return template, err
}

func (d *DB) DeleteInvoke(gid, trigger string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktInvokes).Bucket([]byte(gid))
		if gbkt == nil {
			return nil
		}
		return gbkt.Delete([]byte(strings.ToLower(trigger)))
	})
}

func (d *DB) ListInvokes(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktInvokes).Bucket([]byte(gid))
		if gbkt == nil {
			return nil
		}
		return gbkt.ForEach(func(k, v []byte) error {
			out[string(k)] = string(v)
			return nil
		})
	})
	return out, err
}

func (d *DB) AddBypass(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		gbkt, err := tx.Bucket(bktAntinuke).CreateBucketIfNotExists([]byte(gid))
		if err != nil {
			return err
		}
		return gbkt.Put([]byte(uid), []byte("true"))
	})
}

func (d *DB) HasBypass(gid, uid string) bool {
	has := false
	_ = d.b.View(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktAntinuke).Bucket([]byte(gid))
		if gbkt == nil {
			return nil
		}
		v := gbkt.Get([]byte(uid))
		if v != nil && string(v) == "true" {
			has = true
		}
		return nil
	})
	return has
}

func (d *DB) DeleteBypass(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktAntinuke).Bucket([]byte(gid))
		if gbkt == nil {
			return nil
		}
		return gbkt.Delete([]byte(uid))
	})
}

func (d *DB) ListBypasses(gid string) ([]string, error) {
	var out []string
	err := d.b.View(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktAntinuke).Bucket([]byte(gid))
		if gbkt == nil {
			return nil
		}
		return gbkt.ForEach(func(k, v []byte) error {
			out = append(out, string(k))
			return nil
		})
	})
	return out, err
}

func (d *DB) SavePrefix(gid, prefix string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktPrefixes).Put([]byte(gid), []byte(prefix))
	})
}

func (d *DB) GetPrefix(gid string) (string, error) {
	var prefix string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktPrefixes).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("no custom prefix")
		}
		prefix = string(v)
		return nil
	})
	return prefix, err
}

func (d *DB) DeletePrefix(gid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktPrefixes).Delete([]byte(gid))
	})
}

type TempRole struct {
	GuildID   string    `json:"guild_id"`
	UserID    string    `json:"user_id"`
	RoleID    string    `json:"role_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (d *DB) SaveTempRole(tr TempRole) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(tr.GuildID + ":" + tr.UserID + ":" + tr.RoleID)
		return putJSON(tx.Bucket(bktTempRoles), k, tr)
	})
}

func (d *DB) GetExpiredTempRoles() ([]TempRole, error) {
	var out []TempRole
	now := time.Now()
	err := d.b.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bktTempRoles).ForEach(func(_, v []byte) error {
			var tr TempRole
			if err := json.Unmarshal(v, &tr); err == nil {
				if tr.ExpiresAt.Before(now) {
					out = append(out, tr)
				}
			}
			return nil
		})
	})
	return out, err
}

func (d *DB) DeleteTempRole(gid, uid, rid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid + ":" + rid)
		return tx.Bucket(bktTempRoles).Delete(k)
	})
}

func (d *DB) SaveStickyRole(gid, uid, rid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid + ":" + rid)
		return tx.Bucket(bktStickyRoles).Put(k, []byte("true"))
	})
}

func (d *DB) IsStickyRole(gid, uid, rid string) bool {
	has := false
	_ = d.b.View(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid + ":" + rid)
		v := tx.Bucket(bktStickyRoles).Get(k)
		if v != nil && string(v) == "true" {
			has = true
		}
		return nil
	})
	return has
}

func (d *DB) DeleteStickyRole(gid, uid, rid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid + ":" + rid)
		return tx.Bucket(bktStickyRoles).Delete(k)
	})
}

type StickyRoleEntry struct {
	UserID string
	RoleID string
}

func (d *DB) ListStickyRoles(gid string) ([]StickyRoleEntry, error) {
	var out []StickyRoleEntry
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(gid + ":")
		c := tx.Bucket(bktStickyRoles).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			if string(v) != "true" {
				continue
			}
			parts := strings.Split(string(k), ":")
			if len(parts) >= 3 {
				out = append(out, StickyRoleEntry{
					UserID: parts[1],
					RoleID: parts[2],
				})
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveAutoroles(gid string, roles []string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktAutoroles), []byte(gid), roles)
	})
}

func (d *DB) GetAutoroles(gid string) ([]string, error) {
	var roles []string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktAutoroles).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("no autoroles")
		}
		return json.Unmarshal(v, &roles)
	})
	return roles, err
}

type DailyCfg struct {
	ChannelID string `json:"channel_id"`
	Time      string `json:"time"` 
	Enabled   bool   `json:"enabled"`
}

func (d *DB) SaveDailyQuestion(gid string, cfg DailyCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktDailyQuestions), []byte(gid), cfg)
	})
}

func (d *DB) GetDailyQuestion(gid string) (DailyCfg, error) {
	var cfg DailyCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktDailyQuestions).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
}

func (d *DB) ListDailyQuestions() (map[string]DailyCfg, error) {
	out := make(map[string]DailyCfg)
	err := d.b.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bktDailyQuestions).ForEach(func(k, v []byte) error {
			var cfg DailyCfg
			if err := json.Unmarshal(v, &cfg); err == nil {
				out[string(k)] = cfg
			}
			return nil
		})
	})
	return out, err
}

func (d *DB) SaveDailyQuote(gid string, cfg DailyCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktDailyQuotes), []byte(gid), cfg)
	})
}

func (d *DB) GetDailyQuote(gid string) (DailyCfg, error) {
	var cfg DailyCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktDailyQuotes).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
}

func (d *DB) ListDailyQuotes() (map[string]DailyCfg, error) {
	out := make(map[string]DailyCfg)
	err := d.b.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bktDailyQuotes).ForEach(func(k, v []byte) error {
			var cfg DailyCfg
			if err := json.Unmarshal(v, &cfg); err == nil {
				out[string(k)] = cfg
			}
			return nil
		})
	})
	return out, err
}

func (d *DB) IncrementUserMessages(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktUserMessages)
		k := []byte(gid + ":" + uid)
		var val int
		v := bkt.Get(k)
		if v != nil {
			if num, err := strconv.Atoi(string(v)); err == nil {
				val = num
			}
		}
		val++
		return bkt.Put(k, []byte(strconv.Itoa(val)))
	})
}

func (d *DB) GetUserMessages(gid, uid string) int {
	val := 0
	_ = d.b.View(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid)
		v := tx.Bucket(bktUserMessages).Get(k)
		if v != nil {
			if num, err := strconv.Atoi(string(v)); err == nil {
				val = num
			}
		}
		return nil
	})
	return val
}

type MsgLeaderboardEntry struct {
	UserID string
	Count  int
}

func (d *DB) GetMessageLeaderboard(gid string) ([]MsgLeaderboardEntry, error) {
	var out []MsgLeaderboardEntry
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(gid + ":")
		c := tx.Bucket(bktUserMessages).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			parts := strings.Split(string(k), ":")
			if len(parts) >= 2 {
				count := 0
				if num, err := strconv.Atoi(string(v)); err == nil {
					count = num
				}
				out = append(out, MsgLeaderboardEntry{
					UserID: parts[1],
					Count:  count,
				})
			}
		}
		return nil
	})
	return out, err
}

type BoostCfg struct {
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
}

func (d *DB) SaveBoostCfg(gid string, cfg BoostCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktBoostCfg), []byte(gid), cfg)
	})
}

func (d *DB) GetBoostCfg(gid string) (BoostCfg, error) {
	var cfg BoostCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktBoostCfg).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
}

func (d *DB) SaveBoosterBase(gid, roleID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBoosterRoles).Put([]byte("base:"+gid), []byte(roleID))
	})
}

func (d *DB) GetBoosterBase(gid string) (string, error) {
	var roleID string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktBoosterRoles).Get([]byte("base:"+gid))
		if v == nil {
			return fmt.Errorf("no base role configured")
		}
		roleID = string(v)
		return nil
	})
	return roleID, err
}

func (d *DB) SaveUserBoosterRole(gid, uid, roleID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBoosterRoles).Put([]byte("user:"+gid+":"+uid), []byte(roleID))
	})
}

func (d *DB) GetUserBoosterRole(gid, uid string) (string, error) {
	var roleID string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktBoosterRoles).Get([]byte("user:"+gid+":"+uid))
		if v == nil {
			return fmt.Errorf("no custom role")
		}
		roleID = string(v)
		return nil
	})
	return roleID, err
}

func (d *DB) DeleteUserBoosterRole(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBoosterRoles).Delete([]byte("user:"+gid+":"+uid))
	})
}

func (d *DB) ListUserBoosterRoles(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte("user:" + gid + ":")
		c := tx.Bucket(bktBoosterRoles).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			parts := strings.Split(string(k), ":")
			if len(parts) >= 3 {
				out[parts[2]] = string(v)
			}
		}
		return nil
	})
	return out, err
}

type HallCfg struct {
	FameChannelID  string `json:"fame_channel_id"`
	FameThreshold  int    `json:"fame_threshold"`
	ShameChannelID string `json:"shame_shannel_id"`
	ShameThreshold int    `json:"shame_threshold"`
}

func (d *DB) SaveHallCfg(gid string, cfg HallCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktHallCfg), []byte(gid), cfg)
	})
}

func (d *DB) GetHallCfg(gid string) (HallCfg, error) {
	var cfg HallCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktHallCfg).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
}

func (d *DB) IsHallPosted(gid, msgID, postType string) (bool, error) {
	posted := false
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktHallMessages).Get([]byte(postType + ":" + gid + ":" + msgID))
		if v != nil {
			posted = true
		}
		return nil
	})
	return posted, err
}

func (d *DB) SetHallPosted(gid, msgID, postType string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktHallMessages).Put([]byte(postType+":"+gid+":"+msgID), []byte("1"))
	})
}

type AFKStatus struct {
	Reason string    `json:"reason"`
	Time   time.Time `json:"time"`
	Pings  int       `json:"pings"`
}

func (d *DB) SaveAFK(gid, uid string, status AFKStatus) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktAFK), []byte(gid+":"+uid), status)
	})
}

func (d *DB) GetAFK(gid, uid string) (AFKStatus, error) {
	var status AFKStatus
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktAFK).Get([]byte(gid + ":" + uid))
		if v == nil {
			return fmt.Errorf("no afk status")
		}
		return json.Unmarshal(v, &status)
	})
	return status, err
}

func (d *DB) DeleteAFK(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAFK).Delete([]byte(gid + ":" + uid))
	})
}

func (d *DB) SaveAutoreact(gid, trigger, emoji string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAutoreact).Put([]byte(gid+":"+strings.ToLower(trigger)), []byte(emoji))
	})
}

func (d *DB) DeleteAutoreact(gid, trigger string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAutoreact).Delete([]byte(gid + ":" + strings.ToLower(trigger)))
	})
}

func (d *DB) ListAutoreact(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(gid + ":")
		c := tx.Bucket(bktAutoreact).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			parts := strings.Split(string(k), ":")
			if len(parts) >= 2 {
				out[parts[1]] = string(v)
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveAutoresponder(gid, trigger, response string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAutorespond).Put([]byte(gid+":"+strings.ToLower(trigger)), []byte(response))
	})
}

func (d *DB) DeleteAutoresponder(gid, trigger string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAutorespond).Delete([]byte(gid + ":" + strings.ToLower(trigger)))
	})
}

func (d *DB) ListAutoresponder(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(gid + ":")
		c := tx.Bucket(bktAutorespond).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			parts := strings.Split(string(k), ":")
			if len(parts) >= 2 {
				out[parts[1]] = string(v)
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveBirthday(gid, uid, date string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBirthdays).Put([]byte(gid+":"+uid), []byte(date))
	})
}

func (d *DB) GetBirthday(gid, uid string) (string, error) {
	var val string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktBirthdays).Get([]byte(gid + ":" + uid))
		if v == nil {
			return fmt.Errorf("no birthday set")
		}
		val = string(v)
		return nil
	})
	return val, err
}

func (d *DB) ListBirthdays(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(gid + ":")
		c := tx.Bucket(bktBirthdays).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			parts := strings.Split(string(k), ":")
			if len(parts) >= 2 {
				out[parts[1]] = string(v)
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveTimezone(uid, tz string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktTimezones).Put([]byte(uid), []byte(tz))
	})
}

func (d *DB) GetTimezone(uid string) (string, error) {
	var val string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktTimezones).Get([]byte(uid))
		if v == nil {
			return fmt.Errorf("no timezone set")
		}
		val = string(v)
		return nil
	})
	return val, err
}

func (d *DB) SaveBirthdayChannel(gid, cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBirthdayAnn).Put([]byte(gid), []byte(cid))
	})
}

func (d *DB) GetBirthdayChannel(gid string) (string, error) {
	var val string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktBirthdayAnn).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("no channel set")
		}
		val = string(v)
		return nil
	})
	return val, err
}

func (d *DB) SaveLastBirthdayWished(gid, uid string, year int) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBdayWished).Put([]byte(gid+":"+uid), []byte(strconv.Itoa(year)))
	})
}

func (d *DB) GetLastBirthdayWished(gid, uid string) (int, error) {
	var year int
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktBdayWished).Get([]byte(gid + ":" + uid))
		if v == nil {
			return fmt.Errorf("never wished")
		}
		if num, err := strconv.Atoi(string(v)); err == nil {
			year = num
		} else {
			return err
		}
		return nil
	})
	return year, err
}

type BumpCfg struct {
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	Enabled   bool   `json:"enabled"`
}

func (d *DB) SaveBumpCfg(gid string, cfg BumpCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktBumpReminder), []byte(gid), cfg)
	})
}

func (d *DB) GetBumpCfg(gid string) (BumpCfg, error) {
	var cfg BumpCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktBumpReminder).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
}

func (d *DB) SaveButtonRole(gid, msgID, customID, roleID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktButtonRoles).Put([]byte(gid+":"+msgID+":"+customID), []byte(roleID))
	})
}

func (d *DB) GetButtonRole(gid, msgID, customID string) (string, error) {
	var roleID string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktButtonRoles).Get([]byte(gid + ":" + msgID + ":" + customID))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		roleID = string(v)
		return nil
	})
	return roleID, err
}

func (d *DB) SaveReactRole(gid, msgID, emoji, roleID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktReactRoles).Put([]byte(gid+":"+msgID+":"+emoji), []byte(roleID))
	})
}

func (d *DB) GetReactRole(gid, msgID, emoji string) (string, error) {
	var roleID string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktReactRoles).Get([]byte(gid + ":" + msgID + ":" + emoji))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		roleID = string(v)
		return nil
	})
	return roleID, err
}

func (d *DB) DeleteReactRole(gid, msgID, emoji string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktReactRoles).Delete([]byte(gid + ":" + msgID + ":" + emoji))
	})
}

type Reminder struct {
	ID      string    `json:"id"`
	UserID  string    `json:"user_id"`
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

func (d *DB) SaveReminder(r Reminder) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktReminders), []byte(r.UserID+":"+r.ID), r)
	})
}

func (d *DB) DeleteReminder(uid, id string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktReminders).Delete([]byte(uid + ":" + id))
	})
}

func (d *DB) ListReminders(uid string) ([]Reminder, error) {
	var out []Reminder
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(uid + ":")
		c := tx.Bucket(bktReminders).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			var r Reminder
			if json.Unmarshal(v, &r) == nil {
				out = append(out, r)
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) ListAllReminders() ([]Reminder, error) {
	var out []Reminder
	err := d.b.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bktReminders).ForEach(func(k, v []byte) error {
			var r Reminder
			if json.Unmarshal(v, &r) == nil {
				out = append(out, r)
			}
			return nil
		})
	})
	return out, err
}

type ScheduledMsg struct {
	ID        string    `json:"id"`
	GuildID   string    `json:"guild_id"`
	ChannelID string    `json:"channel_id"`
	Time      time.Time `json:"time"`
	Message   string    `json:"message"`
}

func (d *DB) SaveSchedule(s ScheduledMsg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktSchedules), []byte(s.GuildID+":"+s.ID), s)
	})
}

func (d *DB) DeleteSchedule(gid, id string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktSchedules).Delete([]byte(gid + ":" + id))
	})
}

func (d *DB) ListSchedules(gid string) ([]ScheduledMsg, error) {
	var out []ScheduledMsg
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(gid + ":")
		c := tx.Bucket(bktSchedules).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			var s ScheduledMsg
			if json.Unmarshal(v, &s) == nil {
				out = append(out, s)
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) ListAllSchedules() ([]ScheduledMsg, error) {
	var out []ScheduledMsg
	err := d.b.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bktSchedules).ForEach(func(k, v []byte) error {
			var s ScheduledMsg
			if json.Unmarshal(v, &s) == nil {
				out = append(out, s)
			}
			return nil
		})
	})
	return out, err
}

type StarboardCfg struct {
	ChannelID string `json:"channel_id"`
	Threshold int    `json:"threshold"`
	Enabled   bool   `json:"enabled"`
}

func (d *DB) SaveStarboardCfg(gid string, cfg StarboardCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktStarboardCfg), []byte(gid), cfg)
	})
}

func (d *DB) GetStarboardCfg(gid string) (StarboardCfg, error) {
	var cfg StarboardCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktStarboardCfg).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
}

func (d *DB) SaveStarboardMsg(origID, sbID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktStarboardMsg).Put([]byte(origID), []byte(sbID))
	})
}

func (d *DB) GetStarboardMsg(origID string) (string, error) {
	var val string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktStarboardMsg).Get([]byte(origID))
		if v == nil {
			return fmt.Errorf("not found")
		}
		val = string(v)
		return nil
	})
	return val, err
}

func (d *DB) SaveTag(gid, name, content string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktTags).Put([]byte(gid+":"+strings.ToLower(name)), []byte(content))
	})
}

func (d *DB) GetTag(gid, name string) (string, error) {
	var val string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktTags).Get([]byte(gid + ":" + strings.ToLower(name)))
		if v == nil {
			return fmt.Errorf("not found")
		}
		val = string(v)
		return nil
	})
	return val, err
}

func (d *DB) DeleteTag(gid, name string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktTags).Delete([]byte(gid + ":" + strings.ToLower(name)))
	})
}

func (d *DB) ListTags(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(gid + ":")
		c := tx.Bucket(bktTags).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			parts := strings.Split(string(k), ":")
			if len(parts) >= 2 {
				out[parts[1]] = string(v)
			}
		}
		return nil
	})
	return out, err
}

type TempVoiceCfg struct {
	ParentChannelID string `json:"parent_channel_id"`
	CategoryID      string `json:"category_id"`
	Enabled         bool   `json:"enabled"`
}

func (d *DB) SaveTempVoiceCfg(gid string, cfg TempVoiceCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktTempVoiceCfg), []byte(gid), cfg)
	})
}

func (d *DB) GetTempVoiceCfg(gid string) (TempVoiceCfg, error) {
	var cfg TempVoiceCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktTempVoiceCfg).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
}

func (d *DB) SaveTempVoiceChan(chanID, ownerID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktTempVoiceChan).Put([]byte(chanID), []byte(ownerID))
	})
}

func (d *DB) GetTempVoiceChan(chanID string) (string, error) {
	var val string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktTempVoiceChan).Get([]byte(chanID))
		if v == nil {
			return fmt.Errorf("not found")
		}
		val = string(v)
		return nil
	})
	return val, err
}

func (d *DB) DeleteTempVoiceChan(chanID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktTempVoiceChan).Delete([]byte(chanID))
	})
}

type VanityCfg struct {
	Text    string `json:"text"`
	RoleID  string `json:"role_id"`
	Enabled bool   `json:"enabled"`
}

func (d *DB) SaveVanityCfg(gid string, cfg VanityCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktVanityCfg), []byte(gid), cfg)
	})
}

func (d *DB) GetVanityCfg(gid string) (VanityCfg, error) {
	var cfg VanityCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktVanityCfg).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
}

type Vouch struct {
	TargetUserID string `json:"target_user_id"`
	VoucherID    string `json:"voucher_id"`
	Comment      string `json:"comment"`
	Time         int64  `json:"time"`
}

func (d *DB) SaveVouch(v Vouch) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktVouches), []byte(v.TargetUserID+":"+v.VoucherID), v)
	})
}

func (d *DB) ListVouches(uid string) ([]Vouch, error) {
	var out []Vouch
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(uid + ":")
		c := tx.Bucket(bktVouches).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			var vc Vouch
			if json.Unmarshal(v, &vc) == nil {
				out = append(out, vc)
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) DeleteVouch(targetUID, voucherUID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktVouches).Delete([]byte(targetUID + ":" + voucherUID))
	})
}

type WeedPlant struct {
	Growth         float64   `json:"growth"`
	Water          float64   `json:"water"`
	Fertilizer     float64   `json:"fertilizer"`
	LastAction     time.Time `json:"last_action"`
}

func (d *DB) SaveWeedPlant(gid string, wp WeedPlant) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktGlobal), []byte("weed:"+gid), wp)
	})
}

func (d *DB) GetWeedPlant(gid string) (WeedPlant, error) {
	var wp WeedPlant
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktGlobal).Get([]byte("weed:" + gid))
		if v == nil {
			return fmt.Errorf("no plant")
		}
		return json.Unmarshal(v, &wp)
	})
	return wp, err
}

type LoggerSub struct {
	GuildID    string `json:"guild_id"`
	ChannelID  string `json:"channel_id"`
	Category   string `json:"category"`
	EmbedColor string `json:"embed_color,omitempty"`
}

type LoggerIgnore struct {
	GuildID    string `json:"guild_id"`
	TargetID   string `json:"target_id"`
	TargetType string `json:"target_type"`
}

func (d *DB) SaveLoggerSub(sub LoggerSub) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(sub.GuildID + ":" + sub.ChannelID + ":" + sub.Category)
		v, err := json.Marshal(sub)
		if err != nil {
			return err
		}
		return tx.Bucket(bktLoggerSubs).Put(k, v)
	})
}

func (d *DB) DeleteLoggerSub(gid, cid, cat string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + cid + ":" + cat)
		return tx.Bucket(bktLoggerSubs).Delete(k)
	})
}

func (d *DB) DeleteAllLoggerSubs(gid, cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktLoggerSubs)
		c := b.Cursor()
		prefix := []byte(gid + ":" + cid + ":")
		for k, _ := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, _ = c.Next() {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *DB) GetLoggerSubs(gid, cat string) ([]LoggerSub, error) {
	var out []LoggerSub
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktLoggerSubs)
		c := b.Cursor()
		prefix := []byte(gid + ":")
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			var sub LoggerSub
			if err := json.Unmarshal(v, &sub); err == nil {
				if sub.Category == cat {
					out = append(out, sub)
				}
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) GetChannelLoggerSubs(gid, cid string) ([]LoggerSub, error) {
	var out []LoggerSub
	prefix := []byte(gid + ":" + cid + ":")
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktLoggerSubs)
		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			var sub LoggerSub
			if err := json.Unmarshal(v, &sub); err == nil {
				out = append(out, sub)
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveLoggerIgnore(ig LoggerIgnore) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(ig.GuildID + ":" + ig.TargetID)
		v, err := json.Marshal(ig)
		if err != nil {
			return err
		}
		return tx.Bucket(bktLoggerIgnores).Put(k, v)
	})
}

func (d *DB) DeleteLoggerIgnore(gid, targetID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + targetID)
		return tx.Bucket(bktLoggerIgnores).Delete(k)
	})
}

func (d *DB) IsLoggerIgnored(gid, targetID string) bool {
	ignored := false
	_ = d.b.View(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + targetID)
		v := tx.Bucket(bktLoggerIgnores).Get(k)
		if v != nil {
			ignored = true
		}
		return nil
	})
	return ignored
}

func (d *DB) GetLoggerIgnores(gid string) ([]LoggerIgnore, error) {
	var out []LoggerIgnore
	prefix := []byte(gid + ":")
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktLoggerIgnores)
		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			var ig LoggerIgnore
			if err := json.Unmarshal(v, &ig); err == nil {
				out = append(out, ig)
			}
		}
		return nil
	})
	return out, err
}

type AntispamCfg struct {
	Enabled     bool     `json:"enabled"`
	Limit       int      `json:"limit"`
	Seconds     int      `json:"seconds"`
	Action      string   `json:"action"`
	TimeoutSecs int      `json:"timeout_secs"`
	Whitelist   []string `json:"whitelist"`
	BypassPerms bool     `json:"bypass_perms"`
}

func (d *DB) SaveAntispamCfg(gid string, cfg AntispamCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		v, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return tx.Bucket(bktAntispam).Put([]byte(gid), v)
	})
}

func (d *DB) GetAntispamCfg(gid string) (AntispamCfg, error) {
	cfg := AntispamCfg{
		BypassPerms: false,
	}
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktAntispam).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not found")
		}
		return json.Unmarshal(v, &cfg)
	})
	if err != nil {
		cfg = AntispamCfg{
			Enabled:     false,
			Limit:       5,
			Seconds:     3,
			Action:      "timeout",
			TimeoutSecs: 600,
			BypassPerms: false,
		}
	}
	if cfg.TimeoutSecs == 0 {
		cfg.TimeoutSecs = 600
	}
	return cfg, nil
}

type FilterCfg struct {
	Enabled      bool     `json:"enabled"`
	BlockedWords []string `json:"blocked_words"`
	AllowedWords []string `json:"allowed_words"`
	Regexes      []string `json:"regexes"`
	BypassPerms  bool     `json:"bypass_perms"`
	Whitelist    []string `json:"whitelist"`
}

func (d *DB) SaveFilterCfg(gid string, cfg FilterCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		v, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return tx.Bucket(bktFilters).Put([]byte(gid), v)
	})
}

func (d *DB) GetFilterCfg(gid string) (FilterCfg, error) {
	cfg := FilterCfg{
		BypassPerms: false,
	}
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktFilters).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not found")
		}
		return json.Unmarshal(v, &cfg)
	})
	if err != nil {
		cfg = FilterCfg{
			Enabled:     false,
			BypassPerms: false,
		}
	}
	return cfg, nil
}

type PalantirCfg struct {
	Enabled         bool     `json:"enabled"`
	BlockedGuilds   []string `json:"blocked_guilds"`
	BlockedChannels []string `json:"blocked_channels"`
	BlockedUsers    []string `json:"blocked_users"`
	BlockedEvents   []string `json:"blocked_events"`
}

func (d *DB) SavePalantirCfg(cfg PalantirCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		v, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return tx.Bucket(bktPalantirCfg).Put([]byte("global"), v)
	})
}

func (d *DB) GetPalantirCfg() (PalantirCfg, error) {
	var cfg PalantirCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktPalantirCfg).Get([]byte("global"))
		if v == nil {
			return fmt.Errorf("not found")
		}
		return json.Unmarshal(v, &cfg)
	})
	if err != nil {
		cfg = PalantirCfg{
			Enabled: true,
		}
	}
	return cfg, nil
}

type AntilinkCfg struct {
	Enabled          bool     `json:"enabled"`
	Action           string   `json:"action"`
	TimeoutSecs      int      `json:"timeout_secs"`
	BypassPerms      bool     `json:"bypass_perms"`
	Whitelist        []string `json:"whitelist"`
	AllowedDomains   []string `json:"allowed_domains"`
	BlockedDomains   []string `json:"blocked_domains"`
	BlockInvitesOnly bool     `json:"block_invites_only"`
}

func (d *DB) SaveAntilinkCfg(gid string, cfg AntilinkCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		v, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return tx.Bucket(bktAntilink).Put([]byte(gid), v)
	})
}

func (d *DB) GetAntilinkCfg(gid string) (AntilinkCfg, error) {
	cfg := AntilinkCfg{
		BypassPerms: false,
	}
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktAntilink).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not found")
		}
		return json.Unmarshal(v, &cfg)
	})
	if err != nil {
		cfg = AntilinkCfg{
			Enabled:          false,
			Action:           "delete",
			TimeoutSecs:      600,
			BypassPerms:      false,
			BlockInvitesOnly: false,
		}
	}
	if cfg.TimeoutSecs == 0 {
		cfg.TimeoutSecs = 600
	}
	return cfg, nil
}



