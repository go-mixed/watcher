package watcher

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"github.com/bytedance/sonic"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/multierr"
	"hash"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type Adapter struct {
	hashAlgorithm string
	hash          hash.Hash
	hashingDB     *bolt.DB

	fileList map[string]FileInfos
	settings map[string]*adapterSetting
}

type adapterSetting struct {
	RootPath      string    `yaml:"root_path" json:"root_path"`
	At            time.Time `yaml:"at" json:"at"`
	HashAlgorithm string    `yaml:"hash_algorithm" json:"hash_algorithm"`
	Stats         fileStats `yaml:"stats" json:"stats"`
}

const DBFile = ".watch.db"
const hashingDbFile = "hashing.db"

func NewAdapter(hashAlgorithm string) *Adapter {
	_ = sonic.Pretouch(reflect.TypeOf(&FileInfo{}))

	var h hash.Hash

	switch strings.ToLower(hashAlgorithm) {
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	case "crc32":
		h = crc32.NewIEEE()
	default:
		h = md5.New()
	}

	return &Adapter{
		hash:          h,
		hashAlgorithm: hashAlgorithm,
		fileList:      make(map[string]FileInfos),
		settings:      make(map[string]*adapterSetting),
	}
}

// get the db path of root path
func (s *Adapter) getDbPath(rootPath string) string {
	return filepath.Join(rootPath, DBFile)
}

func (s *Adapter) LoadAll(rootPaths ...string) error {
	var err error

	for _, rootPath := range rootPaths {
		err = multierr.Append(err, s.load(rootPath))
	}
	return err
}

// load the history file list
func (s *Adapter) load(rootPath string) error {
	db, err := s.openDB(s.getDbPath(rootPath))
	if err != nil {
		return err
	}
	defer db.Close()

	s.settings[formatPath(rootPath)] = s.readSetting(db, s.pathKey(rootPath))
	s.fileList[formatPath(rootPath)] = s.readAllFileInfos(db, s.pathKey(rootPath))

	log.Printf("Loaded file list from db: %s", s.getDbPath(rootPath))

	return nil
}

func (s *Adapter) pathKey(path string) []byte {
	return []byte(formatPath(path))
}

func (s *Adapter) Compare(rootPath string, currentFiles FileInfos) (
	created,
	updated,
	deleted,
	moved,
	renamed FileInfos,
) {

	// compare the current file list with the old file list, and stats the created, updated, deleted files
	// **AND** sets the old hash sum to currentFiles for not changed files
	created, updated, deleted = s.compareCUD(rootPath, currentFiles)

	// hashing the created, updated
	s.hashing(rootPath, NewFileInfos().Append(created, updated))

	// stats moved or renamed files from deleted && created files
	// it'll remove the moved or renamed files from deleted && created files
	moved, renamed = s.compareMv(deleted, created)

	return
}

func (s *Adapter) hashing(rootPath string, fileInfos FileInfos) {
	var currentSize int64

	stats := fileInfos.stats()

	if stats.FileCount == 0 {
		return
	}

	db, err := s.openHashingDB()
	if err != nil {
		log.Printf("\n[ERROR] open hashing db error: %s\n", err)
	} else {
		defer db.Close()
	}

	historyFileInfos := s.readFileInfos(db, s.pathKey(rootPath), fileInfos.Keys())
	hashingFileInfos := NewFileInfos()

	var putToHashingDB = func(path string, info *FileInfo) {
		if path != "" {
			hashingFileInfos.Put(path, info)
		}

		if path == "" || hashingFileInfos.Len() >= 100 {
			_ = s.putFileInfos(db, s.pathKey(rootPath), hashingFileInfos, false)
			hashingFileInfos = NewFileInfos()
		}
	}

	// hash sum for created and updated files
	for _, currentFile := range fileInfos {
		if !currentFile.IsDir() {
			path := currentFile.Path()

			// read hash-sum from history file if exists
			if historyFileInfo, ok := historyFileInfos.Get(path); ok {
				if historyFileInfo.Mode() == currentFile.Mode() &&
					historyFileInfo.ModTime() == currentFile.ModTime() &&
					historyFileInfo.FileSize == currentFile.FileSize {
					currentFile.FileHashSum = historyFileInfo.FileHashSum
				}
			}

			// hash-sum
			if len(currentFile.FileHashSum) == 0 {
				currentFile.FileHashSum, err = s.hashSum(path)
				if err != nil {
					log.Printf("\n[ERROR] hashing file error: %s\n", err)
				}

				putToHashingDB(path, currentFile)
			}

			currentSize += currentFile.FileSize

			fmt.Printf("\rHashing \"%s\": progress: %0.2f%% files: %s/%s", rootPath, float64(currentSize)/float64(stats.TotalSize)*100, ByteCountIEC(currentSize), ByteCountIEC(stats.TotalSize))

		}
	}

	putToHashingDB("", nil)

	fmt.Println()
}

