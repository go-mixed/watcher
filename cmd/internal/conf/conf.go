package conf

import (
	"github.com/go-mixed/watcher"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
)

type Conf struct {
	HashAlgorithm string      `yaml:"hash_algorithm"`
	Watch         []WatchConf `yaml:"watch"`
}

func (c *Conf) Paths() []string {
	var paths []string
	for _, watch := range c.Watch {
		paths = append(paths, watch.Paths...)
	}
	return paths
}

type WatchConf struct {
	Paths        []string `yaml:"paths"`
	Recursive    bool     `yaml:"recursive"`
	IgnoreHidden bool     `yaml:"ignoreHidden"`
	Ignore       []string `yaml:"ignore"`
	Actions      []string `yaml:"actions"`
}

func (w *WatchConf) Op() watcher.Op {
	var op watcher.Op
	for _, action := range w.Actions {
		for opV, opStr := range watcher.Ops {
			if strings.EqualFold(opStr, action) {
				op |= opV
			}
		}
	}

	return op
}

func (w *WatchConf) GitIgnore() *watcher.GitIgnore {
	return watcher.CompileIgnoreLines(append([]string{watcher.DBFile}, w.Ignore...)...)
}

func LoadConf(paths ...string) *Conf {
	var conf *Conf = &Conf{}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		if err = yaml.Unmarshal(content, conf); err != nil {
			panic(err)
		}
	}

	if conf.HashAlgorithm == "" {
		conf.HashAlgorithm = "md5"
	}

	return conf
}
