package storage

import (
	"encoding/json"
	"fmt"
	"strings"

	bolt "go.etcd.io/bbolt"
)

type ModlogCfg struct {
	ChannelID  string `json:"channel_id"`
	LogDiscord bool   `json:"log_discord"`
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
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	return cfg, err
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

type AntinukeCfg struct {
	Enabled   bool   `json:"enabled"`
	ChanLimit int    `json:"chan_limit"`
	ChanSecs  int    `json:"chan_secs"`
	RoleLimit int    `json:"role_limit"`
	RoleSecs  int    `json:"role_secs"`
	BanLimit  int    `json:"ban_limit"`
	BanSecs   int    `json:"ban_secs"`
	KickLimit int    `json:"kick_limit"`
	KickSecs  int    `json:"kick_secs"`
	BotLimit  int    `json:"bot_limit"`
	BotSecs   int    `json:"bot_secs"`
	Action    string `json:"action"` // "strip", "ban", "kick"
}

func (d *DB) SaveAntinukeCfg(gid string, cfg AntinukeCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktAntinukeCfg), []byte(gid), cfg)
	})
}

func (d *DB) GetAntinukeCfg(gid string) (AntinukeCfg, error) {
	var cfg AntinukeCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktAntinukeCfg).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	if err != nil {
		cfg = AntinukeCfg{
			Enabled:   false,
			ChanLimit: 4,
			ChanSecs:  10,
			RoleLimit: 4,
			RoleSecs:  10,
			BanLimit:  4,
			BanSecs:   10,
			KickLimit: 4,
			KickSecs:  10,
			BotLimit:  2,
			BotSecs:   10,
			Action:    "strip",
		}
	}
	return cfg, nil
}

type AntiraidCfg struct {
	Enabled   bool   `json:"enabled"`
	JoinLimit int    `json:"join_limit"`
	Seconds   int    `json:"seconds"`
	Action    string `json:"action"` // "notify", "lockdown", "kick", "ban"
}

func (d *DB) SaveAntiraidCfg(gid string, cfg AntiraidCfg) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktAntiraidCfg), []byte(gid), cfg)
	})
}

func (d *DB) GetAntiraidCfg(gid string) (AntiraidCfg, error) {
	var cfg AntiraidCfg
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktAntiraidCfg).Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("not configured")
		}
		return json.Unmarshal(v, &cfg)
	})
	if err != nil {
		cfg = AntiraidCfg{
			Enabled:   false,
			JoinLimit: 10,
			Seconds:   10,
			Action:    "notify",
		}
	}
	return cfg, nil
}

