package watcher

import (
	"bytes"
	"fmt"
	"github.com/samber/lo"
	"os"
	"time"
)

type fileStats struct {
	FileCount int64 `yaml:"file_count" json:"file_count"`
	DirCount  int64 `yaml:"dir_count" json:"dir_count"`
	LinkCount int64 `yaml:"link_count" json:"link_count"`
	TotalSize int64 `yaml:"total_size" json:"total_size"`
}

type FileInfo struct {
	FileName    string    `yaml:"name" json:"name"`
	FilePath    string    `yaml:"path" json:"path"`
	FileSize    int64     `yaml:"size" json:"size"`
	FileMode    uint32    `yaml:"mode" json:"mode"`
	FileHashSum []byte    `yaml:"hash_sum" json:"hash_sum"`
	FileMtime   time.Time `yaml:"mtime" json:"mtime"`

	os.FileInfo `yaml:"-" json:"-"`
}

var _ os.FileInfo = (*FileInfo)(nil)

func (fi *FileInfo) Size() int64 {
	return fi.FileSize
}

func (fi *FileInfo) IsDir() bool {
	return fi.Mode().IsDir()
}

func (fi *FileInfo) Name() string {
	return fi.FileName
}

func (fi *FileInfo) Path() string {
	return fi.FilePath
}

func (fi *FileInfo) Mode() os.FileMode {
	return os.FileMode(fi.FileMode)
}

func (fi *FileInfo) ModTime() time.Time {
	return fi.FileMtime
}

func (fi *FileInfo) HashSum() []byte {
	return fi.FileHashSum
}

func (fi *FileInfo) HasSysFileInfo() bool {
	return fi.FileInfo != nil
}

type FileInfos map[string]*FileInfo

func NewFileInfos() FileInfos {
	return make(FileInfos)
}

func (fis FileInfos) Append(infoList ...FileInfos) FileInfos {
	for _, infos := range infoList {
		for path, info := range infos {
			fis[path] = info
		}
	}
	return fis
}

// Has returns true if the file info map has the given path
func (fis FileInfos) Has(path string) bool {
	_, ok := fis[formatPath(path)]
	return ok
}

// Put adds a new file info to the map
func (fis FileInfos) Put(path string, fi *FileInfo) {
	fis[formatPath(path)] = fi
}

// Get returns the file info of the given path
func (fis FileInfos) Get(path string) (*FileInfo, bool) {
	p, ok := fis[formatPath(path)]
	return p, ok
}

// Delete deletes the file info of the given path
func (fis FileInfos) Delete(path string) {
	delete(fis, formatPath(path))
}

// Len returns the length of the file info map
func (fis FileInfos) Len() int {
	return len(fis)
}

// Keys returns the keys of the file info map
func (fis FileInfos) Keys() []string {
	return lo.MapToSlice(fis, func(key string, value *FileInfo) string {
		return key
	})
}

// Values returns the values of the file info map
func (fis FileInfos) Values() []*FileInfo {
	return lo.MapToSlice(fis, func(key string, value *FileInfo) *FileInfo {
		return value
	})
}

// stats returns the file stats of the file info map
func (fis FileInfos) stats() fileStats {
	var stats fileStats
	for _, fi := range fis {
		if fi.IsDir() {
			stats.DirCount++
		} else if fi.Mode()&os.ModeSymlink != 0 {
			stats.LinkCount++
		} else {
			stats.FileCount++
			stats.TotalSize += fi.Size()
		}
	}

	return stats
}

// convertToFileInfo converts os.FileInfo to *FileInfo
func convertToFileInfo(path string, fi os.FileInfo) *FileInfo {
	return &FileInfo{
		FileInfo:  fi,
		FilePath:  path,
		FileName:  fi.Name(),
		FileSize:  fi.Size(),
		FileMode:  uint32(fi.Mode()),
		FileMtime: fi.ModTime(),
	}
}

// ByteCountIEC byte size to human-readable string
func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}

// sameFile compares two *FileInfo and returns true if they are the same file
// sameFileID: file inode/file id is the same
// sameContent: file content is the same via hash-sum
func sameFile(fi1, fi2 *FileInfo) (sameFileID bool, sameContent bool) {
	if fi1.HasSysFileInfo() && fi2.HasSysFileInfo() {
		sameFileID = os.SameFile(fi1.FileInfo, fi2.FileInfo)
	} else {
		sameFileID = false
	}

	if fi1.IsDir() == fi2.IsDir() {
		// both are files
		if !fi1.IsDir() {
			// check hash-sum if size is equal
			if fi1.Size() == fi2.Size() && fi1.HashSum() != nil && fi2.HashSum() != nil {
				sameContent = bytes.Equal(fi1.HashSum(), fi2.HashSum())
			}
		} else { // folder
			sameContent = fi1.ModTime() == fi2.ModTime() &&
				fi1.Size() == fi2.Size() &&
				fi1.Mode() == fi2.Mode() &&
				fi1.IsDir() == fi2.IsDir()
		}
	}
	return
}
