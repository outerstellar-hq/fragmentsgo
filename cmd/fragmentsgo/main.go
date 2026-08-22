// Command fragmentsgo scaffolds and validates content directories for
// fragmentsgo-based sites.
//
// Usage:
//
//	fragmentsgo new [-dir directory] [-status status] "Post Title"
//	fragmentsgo validate <directory> [directory ...]
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	fragmentsgo "github.com/outerstellar-hq/fragmentsgo"
	"github.com/outerstellar-hq/fragmentsgo/imageopt"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "new":
		return runNew(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "optimize":
		return runOptimize(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "fragmentsgo: unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  fragmentsgo new [-dir directory] [-status status] "Title"
      Scaffold a draft Markdown fragment (slug from the title).
  fragmentsgo validate <directory> [directory ...]
      Parse every fragment and report front-matter, slug, and URL problems.
  fragmentsgo optimize [-max 1600] [-quality 80] <input> [output]
      Downscale and re-encode an image (JPEG/PNG); writes in place when
      no output path is given.
`)
}

func runNew(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("new", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", ".", "directory to create the fragment in")
	status := flags.String("status", "draft", "initial status front matter")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	rest := flags.Args()
	if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
		_, _ = fmt.Fprintln(stderr, "new: exactly one title is required")
		return 2
	}
	switch fragmentsgo.Status(*status) {
	case fragmentsgo.StatusDraft, fragmentsgo.StatusReview, fragmentsgo.StatusApproved,
		fragmentsgo.StatusPublished, fragmentsgo.StatusArchived:
	default:
		_, _ = fmt.Fprintf(stderr, "new: unknown status %q\n", *status)
		return 2
	}
	title := strings.TrimSpace(rest[0])
	slug := fragmentsgo.Slugify(title)
	if slug == "" {
		_, _ = fmt.Fprintln(stderr, "new: title has no slug-able characters")
		return 2
	}
	path := filepath.Join(*dir, slug+".md")
	if _, err := os.Stat(path); err == nil {
		_, _ = fmt.Fprintf(stderr, "new: %s already exists\n", path)
		return 1
	}
	body := fmt.Sprintf("---\ntitle: %s\ndate: %s\nstatus: %s\n---\n\nWrite here.\n",
		title, time.Now().UTC().Format("2006-01-02"), *status)
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		_, _ = fmt.Fprintf(stderr, "new: %v\n", err)
		return 1
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		_, _ = fmt.Fprintf(stderr, "new: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "created %s\n", path)
	return 0
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "validate: at least one directory is required")
		return 2
	}
	problems := 0
	for _, dir := range args {
		problems += validateDir(dir, stdout, stderr)
	}
	if problems > 0 {
		_, _ = fmt.Fprintf(stderr, "validate: %d problem(s)\n", problems)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "validate: OK")
	return 0
}

func validateDir(dir string, stdout, stderr io.Writer) int {
	repository := fragmentsgo.NewFileSystemRepository(fragmentsgo.RepositoryOptions{
		Path:    dir,
		BaseURL: "/",
	})
	if err := repository.Load(); err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: load failed: %v\n", dir, err)
		return 1
	}
	fragments := repository.Everything()
	problems := 0
	slugs := map[string]string{}
	urls := map[string]string{}
	for _, fragment := range fragments {
		if previous, clash := slugs[fragment.Slug]; clash {
			_, _ = fmt.Fprintf(stderr, "%s: duplicate slug %q (also %s)\n", dir, fragment.Slug, previous)
			problems++
		} else {
			slugs[fragment.Slug] = fragment.SourcePath
		}
		if previous, clash := urls[fragment.URL]; clash {
			_, _ = fmt.Fprintf(stderr, "%s: duplicate URL %q (also %s)\n", dir, fragment.URL, previous)
			problems++
		} else {
			urls[fragment.URL] = fragment.SourcePath
		}
	}
	_, _ = fmt.Fprintf(stdout, "%s: %d fragment(s)\n", dir, len(fragments))
	return problems
}

func runOptimize(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("optimize", flag.ContinueOnError)
	flags.SetOutput(stderr)
	maxWidth := flags.Int("max", 1600, "longest-edge pixel cap")
	quality := flags.Int("quality", 80, "JPEG re-encoding quality")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	rest := flags.Args()
	if len(rest) < 1 || len(rest) > 2 {
		_, _ = fmt.Fprintln(stderr, "optimize: input path required, optional output path")
		return 2
	}
	source, err := os.ReadFile(rest[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "optimize: %v\n", err)
		return 1
	}
	result, err := imageopt.Optimize(source, imageopt.Options{
		MaxWidth: *maxWidth, JPEGQuality: *quality,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "optimize: %v\n", err)
		return 1
	}
	if result.Format == imageopt.FormatOriginal {
		_, _ = fmt.Fprintln(stdout, "passed through unchanged (unsupported format)")
		return 0
	}
	target := rest[0]
	if len(rest) == 2 {
		target = rest[1]
	}
	if err := os.WriteFile(target, result.Data, 0o644); err != nil {
		_, _ = fmt.Fprintf(stderr, "optimize: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "%s: %d -> %d bytes (%s, %dx%d)\n",
		target, len(source), len(result.Data), result.Format, result.Width, result.Height)
	return 0
}
