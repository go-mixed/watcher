package watcher

import (
	"errors"
	"log"
	"os"
	"path/filepath"
)

type Watcher struct {
	adapter *Adapter

	optionGroup map[string]WatchOption
	fileGroup   map[string]FileInfos
}

func NewWatcher(db *Adapter) *Watcher {
	return &Watcher{
		adapter:     db,
		optionGroup: make(map[string]WatchOption),
		fileGroup:   make(map[string]FileInfos),
	}
}

// Add the path to the watch list
func (w *Watcher) Add(path string, options WatchOption) error {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("path does not exist: " + path)
		}
		return err
	}

	w.optionGroup[path] = options
	w.fileGroup[path] = nil

	log.Printf("Add path: %s", path)

	return nil
}

// Remove the path from the watch list
func (w *Watcher) Remove(path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	delete(w.optionGroup, absPath)
	delete(w.fileGroup, absPath)
}

func (w *Watcher) Watch() error {
	for path, option := range w.optionGroup {
		log.Printf("mapping files of \"%s\"...", path)
		infos, err := w.listFileInfos(path, option)
		if err != nil {
			return err
		}
		w.fileGroup[path] = infos

		stats := infos.stats()
		log.Printf("mapping files of \"%s\" done, "+
			"total size: %s, "+
			"files: %d, "+
			"directories: %d, "+
			"symlinks: %d",
			path,
			ByteCountIEC(stats.TotalSize),
			stats.FileCount,
			stats.DirCount,
			stats.LinkCount)
	}

	var created, updated, deleted, moved, renamed FileInfos
	for rootPath, infos := range w.fileGroup {
		log.Printf("comparing: %s", rootPath)
		created, updated, deleted, moved, renamed = w.adapter.Compare(rootPath, infos)
		log.Printf("created: %d, updated: %d, deleted: %d, moved: %d, renamed: %d of \"%s\"", len(created), len(updated), len(deleted), len(moved), len(renamed), rootPath)

		// save the current file list to db if there are created, updated, deleted files
		if len(deleted) > 0 || len(created) > 0 || len(updated) > 0 || len(moved) > 0 || len(renamed) > 0 {
			w.adapter.Save(rootPath, infos)
		}
	}

	return nil
}

func (w *Watcher) listFileInfos(rootPath string, option WatchOption) (FileInfos, error) {
	var fileInfos = NewFileInfos()

	var addFile = func(path string, info os.FileInfo) error {
		isHidden, err := isHiddenFile(rootPath)
		if err != nil {
			return err
		}

		var ignored bool
		if relPath, _ := filepath.Rel(rootPath, path); relPath != "" {
			ignored = option.Ignore.MatchesPath(relPath)
		}

		// Ignore hidden files and directories if the option is set
		// or filter by the ignore pattern
		// return filepath.SkipDir
		if ignored || (option.IgnoreHidden && isHidden) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		fileInfos.Put(path, convertToFileInfo(path, info))

		return nil
	}

	if option.Recursive {
		if err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return addFile(path, info)
		}); err != nil {
			return nil, err
		}
	} else {
		dirs, err := os.ReadDir(rootPath)
		if err != nil {
			return nil, err
		}
		for _, dir := range dirs {
			info, err := dir.Info()
			if err != nil {
				return nil, err
			}
			if err = addFile(filepath.Join(rootPath, dir.Name()), info); err != nil {
				if errors.Is(err, filepath.SkipDir) {
					continue
				}
				return nil, err
			}
		}
	}

	// Remove the root path
	fileInfos.Delete(rootPath)
	return fileInfos, nil
}
