// watcher.go implements a stdlib-only file watcher using os.Stat polling.
// It walks the directory tree, records modification times, and detects
// changes on a configurable poll interval. A debounce mechanism ensures
// rapid successive changes are coalesced into a single rebuild trigger.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// watcher watches a directory tree for .go file changes using os.Stat polling.
type watcher struct {
	root            string
	pollInterval    time.Duration
	debounce        time.Duration
	modTimes        map[string]time.Time
	warnedFileLimit bool
}

// newWatcher creates a watcher for the given root directory.
func newWatcher(root string) *watcher {
	return &watcher{
		root:         root,
		pollInterval: 500 * time.Millisecond,
		debounce:     100 * time.Millisecond,
		modTimes:     make(map[string]time.Time),
	}
}

// skipDir returns true if the directory should be excluded from watching.
func skipDir(name string) bool {
	switch name {
	case ".git", "vendor", "node_modules":
		return true
	}
	// Skip hidden directories (starting with '.')
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}
	return false
}

// isWatchedFile returns true if the file should be watched for changes.
func isWatchedFile(name string) bool {
	if !strings.HasSuffix(name, ".go") {
		return false
	}
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	return true
}

// scan walks the directory tree and returns the current mod times for all watched files.
func (w *watcher) scan() (map[string]time.Time, error) {
	times := make(map[string]time.Time)
	fileCount := 0
	err := filepath.Walk(w.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			if skipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isWatchedFile(info.Name()) {
			fileCount++
			if fileCount > 10000 {
				if !w.warnedFileLimit {
					log.Printf("Warning: File count exceeds 10,000 - continuing but performance may degrade")
					w.warnedFileLimit = true
				}
			}
			times[path] = info.ModTime()
		}
		return nil
	})
	return times, err
}

// init performs the initial scan and records all file mod times.
func (w *watcher) init() error {
	times, err := w.scan()
	if err != nil {
		return err
	}
	w.modTimes = times
	return nil
}

// watch polls for file changes and sends changed file paths to the returned channel.
// It applies debouncing: after detecting a change, it waits for the debounce duration
// with no further changes before emitting. The channel is closed when done is closed.
func (w *watcher) watch(done <-chan struct{}) <-chan []string {
	ch := make(chan []string)

	go func() {
		defer close(ch)
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				changed := w.detectChanges()
				if len(changed) == 0 {
					continue
				}

				// Debounce: keep scanning until no new changes for the debounce period.
				timer := time.NewTimer(w.debounce)
				debouncing := true
				for debouncing {
					select {
					case <-done:
						timer.Stop()
						return
					case <-timer.C:
						debouncing = false
					case <-time.After(w.pollInterval):
						if more := w.detectChanges(); len(more) > 0 {
							for _, f := range more {
								changed = appendUnique(changed, f)
							}
							timer.Reset(w.debounce)
						}
					}
				}
				timer.Stop()

				select {
				case ch <- changed:
				case <-done:
					return
				}
			}
		}
	}()

	return ch
}

// detectChanges compares current file state against recorded mod times.
// Returns paths of changed, added, or deleted files and updates internal state.
func (w *watcher) detectChanges() []string {
	current, err := w.scan()
	if err != nil {
		return nil
	}

	var changed []string

	for path, modTime := range current {
		prev, exists := w.modTimes[path]
		if !exists || !modTime.Equal(prev) {
			changed = append(changed, path)
		}
	}

	for path := range w.modTimes {
		if _, exists := current[path]; !exists {
			changed = append(changed, path)
		}
	}

	w.modTimes = current
	return changed
}

// appendUnique appends s to the slice only if it's not already present.
func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}
