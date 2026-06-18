package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFind_InStartDir(t *testing.T) {
	root := t.TempDir()
	if err := Save(ConfigPath(root), Config{Owner: "acme"}); err != nil {
		t.Fatal(err)
	}

	ws, err := Find(root)
	if err != nil {
		t.Fatalf("Find() error: %v", err)
	}
	if ws.Root != root {
		t.Fatalf("Root = %q, want %q", ws.Root, root)
	}
	if ws.Path != ConfigPath(root) {
		t.Fatalf("Path = %q, want %q", ws.Path, ConfigPath(root))
	}
	if ws.Config.Owner != "acme" {
		t.Fatalf("Owner = %q, want acme", ws.Config.Owner)
	}
}

func TestFind_WalksUpToAncestor(t *testing.T) {
	root := t.TempDir()
	if err := Save(ConfigPath(root), Config{Owner: "acme"}); err != nil {
		t.Fatal(err)
	}
	// A repo + worktree nested several levels below the workspace root.
	deep := filepath.Join(root, "worktrees", "web", "feature-x")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	ws, err := Find(deep)
	if err != nil {
		t.Fatalf("Find() from a nested dir should walk up: %v", err)
	}
	if ws.Root != root {
		t.Fatalf("Root = %q, want %q (the workspace root)", ws.Root, root)
	}
}

func TestFind_NotAWorkspace(t *testing.T) {
	_, err := Find(t.TempDir())
	if !errors.Is(err, ErrNotWorkspace) {
		t.Fatalf("Find() in a plain dir should return ErrNotWorkspace, got: %v", err)
	}
}
