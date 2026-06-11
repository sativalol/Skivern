package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

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
		key := []byte(fmt.Sprintf("%d", id))
		return putJSON(gbkt, key, c)
	})
	return id, err
}

func (d *DB) GetCase(gid string, id int) (Case, error) {
	var c Case
	err := d.b.View(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktCases).Bucket([]byte(gid))
		if gbkt == nil {
			return fmt.Errorf("no cases")
		}
		v := gbkt.Get([]byte(fmt.Sprintf("%d", id)))
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
			return fmt.Errorf("no cases")
		}
		return gbkt.Delete([]byte(fmt.Sprintf("%d", id)))
	})
}

func (d *DB) DeleteAllCases(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		gbkt := tx.Bucket(bktCases).Bucket([]byte(gid))
		if gbkt == nil {
			return nil
		}
		c := gbkt.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var entry Case
			if json.Unmarshal(v, &entry) == nil && entry.UserID == uid {
				_ = gbkt.Delete(k)
			}
		}
		return nil
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
			var entry Case
			if json.Unmarshal(v, &entry) == nil && entry.UserID == uid {
				out = append(out, entry)
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
			return fmt.Errorf("no jail record")
		}
		v := gbkt.Get([]byte(uid))
		if v == nil {
			return fmt.Errorf("user not jailed")
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

func (d *DB) SaveNicklock(gid, uid, nickname string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid)
		return tx.Bucket(bktNicklocks).Put(k, []byte(nickname))
	})
}

func (d *DB) GetNicklock(gid, uid string) (string, error) {
	var nickname string
	err := d.b.View(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid)
		v := tx.Bucket(bktNicklocks).Get(k)
		if v == nil {
			return fmt.Errorf("not locked")
		}
		nickname = string(v)
		return nil
	})
	return nickname, err
}

func (d *DB) DeleteNicklock(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		k := []byte(gid + ":" + uid)
		return tx.Bucket(bktNicklocks).Delete(k)
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
	err := d.b.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktTempRoles)
		now := time.Now()
		return b.ForEach(func(_, v []byte) error {
			var tr TempRole
			if json.Unmarshal(v, &tr) == nil {
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

type StickyRoleEntry struct {
	UserID string
	RoleID string
}

func (d *DB) SaveStickyRole(gid, uid, rid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktStickyRoles).Put([]byte(gid+":"+uid+":"+rid), []byte("1"))
	})
}

func (d *DB) IsStickyRole(gid, uid, rid string) bool {
	sticky := false
	_ = d.b.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bktStickyRoles).Get([]byte(gid + ":" + uid + ":" + rid))
		if v != nil {
			sticky = true
		}
		return nil
	})
	return sticky
}

func (d *DB) DeleteStickyRole(gid, uid, rid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktStickyRoles).Delete([]byte(gid + ":" + uid + ":" + rid))
	})
}

func (d *DB) ListStickyRoles(gid string) ([]StickyRoleEntry, error) {
	var out []StickyRoleEntry
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(gid + ":")
		c := tx.Bucket(bktStickyRoles).Cursor()
		for k, _ := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, _ = c.Next() {
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
			return nil
		}
		return json.Unmarshal(v, &roles)
	})
	return roles, err
}

func (d *DB) AddBypass(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAntinuke).Put([]byte(gid+":"+uid), []byte("1"))
	})
}

func (d *DB) HasBypass(gid, uid string) bool {
	ok := false
	_ = d.b.View(func(tx *bolt.Tx) error {
		if tx.Bucket(bktAntinuke).Get([]byte(gid+":"+uid)) != nil {
			ok = true
		}
		return nil
	})
	return ok
}

func (d *DB) DeleteBypass(gid, uid string) error {
	return d.b.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bktAntinuke).Delete([]byte(gid + ":" + uid))
	})
}

func (d *DB) ListBypasses(gid string) ([]string, error) {
	var out []string
	err := d.b.View(func(tx *bolt.Tx) error {
		prefix := []byte(gid + ":")
		c := tx.Bucket(bktAntinuke).Cursor()
		for k, _ := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, _ = c.Next() {
			parts := strings.Split(string(k), ":")
			if len(parts) >= 2 {
				out = append(out, parts[1])
			}
		}
		return nil
	})
	return out, err
}
