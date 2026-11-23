package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	bolt "go.etcd.io/bbolt"
)

// dbFileName 是索引数据库在用户数据目录下的文件名。
const dbFileName = "launcher.db"

// bucket 名称；entries / usage 都以条目路径为 key。
var (
	bucketEntries = []byte("entries")
	bucketUsage   = []byte("usage")
)

// store 用 bbolt 持久化索引条目与使用记录。nil 值表示数据库不可用，
// 所有方法都可以安全地在 nil 上调用，只是不做任何事。
type store struct {
	db *bolt.DB
}

// dbPath 返回数据库文件的绝对路径。
func dbPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, dbFileName), nil
}

// openStore 打开用户数据目录下的数据库，必要时创建目录。
func openStore() (*store, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return openStoreAt(path)
}

// openStoreAt 打开指定路径的数据库；测试直接使用临时目录。
func openStoreAt(path string) (*store, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	return &store{db: db}, nil
}

// Close 关闭数据库。
func (s *store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Entries 读回索引条目；损坏的单条记录会被跳过。
func (s *store) Entries() []Entry {
	if s == nil || s.db == nil {
		return nil
	}
	var entries []Entry
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketEntries)
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			var e Entry
			if json.Unmarshal(v, &e) == nil && e.Path != "" {
				entries = append(entries, e)
			}
			return nil
		})
	})
	if err != nil {
		return nil
	}
	return entries
}

// SaveIndex 在一个事务里替换全部索引条目。
func (s *store) SaveIndex(entries []Entry) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketEntries)
		if err != nil {
			return err
		}
		if err := clearBucket(b); err != nil {
			return err
		}
		for _, e := range entries {
			if e.Path == "" {
				continue
			}
			data, err := json.Marshal(e)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(e.Path), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// Usage 读回使用记录；空库返回 nil。
func (s *store) Usage() map[string]Usage {
	if s == nil || s.db == nil {
		return nil
	}
	usage := make(map[string]Usage)
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUsage)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var u Usage
			if json.Unmarshal(v, &u) == nil {
				usage[string(k)] = u
			}
			return nil
		})
	})
	if err != nil || len(usage) == 0 {
		return nil
	}
	return usage
}

// SaveUsage 在一个事务里替换全部使用记录。
func (s *store) SaveUsage(usage map[string]Usage) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketUsage)
		if err != nil {
			return err
		}
		if err := clearBucket(b); err != nil {
			return err
		}
		for path, u := range usage {
			data, err := json.Marshal(u)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(path), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// clearBucket 删除桶里的全部键。先收集再删除，避免在 ForEach 里改动桶。
func clearBucket(b *bolt.Bucket) error {
	var keys [][]byte
	if err := b.ForEach(func(k, _ []byte) error {
		keys = append(keys, append([]byte(nil), k...))
		return nil
	}); err != nil {
		return err
	}
	for _, k := range keys {
		if err := b.Delete(k); err != nil {
			return err
		}
	}
	return nil
}
