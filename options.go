package watcher

// An Op is a type that is used to describe what type
// of event has occurred during the watching process.
type Op uint32

// Ops
const (
	Create Op = 1
	Write  Op = 2
	Remove Op = 4
	Rename Op = 8
	Chmod  Op = 16
	Move   Op = 32

	All Op = Create | Write | Remove | Rename | Chmod | Move
)

var Ops = map[Op]string{
	Create: "CREATE",
	Write:  "WRITE",
	Remove: "REMOVE",
	Rename: "RENAME",
	Chmod:  "CHMOD",
	Move:   "MOVE",
	All:    "ALL",
}

// String prints the string version of the Op consts
func (e Op) String() string {
	if op, found := Ops[e]; found {
		return op
	}
	return "???"
}

type WatchOption struct {
	Recursive    bool
	IgnoreHidden bool
	Ignore       *GitIgnore
	Op           Op
}
