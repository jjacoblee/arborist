package cli

import (
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/config"
)

func TestConfig_ListsGroupsAndDefaultGroup(t *testing.T) {
	dir := writeGroupWorkspace(t, []string{"review", "spike"}, "review")

	out, err := runRoot(t, "dev", "config", "--dir", dir)
	if err != nil {
		t.Fatalf("config: %v\n%s", err, out)
	}
	if !strings.Contains(out, "groups:        review, spike") {
		t.Fatalf("config should print groups, got:\n%s", out)
	}
	if !strings.Contains(out, "defaultGroup:  review") {
		t.Fatalf("config should print defaultGroup, got:\n%s", out)
	}
}

func TestConfig_ListsUnsetGroupsAsNone(t *testing.T) {
	dir := writeWorkspace(t, "acme")

	out, err := runRoot(t, "dev", "config", "--dir", dir)
	if err != nil {
		t.Fatalf("config: %v\n%s", err, out)
	}
	if !strings.Contains(out, "groups:        (none)") {
		t.Fatalf("unset groups should print (none), got:\n%s", out)
	}
	if !strings.Contains(out, "defaultGroup:  (none)") {
		t.Fatalf("unset defaultGroup should print (none), got:\n%s", out)
	}
}

func TestConfigGetSet_DefaultGroup(t *testing.T) {
	dir := writeWorkspace(t, "acme")

	out, err := runRoot(t, "dev", "config", "set", "defaultGroup", "review", "--dir", dir)
	if err != nil {
		t.Fatalf("config set defaultGroup: %v\n%s", err, out)
	}

	got, err := runRoot(t, "dev", "config", "get", "defaultGroup", "--dir", dir)
	if err != nil {
		t.Fatalf("config get defaultGroup: %v\n%s", err, got)
	}
	if strings.TrimSpace(got) != "review" {
		t.Fatalf("get defaultGroup = %q, want review", got)
	}

	cfg, err := config.Load(config.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultGroup != "review" {
		t.Fatalf("saved defaultGroup = %q, want review", cfg.DefaultGroup)
	}
}

func TestConfigSet_ClearsDefaultGroup(t *testing.T) {
	dir := writeGroupWorkspace(t, nil, "review")

	if _, err := runRoot(t, "dev", "config", "set", "defaultGroup", "", "--dir", dir); err != nil {
		t.Fatalf("config set defaultGroup \"\": %v", err)
	}

	cfg, err := config.Load(config.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultGroup != "" {
		t.Fatalf("cleared defaultGroup = %q, want empty", cfg.DefaultGroup)
	}
}

func TestConfigSet_RejectsUnusableDefaultGroup(t *testing.T) {
	dir := writeWorkspace(t, "acme")

	_, err := runRoot(t, "dev", "config", "set", "defaultGroup", "..", "--dir", dir)
	if err == nil {
		t.Fatal("config set defaultGroup .. should fail")
	}

	cfg, err := config.Load(config.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultGroup != "" {
		t.Fatalf("invalid defaultGroup must not be saved, got %q", cfg.DefaultGroup)
	}
}
