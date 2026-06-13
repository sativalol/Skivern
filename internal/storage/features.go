package storage

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

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
	return d.b.Batch(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktUserMessages)
		k := []byte(gid + ":" + uid)
		v := bkt.Get(k)
		n := 1
		if v != nil {
			if i, err := strconv.Atoi(string(v)); err == nil {
				n = i + 1
			}
		}
		return bkt.Put(k, []byte(strconv.Itoa(n)))
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
	Growth     float64   `json:"growth"`
	Water      float64   `json:"water"`
	Fertilizer float64   `json:"fertilizer"`
	LastAction time.Time `json:"last_action"`
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

func (d *DB) SaveInvoke(gid, trigger, template string) error {
	return d.b.Batch(func(tx *bolt.Tx) error {
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

type TouchRecord struct {
	GuildID string `json:"guild_id"`
	UserID  string `json:"user_id"`
	Sent    int    `json:"sent"`
	Recv    int    `json:"recv"`
}

func (d *DB) GetTouch(gid, uid string) (TouchRecord, error) {
	var tr TouchRecord
	err := d.b.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktTouches)
		v := bkt.Get([]byte(gid + ":" + uid))
		if v == nil {
			tr = TouchRecord{GuildID: gid, UserID: uid}
			return nil
		}
		return json.Unmarshal(v, &tr)
	})
	return tr, err
}

func (d *DB) RecordTouch(gid, sender, receiver string) (TouchRecord, TouchRecord, error) {
	var sRec, rRec TouchRecord
	err := d.b.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktTouches)
		
		sv := bkt.Get([]byte(gid + ":" + sender))
		if sv != nil {
			_ = json.Unmarshal(sv, &sRec)
		} else {
			sRec = TouchRecord{GuildID: gid, UserID: sender}
		}
		sRec.Sent++
		
		if sender != receiver {
			rv := bkt.Get([]byte(gid + ":" + receiver))
			if rv != nil {
				_ = json.Unmarshal(rv, &rRec)
			} else {
				rRec = TouchRecord{GuildID: gid, UserID: receiver}
			}
			rRec.Recv++
		} else {
			sRec.Recv++
			rRec = sRec
		}

		sBytes, _ := json.Marshal(sRec)
		if err := bkt.Put([]byte(gid+":"+sender), sBytes); err != nil {
			return err
		}

		if sender != receiver {
			rBytes, _ := json.Marshal(rRec)
			if err := bkt.Put([]byte(gid+":"+receiver), rBytes); err != nil {
				return err
			}
		}
		return nil
	})
	return sRec, rRec, err
}

func (d *DB) ListTouches(gid string) ([]TouchRecord, error) {
	var out []TouchRecord
	err := d.b.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktTouches)
		prefix := []byte(gid + ":")
		c := bkt.Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			var tr TouchRecord
			if err := json.Unmarshal(v, &tr); err == nil {
				out = append(out, tr)
			}
		}
		return nil
	})
	return out, err
}

type GuildSettings struct {
	BaseRoleID     string `json:"base_role_id"`
	JoinLogsChanID string `json:"join_logs_chan_id"`
	JailRoleID     string `json:"jail_role_id"`
	JailChanID     string `json:"jail_chan_id"`
	JailMessage    string `json:"jail_message"`
	JailRoles      bool   `json:"jail_roles"`
	Autoplay       string `json:"autoplay"`
	MaxShares      int    `json:"max_shares"`
	BoosterLimit   int    `json:"booster_limit"`
}

func (d *DB) GetGuildSettings(gid string) (GuildSettings, error) {
	cfg := GuildSettings{
		MaxShares: 5,
	}
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktGuildSettings)
		if b == nil {
			return fmt.Errorf("no bucket")
		}
		v := b.Get([]byte(gid))
		if v == nil {
			return fmt.Errorf("no config found")
		}
		return json.Unmarshal(v, &cfg)
	})
	if err != nil {
		return GuildSettings{MaxShares: 5}, nil
	}
	if cfg.MaxShares == 0 {
		cfg.MaxShares = 5
	}
	return cfg, nil
}

