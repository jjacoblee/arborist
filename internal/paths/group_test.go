package paths

import (
	"errors"
	"testing"
)

func TestSanitizeGroupName(t *testing.T) {
	ok := []struct{ given, want string }{
		{"review", "review"},
		{"code review", "code-review"},
		{"Review/PRs", "Review-PRs"},
		{"spike_1", "spike_1"},
		{"v1.2", "v1.2"},
		// Separators collapse like they do in a branch name, which neutralizes
		// traversal without needing a separate rule for it.
		{"../etc", "..-etc"},
	}
	for _, tt := range ok {
		got, err := SanitizeGroupName(tt.given)
		if err != nil {
			t.Fatalf("SanitizeGroupName(%q): %v", tt.given, err)
		}
		if got != tt.want {
			t.Fatalf("SanitizeGroupName(%q) = %q, want %q", tt.given, got, tt.want)
		}
	}

	// A group becomes a directory name, so anything that would leave nothing
	// usable or climb out of the worktree root is rejected outright.
	bad := []string{"", "   ", ".", "..", "///", "-"}
	for _, given := range bad {
		got, err := SanitizeGroupName(given)
		if !errors.Is(err, ErrInvalidGroupName) {
			t.Fatalf("SanitizeGroupName(%q) = %q, %v; want ErrInvalidGroupName", given, got, err)
		}
	}
}
