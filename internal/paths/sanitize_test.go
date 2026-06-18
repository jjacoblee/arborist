package paths

import (
	"errors"
	"testing"
)

func TestValidateBranchName_Valid(t *testing.T) {
	valid := []string{
		"main",
		"feature/company-migration-flow",
		"release/1.2.0",
		"fix/bug_123",
		"user/feature/nested/path",
		"v1.0.0",
	}
	for _, name := range valid {
		if err := ValidateBranchName(name); err != nil {
			t.Errorf("ValidateBranchName(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateBranchName_Invalid(t *testing.T) {
	invalid := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"leading dash", "-branch"},
		{"leading slash", "/branch"},
		{"trailing slash", "branch/"},
		{"double slash", "feature//x"},
		{"dot dot", "feature/../etc"},
		{"single dot segment", "feature/./x"},
		{"lock suffix", "feature/x.lock"},
		{"space", "feature x"},
		{"tilde", "feature~x"},
		{"colon", "feature:x"},
		{"question", "feature?x"},
		{"asterisk", "feature*x"},
		{"backslash", "feature\\x"},
		{"control char", "feature\tx"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchName(tt.input)
			if err == nil {
				t.Fatalf("ValidateBranchName(%q) = nil, want error", tt.input)
			}
			if !errors.Is(err, ErrInvalidBranchName) {
				t.Fatalf("ValidateBranchName(%q) error should wrap ErrInvalidBranchName, got %v", tt.input, err)
			}
		})
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"slash separated", "feature/company-migration-flow", "feature-company-migration-flow"},
		{"nested slashes", "user/feature/x", "user-feature-x"},
		{"keeps dots and underscores", "release/1.2.0_rc1", "release-1.2.0_rc1"},
		{"collapses repeats", "feature///x", "feature-x"},
		{"trims edges", "/feature/x/", "feature-x"},
		{"spaces become dash", "feature x y", "feature-x-y"},
		{"unicode stripped", "feat/café", "feat-caf"},
		{"already safe", "main", "main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeBranchName(tt.input); got != tt.want {
				t.Fatalf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