func (d *DB) SaveGuildSettings(gid string, cfg GuildSettings) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktGuildSettings), []byte(gid), cfg)
	})
}

func (d *DB) SaveAlias(gid, trigger, target string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAliases).Put([]byte(gid+":"+trigger), []byte(target))
	})
}

func (d *DB) GetAlias(gid, trigger string) (string, error) {
	var target string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktAliases).Get([]byte(gid + ":" + trigger))
		if v == nil {
			return fmt.Errorf("no alias")
		}
		target = string(v)
		return nil
	})
	return target, err
}

func (d *DB) DeleteAlias(gid, trigger string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAliases).Delete([]byte(gid + ":" + trigger))
	})
}

func (d *DB) DeleteAllAliases(gid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktAliases)
		c := b.Cursor()
		prefix := gid + ":"
		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
			_ = b.Delete(k)
		}
		return nil
	})
}

func (d *DB) ListAliases(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktAliases)
		c := b.Cursor()
		prefix := gid + ":"
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			trigger := strings.TrimPrefix(string(k), prefix)
			out[trigger] = string(v)
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveWelcomeMsg(gid, cid, msg string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktWelcomeMsgs).Put([]byte(gid+":"+cid), []byte(msg))
	})
}

func (d *DB) GetWelcomeMsg(gid, cid string) (string, error) {
	var msg string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktWelcomeMsgs).Get([]byte(gid + ":" + cid))
		if v == nil {
			return fmt.Errorf("not found")
		}
		msg = string(v)
		return nil
	})
	return msg, err
}

func (d *DB) DeleteWelcomeMsg(gid, cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktWelcomeMsgs).Delete([]byte(gid + ":" + cid))
	})
}

func (d *DB) ListWelcomeMsgs(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktWelcomeMsgs)
		c := b.Cursor()
		prefix := gid + ":"
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			cid := strings.TrimPrefix(string(k), prefix)
			out[cid] = string(v)
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveGoodbyeMsg(gid, cid, msg string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktGoodbyeMsgs).Put([]byte(gid+":"+cid), []byte(msg))
	})
}

func (d *DB) GetGoodbyeMsg(gid, cid string) (string, error) {
	var msg string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktGoodbyeMsgs).Get([]byte(gid + ":" + cid))
		if v == nil {
			return fmt.Errorf("not found")
		}
		msg = string(v)
		return nil
	})
	return msg, err
}

func (d *DB) DeleteGoodbyeMsg(gid, cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktGoodbyeMsgs).Delete([]byte(gid + ":" + cid))
	})
}

func (d *DB) ListGoodbyeMsgs(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktGoodbyeMsgs)
		c := b.Cursor()
		prefix := gid + ":"
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			cid := strings.TrimPrefix(string(k), prefix)
			out[cid] = string(v)
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveBoostMsg(gid, cid, msg string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBoostMsgs).Put([]byte(gid+":"+cid), []byte(msg))
	})
}

func (d *DB) GetBoostMsg(gid, cid string) (string, error) {
	var msg string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktBoostMsgs).Get([]byte(gid + ":" + cid))
		if v == nil {
			return fmt.Errorf("not found")
		}
		msg = string(v)
		return nil
	})
	return msg, err
}

func (d *DB) DeleteBoostMsg(gid, cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBoostMsgs).Delete([]byte(gid + ":" + cid))
	})
}

func (d *DB) ListBoostMsgs(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktBoostMsgs)
		c := b.Cursor()
		prefix := gid + ":"
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			cid := strings.TrimPrefix(string(k), prefix)
			out[cid] = string(v)
		}
		return nil
	})
	return out, err
}

type StickyMessage struct {
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	LastMsgID string `json:"last_msg_id"`
}

func (d *DB) SaveStickyMessage(gid, cid string, sm StickyMessage) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return putJSON(tx.Bucket(bktStickyMsgs), []byte(gid+":"+cid), sm)
	})
}

func (d *DB) GetStickyMessage(gid, cid string) (StickyMessage, error) {
	var sm StickyMessage
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktStickyMsgs).Get([]byte(gid + ":" + cid))
		if v == nil {
			return fmt.Errorf("not found")
		}
		return json.Unmarshal(v, &sm)
	})
	return sm, err
}

