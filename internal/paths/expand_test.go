package paths

import (
	"path/filepath"
	"testing"
)

func TestExpand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // os.UserHomeDir uses HOME on unix

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "bare tilde", input: "~", want: home},
		{name: "tilde slash", input: "~/code/repos", want: filepath.Join(home, "code", "repos")},
		{name: "absolute unchanged", input: "/var/data", want: "/var/data"},
		{name: "relative unchanged", input: "code/repos", want: "code/repos"},
		{name: "empty unchanged", input: "", want: ""},
		{name: "tilde-user not supported", input: "~alice/x", want: "~alice/x"},
		{name: "tilde mid-string unchanged", input: "/opt/~/x", want: "/opt/~/x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Expand(tt.input)
			if err != nil {
				t.Fatalf("Expand(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("Expand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
