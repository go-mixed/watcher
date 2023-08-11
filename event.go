package watcher

import (
	"fmt"
)

// An Event describes an event that is received when lastFiles or directory
// changes occur. It includes the os.FileInfo of the changed file or
// directory and the type of event that's occurred and the full path of the file.
type Event struct {
	Op
	Path      string `yaml:"path"`
	OldPath   string `yaml:"old_path"`
	*FileInfo `yaml:"-"`
}

// String returns a string depending on what type of event occurred and the
// file name associated with the event.
func (e Event) String() string {
	if e.FileInfo == nil {
		return "???"
	}

	pathType := "FILE"
	if e.IsDir() {
		pathType = "DIRECTORY"
	}
	return fmt.Sprintf("%s %q %s [%s]", pathType, e.Name(), e.Op, e.Path)
}