func (d *DB) DeleteStickyMessage(gid, cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktStickyMsgs).Delete([]byte(gid + ":" + cid))
	})
}

func (d *DB) ListStickyMessages(gid string) ([]StickyMessage, error) {
	var out []StickyMessage
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktStickyMsgs)
		c := b.Cursor()
		prefix := gid + ":"
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			var sm StickyMessage
			if json.Unmarshal(v, &sm) == nil {
				out = append(out, sm)
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveImgOnlyChannel(gid, cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktImgOnlyChans).Put([]byte(gid+":"+cid), []byte{1})
	})
}

func (d *DB) IsImgOnlyChannel(gid, cid string) (bool, error) {
	var exists bool
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktImgOnlyChans).Get([]byte(gid + ":" + cid))
		exists = (v != nil)
		return nil
	})
	return exists, err
}

func (d *DB) DeleteImgOnlyChannel(gid, cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktImgOnlyChans).Delete([]byte(gid + ":" + cid))
	})
}

func (d *DB) ListImgOnlyChannels(gid string) ([]string, error) {
	var out []string
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktImgOnlyChans)
		c := b.Cursor()
		prefix := gid + ":"
		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
			cid := strings.TrimPrefix(string(k), prefix)
			out = append(out, cid)
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveUserPrefix(uid, prefix string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktPrefixes).Put([]byte("user:"+uid), []byte(prefix))
	})
}

func (d *DB) GetUserPrefix(uid string) (string, error) {
	var prefix string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktPrefixes).Get([]byte("user:" + uid))
		if v == nil {
			return fmt.Errorf("no user prefix")
		}
		prefix = string(v)
		return nil
	})
	return prefix, err
}

func (d *DB) DeleteUserPrefix(uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktPrefixes).Delete([]byte("user:" + uid))
	})
}

func (d *DB) SaveBoosterShare(gid, ownerID, targetID, roleID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBoosterShares).Put([]byte(gid+":"+ownerID+":"+targetID), []byte(roleID))
	})
}

func (d *DB) DeleteBoosterShare(gid, ownerID, targetID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBoosterShares).Delete([]byte(gid + ":" + ownerID + ":" + targetID))
	})
}

func (d *DB) ListBoosterSharesForOwner(gid, ownerID string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktBoosterShares)
		c := b.Cursor()
		prefix := gid + ":" + ownerID + ":"
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			parts := strings.Split(string(k), ":")
			if len(parts) >= 3 {
				targetID := parts[2]
				out[targetID] = string(v)
			}
		}
		return nil
	})
	return out, err
}

func (d *DB) ListAllBoosterShares(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktBoosterShares)
		c := b.Cursor()
		prefix := gid + ":"
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			out[string(k)] = string(v)
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveBoosterFilter(gid, word string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBoosterFilters).Put([]byte(gid+":"+strings.ToLower(word)), []byte{1})
	})
}

func (d *DB) DeleteBoosterFilter(gid, word string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktBoosterFilters).Delete([]byte(gid + ":" + strings.ToLower(word)))
	})
}

func (d *DB) ListBoosterFilters(gid string) ([]string, error) {
	var out []string
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktBoosterFilters)
		c := b.Cursor()
		prefix := gid + ":"
		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
			word := strings.TrimPrefix(string(k), prefix)
			out = append(out, word)
		}
		return nil
	})
	return out, err
}

type Note struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Moderator string    `json:"moderator"`
	Timestamp time.Time `json:"timestamp"`
}

func (d *DB) SaveNote(gid, uid, text, moderatorID string) (string, error) {
	noteID := fmt.Sprintf("%04x", time.Now().UnixNano()&0xffff)
	note := Note{
		ID:        noteID,
		Text:      text,
		Moderator: moderatorID,
		Timestamp: time.Now(),
	}

	err := d.b.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktNotes)
		key := []byte(gid + ":" + uid)
		var notes []Note
		v := bkt.Get(key)
		if v != nil {
			_ = json.Unmarshal(v, &notes)
		}
		notes = append(notes, note)
		return putJSON(bkt, key, notes)
	})
	return noteID, err
}

