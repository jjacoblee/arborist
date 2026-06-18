package paths

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrInvalidBranchName indicates a branch name is empty or unsafe to use.
var ErrInvalidBranchName = errors.New("invalid branch name")

// forbiddenBranchRunes are characters git forbids in ref names (besides
// whitespace and control characters, which are checked separately).
const forbiddenBranchRunes = "~^:?*[\\"

// ValidateBranchName checks that name is usable as a git branch name. The rules
// are a practical subset of git's check-ref-format:
//
//   - not empty
//   - does not start with '-' (would look like a flag) or '/'
//   - does not end with '/' or ".lock"
//   - no empty, "." or ".." path segments (and therefore no "//" or "..")
//   - no whitespace or control characters
//   - none of: ~ ^ : ? * [ \
//
// It returns an error wrapping ErrInvalidBranchName when the name is rejected.
func ValidateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is empty", ErrInvalidBranchName)
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("%w: %q must not start with '-'", ErrInvalidBranchName, name)
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return fmt.Errorf("%w: %q must not start or end with '/'", ErrInvalidBranchName, name)
	}
	if strings.HasSuffix(name, ".lock") {
		return fmt.Errorf("%w: %q must not end with '.lock'", ErrInvalidBranchName, name)
	}

	for _, r := range name {
		if r <= ' ' || r == 0x7f {
			return fmt.Errorf("%w: %q must not contain whitespace or control characters", ErrInvalidBranchName, name)
		}
		if strings.ContainsRune(forbiddenBranchRunes, r) {
			return fmt.Errorf("%w: %q must not contain %q", ErrInvalidBranchName, name, r)
		}
	}

	for _, seg := range strings.Split(name, "/") {
		switch seg {
		case "":
			return fmt.Errorf("%w: %q must not contain empty path segments ('//')", ErrInvalidBranchName, name)
		case ".", "..":
			return fmt.Errorf("%w: %q must not contain '.' or '..' segments", ErrInvalidBranchName, name)
		}
	}
	return nil
}

// unsafePathChars matches any run of characters that are not safe in a single
// filesystem path segment.
var unsafePathChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// SanitizeBranchName converts a branch name into a single, filesystem-safe path
// segment. Runs of unsafe characters (including '/') become a single '-', and
// leading/trailing/duplicate dashes are removed. For example:
//
//	"feature/company-migration-flow" -> "feature-company-migration-flow"
//
// Sanitization is for directory naming only; the original, unmodified branch
// name is always used for git operations.
func SanitizeBranchName(name string) string {
	s := unsafePathChars.ReplaceAllString(name, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
