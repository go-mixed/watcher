package watcher

import (
	"fmt"
	"github.com/bytedance/sonic"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/multierr"
	"os"
	"path/filepath"
	"time"
)

func (s *Adapter) openHashingDB() (*bolt.DB, error) {
	p, _ := os.Executable()
	dir := filepath.Join(filepath.Dir(p), "data")
	_ = os.MkdirAll(dir, 0)
	return s.openDB(filepath.Join(dir, hashingDbFile))
}

// openDB
func (s *Adapter) openDB(path string) (*bolt.DB, error) {
	return bolt.Open(path, 0665, &bolt.Options{Timeout: 5 * time.Second})
}

func (s *Adapter) readFileInfos(db *bolt.DB, bucketName []byte, keys []string) FileInfos {
	var infos FileInfos = NewFileInfos()
	if db == nil {
		return infos
	}
	// load file list
	_ = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return nil
		}

		for _, key := range keys {
			var info *FileInfo
			_ = sonic.Unmarshal(bucket.Get(s.pathKey(key)), &info)
			if info != nil {
				infos.Put(key, info)
			}

		}
		return nil
	})

	return infos
}

func (s *Adapter) readAllFileInfos(db *bolt.DB, bucketName []byte) FileInfos {
	var infos FileInfos = NewFileInfos()
	if db == nil {
		return infos
	}

	// load file list
	_ = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return nil
		}

		_ = bucket.ForEach(func(k, v []byte) error {
			var info *FileInfo
			_ = sonic.Unmarshal(v, &info)
			if info != nil {
				infos.Put(string(k), info)
			}
			return nil
		})

		return nil
	})

	return infos
}

func (s *Adapter) putFileInfos(db *bolt.DB, bucketName []byte, infos FileInfos, deleteHistory bool) error {
	if db == nil {
		return nil
	}

	if deleteHistory {
		// get old keys
		_ = db.Update(func(tx *bolt.Tx) error {
			return tx.DeleteBucket(bucketName)
		})
	}

	var n int64
	// split infos into chunks
	var chunks []FileInfos
	var tmpInfos FileInfos = NewFileInfos()
	for k, v := range infos {
		n++
		tmpInfos[k] = v
		if n%1000 == 0 {
			chunks = append(chunks, tmpInfos)
			tmpInfos = NewFileInfos()
		}
	}
	chunks = append(chunks, tmpInfos)

	var err error
	n = 0
	for _, chunk := range chunks {
		err = multierr.Append(err,
			db.Batch(func(tx *bolt.Tx) error {
				bucket, err := tx.CreateBucketIfNotExists(bucketName)
				if err != nil {
					return err
				}

				// save new keys
				for path, info := range chunk {
					if info == nil {
						continue
					}

					j, _ := sonic.Marshal(info)
					if len(j) == 0 { // json is empty or error
						continue
					}

					if err = bucket.Put(s.pathKey(path), j); err != nil {
						return err
					}
					n++
				}
				return nil
			}),
		)
		fmt.Printf("\rSaved %d/%d file informations to \"%s\"", n, len(infos), db.Path())
	}
	fmt.Println()
	return err
}

func (s *Adapter) putSetting(db *bolt.DB, keyName []byte, setting *adapterSetting) error {
	if db == nil {
		return nil
	}
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("setting"))
		if err != nil {
			return err
		}

		j, _ := sonic.Marshal(setting)
		if len(j) == 0 { // json is empty or error
			return nil
		}

		return bucket.Put(keyName, j)
	})
}

func (s *Adapter) readSetting(db *bolt.DB, keyName []byte) *adapterSetting {
	if db == nil {
		return nil
	}
	var setting *adapterSetting
	_ = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("setting"))
		if bucket == nil {
			return nil
		}

		_ = sonic.Unmarshal(bucket.Get(keyName), &setting)
		return nil
	})

	return setting
}
