// Package reload provides a dependency-free content watcher for
// development servers: it polls the watched directories and invokes a
// callback once changes settle. Polling rather than fs events keeps the
// library stdlib-only and behaves identically on every platform.
package reload

import (
	"context"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Options tune the watcher.
type Options struct {
	// Interval is the poll period; zero means 2 seconds.
	Interval time.Duration
	// Debounce is the quiet period after the last change before the
	// callback fires, so editor save bursts trigger one reload; zero
	// means 500 milliseconds.
	Debounce time.Duration
}

// Watch polls the Markdown files under dirs (recursively) until ctx is
// done, calling onChange on the watcher's goroutine each time the file
// set settles after changes. It returns immediately.
func Watch(ctx context.Context, dirs []string, options Options, onChange func()) {
	if options.Interval <= 0 {
		options.Interval = 2 * time.Second
	}
	if options.Debounce <= 0 {
		options.Debounce = 500 * time.Millisecond
	}
	snapshot := scan(dirs)
	go func() {
		ticker := time.NewTicker(options.Interval)
		defer ticker.Stop()
		var lastChange time.Time
		dirty := false
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				current := scan(dirs)
				if !same(snapshot, current) {
					snapshot = current
					lastChange = now
					dirty = true
					continue
				}
				if dirty && now.Sub(lastChange) >= options.Debounce {
					dirty = false
					onChange()
				}
			}
		}
	}()
}

// scan builds the change signature of every Markdown file under dirs:
// path → modification time and size.
func scan(dirs []string) map[string]string {
	signatures := map[string]string{}
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			signatures[path] = info.ModTime().Format(time.RFC3339Nano) + "|" +
				strconv.FormatInt(info.Size(), 10)
			return nil
		})
	}
	return signatures
}

func same(before, after map[string]string) bool {
	if len(before) != len(after) {
		return false
	}
	for path, signature := range before {
		if after[path] != signature {
			return false
		}
	}
	return true
}
