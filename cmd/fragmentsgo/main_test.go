package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewScaffoldsFragment(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"new", "-dir", dir, "Hello, World!"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	path := filepath.Join(dir, "hello-world.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("scaffold missing: %v", err)
	}
	text := string(body)
	for _, want := range []string{"title: Hello, World!", "date: " + time.Now().UTC().Format("2006-01-02"), "status: draft"} {
		if !strings.Contains(text, want) {
			t.Errorf("scaffold missing %q in:\n%s", want, text)
		}
	}
	if !strings.Contains(stdout.String(), path) {
		t.Errorf("stdout = %s", stdout.String())
	}

	// A second run refuses to overwrite.
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"new", "-dir", dir, "Hello, World!"}, &stdout, &stderr); code != 1 {
		t.Fatalf("overwrite code = %d", code)
	}

	// Unknown statuses are rejected up front.
	stderr.Reset()
	if code := run([]string{"new", "-dir", dir, "-status", "nonsense", "X"}, &stdout, &stderr); code != 2 {
		t.Fatalf("bad status code = %d", code)
	}
}

func TestValidateReportsProblems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"),
		[]byte("---\ntitle: Same\nslug: clash\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.md"),
		[]byte("---\ntitle: Also Same\nslug: clash\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"validate", dir}, &stdout, &stderr); code != 1 {
		t.Fatalf("duplicate slug code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), `duplicate slug "clash"`) {
		t.Errorf("stderr = %s", stderr.String())
	}

	clean := t.TempDir()
	if err := os.WriteFile(filepath.Join(clean, "only.md"),
		[]byte("---\ntitle: Only\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"validate", clean}, &stdout, &stderr); code != 0 {
		t.Fatalf("clean code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 fragment(s)") || !strings.Contains(stdout.String(), "OK") {
		t.Errorf("stdout = %s", stdout.String())
	}
}
