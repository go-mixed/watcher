package watcher

import "errors"

var (
	// ErrDurationTooShort occurs when calling the watcher's Start
	// method with a duration that's less than 1 nanosecond.
	ErrDurationTooShort = errors.New("error: duration is less than 1ns")

	// ErrWatcherRunning occurs when trying to call the watcher's
	// Start method and the polling cycle is still already running
	// from previously calling Start and not yet calling Close.
	ErrWatcherRunning = errors.New("error: watcher is already running")

	// ErrWatchedFileDeleted is an error that occurs when a file or folder that was
	// being watched has been deleted.
	ErrWatchedFileDeleted = errors.New("error: watched file or folder deleted")

	// ErrSkip is less of an error, but more of a way for path hooks to skip a file or
	// directory.
	ErrSkip = errors.New("error: skipping file")
)
