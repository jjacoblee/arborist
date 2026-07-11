package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/exectest"
)

// mkWorktree creates a worktree directory under a workspace's worktree root,
// marked with a ".git" file so the scanner treats it as a worktree root.
func mkWorktree(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestList_PrintsWorktrees(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	wtPath := filepath.Join(workspaceWorktreeRoot(dir), "web", "feature-x")
	mkWorktree(t, wtPath)
	baseRepo := filepath.Join(dir, "web")

	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", wtPath, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(baseRepo + "/.git\n")},
			exectest.Key("git", "-C", wtPath, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", wtPath, "status", "--porcelain"):                                   {}, // clean
		},
	}
	out, err := runRootWithRunner(t, fake, "list", "--dir", dir)
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	for _, want := range []string{"ID", "REPOSITORY", "acme/web", "feature/x", "web/feature-x"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	// Path is shown relative to the worktree root, not as an absolute path.
	if strings.Contains(out, workspaceWorktreeRoot(dir)) {
		t.Fatalf("expected a relative path, but output has the absolute root:\n%s", out)
	}
	// Header plus exactly one worktree row.
	if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) != 2 {
		t.Fatalf("expected header + one row, got %d lines:\n%s", len(lines), out)
	}
}

func TestList_FullPaths(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	wtPath := filepath.Join(workspaceWorktreeRoot(dir), "web", "feature-x")
	mkWorktree(t, wtPath)
	baseRepo := filepath.Join(dir, "web")

	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", wtPath, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(baseRepo + "/.git\n")},
			exectest.Key("git", "-C", wtPath, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", wtPath, "status", "--porcelain"):                                   {},
		},
	}
	out, err := runRootWithRunner(t, fake, "list", "--dir", dir, "--full")
	if err != nil {
		t.Fatalf("list --full: %v\n%s", err, out)
	}
	if !strings.Contains(out, wtPath) {
		t.Fatalf("--full should show the absolute path %q:\n%s", wtPath, out)
	}
}

// TestList_ContinuesPastBrokenWorktree: a worktree whose `git status` fails
// (for example corrupt objects) must not abort "arb list". It is shown as
// broken, its error is surfaced as a warning, and the healthy rows still print.
func TestList_ContinuesPastBrokenWorktree(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	broken := filepath.Join(workspaceWorktreeRoot(dir), "web", "broken-x")
	healthy := filepath.Join(workspaceWorktreeRoot(dir), "web", "feature-x")
	mkWorktree(t, broken)
	mkWorktree(t, healthy)
	baseRepo := filepath.Join(dir, "web")

	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", broken, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(baseRepo + "/.git\n")},
			exectest.Key("git", "-C", broken, "branch", "--show-current"):                                {Out: []byte("broken/branch\n")},
			exectest.Key("git", "-C", broken, "status", "--porcelain"): {
				Err: errors.New("exit status 128: fatal: unable to read 0c71cf32"),
			},
			exectest.Key("git", "-C", healthy, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(baseRepo + "/.git\n")},
			exectest.Key("git", "-C", healthy, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", healthy, "status", "--porcelain"):                                   {}, // clean
		},
	}
	out, err := runRootWithRunner(t, fake, "list", "--dir", dir)
	if err != nil {
		t.Fatalf("list should succeed despite a broken worktree: %v\n%s", err, out)
	}
	for _, want := range []string{
		"feature/x", "clean", // the healthy row is intact
		"broken/branch", "broken", // the broken row is still listed
		"warning:", "unable to read 0c71cf32", // and its error is surfaced
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

// listJSONEntry mirrors the `arb list --json` entry shape for assertions.
type listJSONEntry struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Status     string `json:"status"`
	Path       string `json:"path"`
	Error      string `json:"error"`
}

func TestList_JSON(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	broken := filepath.Join(workspaceWorktreeRoot(dir), "web", "broken-x")
	healthy := filepath.Join(workspaceWorktreeRoot(dir), "web", "feature-x")
	mkWorktree(t, broken)
	mkWorktree(t, healthy)
	baseRepo := filepath.Join(dir, "web")

	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", broken, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(baseRepo + "/.git\n")},
			exectest.Key("git", "-C", broken, "branch", "--show-current"):                                {Out: []byte("broken/branch\n")},
			exectest.Key("git", "-C", broken, "status", "--porcelain"): {
				Err: errors.New("exit status 128: fatal: unable to read 0c71cf32"),
			},
			exectest.Key("git", "-C", healthy, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(baseRepo + "/.git\n")},
			exectest.Key("git", "-C", healthy, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", healthy, "status", "--porcelain"):                                   {}, // clean
		},
	}
	out, err := runRootWithRunner(t, fake, "list", "--dir", dir, "--json")
	if err != nil {
		t.Fatalf("list --json: %v\n%s", err, out)
	}

	// stdout must be exactly one JSON array — no table, no warnings.
	var entries []listJSONEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2:\n%s", len(entries), out)
	}

	byPath := map[string]listJSONEntry{}
	for _, e := range entries {
		byPath[e.Path] = e
	}
	h, ok := byPath[healthy]
	if !ok {
		t.Fatalf("missing entry for %q:\n%s", healthy, out)
	}
	if h.ID == "" || h.Repository != "acme/web" || h.Branch != "feature/x" || h.Status != "clean" || h.Error != "" {
		t.Fatalf("healthy entry = %+v", h)
	}
	b, ok := byPath[broken]
	if !ok {
		t.Fatalf("missing entry for %q:\n%s", broken, out)
	}
	// The inspection error rides inside the entry instead of a stderr warning.
	if b.Status != "broken" || !strings.Contains(b.Error, "unable to read 0c71cf32") {
		t.Fatalf("broken entry = %+v", b)
	}
}

func TestList_JSON_EmptyIsAnArray(t *testing.T) {
	dir := writeWorkspace(t, "acme") // no worktrees created
	fake := &exectest.Fake{}

	out, err := runRootWithRunner(t, fake, "list", "--dir", dir, "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty list should be [], got:\n%s", out)
	}
}

func TestList_Empty(t *testing.T) {
	dir := writeWorkspace(t, "acme") // no worktrees created
	fake := &exectest.Fake{}
	out, err := runRootWithRunner(t, fake, "list", "--dir", dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "No worktrees found.") {
		t.Fatalf("expected empty message, got:\n%s", out)
	}
}
