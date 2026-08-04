package paths

import (
	"errors"
	"fmt"
)

// ErrInvalidGroupName indicates a group name cannot be used as a directory.
var ErrInvalidGroupName = errors.New("invalid group name")

// SanitizeGroupName converts a group name into a single, filesystem-safe path
// segment, using the same rules as branch names: runs of unsafe characters
// (including '/') collapse to a single '-', and leading, trailing and duplicate
// dashes are dropped.
//
// A group becomes a directory under the worktree root, so a name that leaves
// nothing usable ("", "///") or that would climb out of the tree (".", "..") is
// rejected rather than quietly turned into something else.
func SanitizeGroupName(name string) (string, error) {
	clean := SanitizeBranchName(name)
	switch clean {
	case "", ".", "..":
		return "", fmt.Errorf("%w: %q is not usable as a folder name", ErrInvalidGroupName, name)
	}
	return clean, nil
}
