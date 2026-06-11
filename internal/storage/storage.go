package storage

import (
	"encoding/json"
	"fmt"
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
	bktAntinukeCfg  = []byte("AntinukeCfg")
	bktAntiraidCfg  = []byte("AntiraidCfg")

	keyGlobal = []byte("cfg")
)

type Analytics struct {
	TotalCmds  int64 `json:"total_cmds"`
	PrefixCmds int64 `json:"prefix_cmds"`
	SlashCmds  int64 `json:"slash_cmds"`
	GuildCount int   `json:"guild_count"`
}

type DB struct {
	b *bolt.DB
}

func Open(path string) (*DB, error) {
	opts := &bolt.Options{
		Timeout:         1 * time.Second,
		NoSync:          true,
		FreelistType:    bolt.FreelistMapType,
		InitialMmapSize: 1 << 30, // 1GB
	}
	b, err := bolt.Open(path, 0600, opts)
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
			bktAntinukeCfg, bktAntiraidCfg,
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
	return d.b.Batch(func(tx *bolt.Tx) error {
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

func (d *DB) Update(fn func(tx *bolt.Tx) error) error {
	return d.b.Update(fn)
}

func (d *DB) View(fn func(tx *bolt.Tx) error) error {
	return d.b.View(fn)
}