func (d *DB) DeleteNote(gid, uid, noteID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktNotes)
		key := []byte(gid + ":" + uid)
		var notes []Note
		v := bkt.Get(key)
		if v == nil {
			return fmt.Errorf("no notes found")
		}
		_ = json.Unmarshal(v, &notes)

		found := false
		var newNotes []Note
		for _, n := range notes {
			if n.ID == noteID {
				found = true
				continue
			}
			newNotes = append(newNotes, n)
		}
		if !found {
			return fmt.Errorf("note not found")
		}
		if len(newNotes) == 0 {
			return bkt.Delete(key)
		}
		return putJSON(bkt, key, newNotes)
	})
}

func (d *DB) ListNotes(gid, uid string) ([]Note, error) {
	var notes []Note
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktNotes).Get([]byte(gid + ":" + uid))
		if v == nil {
			return nil
		}
		return json.Unmarshal(v, &notes)
	})
	return notes, err
}

func (d *DB) ClearNotes(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktNotes).Delete([]byte(gid + ":" + uid))
	})
}

func (d *DB) SaveLockdownIgnore(gid, cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktLockdownIgnores).Put([]byte(gid+":"+cid), []byte{1})
	})
}

func (d *DB) IsLockdownIgnore(gid, cid string) (bool, error) {
	ignored := false
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktLockdownIgnores).Get([]byte(gid + ":" + cid))
		if v != nil {
			ignored = true
		}
		return nil
	})
	return ignored, err
}

func (d *DB) DeleteLockdownIgnore(gid, cid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktLockdownIgnores).Delete([]byte(gid + ":" + cid))
	})
}

func (d *DB) ListLockdownIgnores(gid string) ([]string, error) {
	var out []string
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktLockdownIgnores)
		c := b.Cursor()
		prefix := gid + ":"
		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
			cid := strings.TrimPrefix(string(k), prefix)
			out = append(out, cid)
		}
		return nil
	})
	return out, err
}

func (d *DB) SaveRestrictedCommand(gid, cmd, roleID string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktRestrictedCmds).Put([]byte(gid+":"+strings.ToLower(cmd)), []byte(roleID))
	})
}

func (d *DB) GetRestrictedCommand(gid, cmd string) (string, error) {
	var roleID string
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktRestrictedCmds).Get([]byte(gid + ":" + strings.ToLower(cmd)))
		if v != nil {
			roleID = string(v)
		}
		return nil
	})
	return roleID, err
}

func (d *DB) DeleteRestrictedCommand(gid, cmd string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktRestrictedCmds).Delete([]byte(gid + ":" + strings.ToLower(cmd)))
	})
}

func (d *DB) ListRestrictedCommands(gid string) (map[string]string, error) {
	out := make(map[string]string)
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktRestrictedCmds)
		c := b.Cursor()
		prefix := gid + ":"
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			cmd := strings.TrimPrefix(string(k), prefix)
			out[cmd] = string(v)
		}
		return nil
	})
	return out, err
}

func (d *DB) DeleteAllRestrictedCommands(gid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktRestrictedCmds)
		c := b.Cursor()
		prefix := gid + ":"
		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
			_ = b.Delete(k)
		}
		return nil
	})
}

func (d *DB) SaveWatchedThread(gid, tid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktWatchedThreads).Put([]byte(gid+":"+tid), []byte{1})
	})
}

func (d *DB) IsWatchedThread(gid, tid string) (bool, error) {
	watched := false
	err := d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktWatchedThreads).Get([]byte(gid + ":" + tid))
		if v != nil {
			watched = true
		}
		return nil
	})
	return watched, err
}

func (d *DB) DeleteWatchedThread(gid, tid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktWatchedThreads).Delete([]byte(gid + ":" + tid))
	})
}

func (d *DB) ListWatchedThreads(gid string) ([]string, error) {
	var out []string
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktWatchedThreads)
		c := b.Cursor()
		prefix := gid + ":"
		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
			tid := strings.TrimPrefix(string(k), prefix)
			out = append(out, tid)
		}
		return nil
	})
	return out, err
}

