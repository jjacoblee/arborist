package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func validConfig() Config {
	return Config{Owner: "acme"}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "owner only", cfg: Config{Owner: "acme"}, wantErr: false},
		{name: "owner and worktreeRoot", cfg: Config{Owner: "acme", WorktreeRoot: "~/wt"}, wantErr: false},
		{name: "empty owner", cfg: Config{Owner: ""}, wantErr: true},
		{name: "owner with space", cfg: Config{Owner: "acme org"}, wantErr: true},
		{name: "owner with slash", cfg: Config{Owner: "acme/web"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestSetupCommands(t *testing.T) {
	c := Config{Setup: map[string][]string{
		"web": {"pnpm install", "uv sync"},
		"*":   {"echo default"},
	}}
	if got := c.SetupCommands("web"); len(got) != 2 || got[0] != "pnpm install" {
		t.Fatalf("exact repo = %v, want [pnpm install uv sync]", got)
	}
	if got := c.SetupCommands("api"); len(got) != 1 || got[0] != "echo default" {
		t.Fatalf("fallback = %v, want [echo default]", got)
	}
	if got := (Config{}).SetupCommands("web"); got != nil {
		t.Fatalf("no setup configured should be nil, got %v", got)
	}
}

func TestResolveWorktreeRoot(t *testing.T) {
	const root = "/home/u/work/acme"
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "default", cfg: Config{Owner: "acme"}, want: filepath.Join(root, "worktrees")},
		{name: "relative", cfg: Config{Owner: "acme", WorktreeRoot: "trees"}, want: filepath.Join(root, "trees")},
		{name: "absolute", cfg: Config{Owner: "acme", WorktreeRoot: "/srv/wt"}, want: "/srv/wt"},
		{name: "tilde kept", cfg: Config{Owner: "acme", WorktreeRoot: "~/wt"}, want: "~/wt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ResolveWorktreeRoot(root); got != tt.want {
				t.Fatalf("ResolveWorktreeRoot() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	// Path includes a missing parent dir to confirm Save creates it.
	path := filepath.Join(t.TempDir(), "nested", FileName)

	want := Config{
		Owner:        "acme-org",
		WorktreeRoot: "~/work/acme/worktrees",
		CopyEnvFiles: true,
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip mismatch:\n got = %+v\nwant = %+v", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil {
		t.Fatal("Load() of a missing file should error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load() error should wrap os.ErrNotExist, got: %v", err)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() of invalid JSON should error")
	}
}

func TestLoadValidJSONInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	// Well-formed JSON, but owner is missing.
	if err := os.WriteFile(path, []byte(`{"worktreeRoot":"~/w"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() of a valid-JSON but invalid config should error")
	}
}

func TestSaveRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := Save(path, Config{}); err == nil {
		t.Fatal("Save() should refuse to write an invalid config")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Save() must not create a file when the config is invalid")
	}
}

func TestLoad_RejectsWorldWritable(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := Save(path, validConfig()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o666); err != nil { // group/other writable
		t.Fatal(err)
	}
	if _, err := Load(path); !errors.Is(err, ErrUntrustedConfig) {
		t.Fatalf("a world-writable config should be rejected, got: %v", err)
	}
}

func TestLoad_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.json")
	if err := Save(real, validConfig()); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, FileName)
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(link); !errors.Is(err, ErrUntrustedConfig) {
		t.Fatalf("a symlinked config should be rejected, got: %v", err)
	}
}

func TestLoad_AcceptsOwnerOnlyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := Save(path, validConfig()); err != nil { // Save writes 0o600
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("a private, owner-written config should load: %v", err)
	}
}
