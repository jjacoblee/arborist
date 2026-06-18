package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/config"
)

func TestInit_CreatesWorkspace(t *testing.T) {
	dir := t.TempDir()

	out, err := runRoot(t, "dev", "init", "--dir", dir, "--owner", "acme")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out, dir) {
		t.Fatalf("output %q should mention the workspace dir %q", out, dir)
	}

	cfg, err := config.Load(config.ConfigPath(dir))
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	if cfg.Owner != "acme" {
		t.Fatalf("written owner = %q, want acme", cfg.Owner)
	}
}

func TestInit_RequiresOwner(t *testing.T) {
	dir := t.TempDir()

	_, err := runRoot(t, "dev", "init", "--dir", dir)
	if err == nil {
		t.Fatal("init without --owner should error")
	}
	if _, statErr := os.Stat(config.ConfigPath(dir)); !os.IsNotExist(statErr) {
		t.Fatal("init must not write a config when owner is missing")
	}
}

func TestInit_DoesNotOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()

	// Pre-existing, customized workspace config.
	custom := config.Config{Owner: "acme-org", WorktreeRoot: "~/custom/worktrees"}
	if err := config.Save(config.ConfigPath(dir), custom); err != nil {
		t.Fatal(err)
	}

	out, err := runRoot(t, "dev", "init", "--dir", dir, "--owner", "someone-else")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out, "already configured") {
		t.Fatalf("output %q should report that the workspace already exists", out)
	}

	got, err := config.Load(config.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, custom) {
		t.Fatalf("config was modified without --force: got %+v, want %+v", got, custom)
	}
}

func TestInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()

	custom := config.Config{Owner: "acme-org", WorktreeRoot: "~/custom/worktrees"}
	if err := config.Save(config.ConfigPath(dir), custom); err != nil {
		t.Fatal(err)
	}

	if _, err := runRoot(t, "dev", "init", "--dir", dir, "--owner", "new-owner", "--force"); err != nil {
		t.Fatalf("init --force: %v", err)
	}

	got, err := config.Load(config.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if got.Owner != "new-owner" {
		t.Fatalf("owner after --force = %q, want new-owner", got.Owner)
	}
}

func TestInit_CreatesTargetDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")

	if _, err := runRoot(t, "dev", "init", "--dir", dir, "--owner", "acme"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(config.ConfigPath(dir)); err != nil {
		t.Fatalf("expected config file at %s: %v", config.ConfigPath(dir), err)
	}
}