// compareCUD compares the current file list with the old file list, and stats the created, updated, deleted files
// **AND** sets the old hash sum to currentFiles for not changed files
func (s *Adapter) compareCUD(rootPath string, currentFileInfos FileInfos) (created, updated, deleted FileInfos) {
	var oldFileInfos FileInfos
	var ok bool
	var path string
	if oldFileInfos, ok = s.fileList[formatPath(rootPath)]; !ok || oldFileInfos == nil {
		oldFileInfos = NewFileInfos()
	}

	created = make(FileInfos)
	updated = make(FileInfos)
	deleted = make(FileInfos)

	// stats created, updated files
	var currentFile *FileInfo
	var oldFile *FileInfo
	for path, currentFile = range currentFileInfos {
		oldFile, ok = oldFileInfos[path]
		if !ok {
			created.Put(path, currentFile)
			continue
		}

		if !currentFile.FileMtime.Equal(oldFile.FileMtime) || currentFile.FileSize != oldFile.FileSize {
			updated.Put(path, currentFile)
			continue
		}

		// if the file is not changed, use the old hash sum
		currentFile.FileHashSum = oldFile.FileHashSum
	}

	// stats deleted files
	for path, oldFile = range oldFileInfos {
		if _, ok = currentFileInfos[path]; !ok {
			deleted.Put(path, oldFile)
		}
	}

	return
}

// compareMv stats moved or renamed files from deleted && created files
// it'll remove the moved or renamed files from deleted && created files
func (s *Adapter) compareMv(deleted, created FileInfos) (moved, renamed FileInfos) {
	moved = make(FileInfos)
	renamed = make(FileInfos)

	// stats moved or renamed files from deleted && created files
	// 1. deleteFile must be in the history file list
	for deletedPath, deletedFile := range deleted {
		for createdPath, createdFile := range created {
			if sameFileID, sameContent := sameFile(deletedFile, createdFile); sameFileID || sameContent {
				if filepath.Dir(deletedPath) == filepath.Dir(createdFile.Path()) {
					renamed.Put(deletedPath, createdFile)
				} else {
					moved.Put(deletedPath, createdFile)
				}

				deleted.Delete(deletedPath)
				created.Delete(createdPath)
			}
		}
	}
	return
}

func (s *Adapter) hashSum(path string) ([]byte, error) {
	s.hash.Reset()
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if _, err = io.Copy(s.hash, file); err != nil {
		return nil, err
	}

	return s.hash.Sum(nil), nil
}

func (s *Adapter) SaveAll() {
	for rootPath, currentFiles := range s.fileList {
		s.Save(rootPath, currentFiles)
	}
}

func (s *Adapter) Save(rootPath string, fileInfos FileInfos) {
	dbPath := s.getDbPath(rootPath)
	db, err := s.openDB(dbPath)
	if err != nil {
		log.Printf("[ERROR] opening database \"%s\" error: %s\n", dbPath, err)
	}
	defer db.Close()

	setting := &adapterSetting{
		RootPath:      rootPath,
		At:            time.Now(),
		HashAlgorithm: s.hashAlgorithm,
		Stats:         fileInfos.stats(),
	}

	s.settings[formatPath(rootPath)] = setting
	s.fileList[formatPath(rootPath)] = fileInfos

	if err = s.putSetting(db, s.pathKey(rootPath), setting); err != nil {
		log.Printf("[ERROR] saving setting to \"%s\" error: %s\n", dbPath, err)
	}

	if err = s.putFileInfos(db, s.pathKey(rootPath), fileInfos, true); err != nil {
		log.Printf("[ERROR] saving file informations to \"%s\" error: %s\n", dbPath, err)
	}

	log.Printf("Saved file informations to \"%s\"", dbPath)
}
