package main

import (
	"github.com/go-mixed/watcher"
	"github.com/go-mixed/watcher/cmd/internal/conf"
	"os"
	"path/filepath"
)

func main() {
	currentPath, _ := os.Executable()
	currentDir := filepath.Dir(currentPath)
	config := conf.LoadConf(currentDir + "/conf.yaml")

	adapter := watcher.NewAdapter(config.HashAlgorithm)
	if err := adapter.LoadAll(config.Paths()...); err != nil {
		panic(err)
	}

	watch := watcher.NewWatcher(adapter)

	for _, w := range config.Watch {
		options := watcher.WatchOption{
			Recursive:    w.Recursive,
			IgnoreHidden: w.IgnoreHidden,
			Ignore:       w.GitIgnore(),
			Op:           w.Op(),
		}

		for _, path := range w.Paths {
			if err := watch.Add(path, options); err != nil {
				panic(err)
			}
		}
	}

	watch.Watch()

}
