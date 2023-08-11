//go build !windows
//go:build !windows
// +build !windows

package watcher

import (
	"path/filepath"
	"strings"
)

func formatPath(path string) string {
	return path
}

func isHiddenFile(path string) (bool, error) {
	return strings.HasPrefix(filepath.Base(path), "."), nil
}

func setHidden(path string) error {
	return nil
}
