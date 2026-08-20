package reload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func TestWatchFiresOnChangeAndDebouncesBursts(t *testing.T) {
	dir := t.TempDir()
	changes := make(chan int, 8)
	count := 0

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	Watch(ctx, []string{dir}, Options{Interval: 20 * time.Millisecond, Debounce: 60 * time.Millisecond},
		func() { count++; changes <- count })

	// Quiet: no callback without changes.
	if waitFor(t, 300*time.Millisecond, func() bool { return count > 0 }) {
		t.Fatal("callback fired without changes")
	}

	// A burst of writes collapses into one callback.
	for i := 0; i < 4; i++ {
		if err := os.WriteFile(filepath.Join(dir, "post.md"),
			[]byte("---\ntitle: Post\n---\nbody version "+time.Now().String()), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(15 * time.Millisecond)
	}
	select {
	case <-changes:
	case <-time.After(2 * time.Second):
		t.Fatal("no callback after changes")
	}
	if !waitFor(t, 2*time.Second, func() bool { return count >= 1 }) {
		t.Fatal("callback state not visible")
	}

	// The callback fires once per settled burst, not per write.
	time.Sleep(300 * time.Millisecond)
	if count > 2 {
		t.Fatalf("burst produced %d callbacks, want at most 2", count)
	}

	// Creating a nested file triggers as well.
	nested := filepath.Join(dir, "sub")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "added.md"),
		[]byte("---\ntitle: Added\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changes:
	case <-time.After(2 * time.Second):
		t.Fatal("no callback after adding a nested file")
	}

	// Non-Markdown files are ignored.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	// Cancellation stops the watcher.
	cancel()
	baseline := count
	if err := os.WriteFile(filepath.Join(dir, "post.md"), []byte("---\ntitle: Post\n---\nfinal"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if count != baseline {
		t.Fatal("callback fired after cancellation")
	}
}
