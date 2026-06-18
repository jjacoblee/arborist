package worktree

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

// idLength is the number of hex characters in a worktree's full internal id.
// 12 hex chars (48 bits) is far more than enough to keep worktree ids distinct
// while staying short; the displayed id is shortened further (see ShortenIDs).
const idLength = 12

// minShortID is the shortest id Arborist will display, mirroring how git shows
// abbreviated object hashes.
const minShortID = 4

// ID returns a stable identifier for a worktree, derived from its absolute path.
// The same worktree always yields the same id, and no state needs to be stored.
func ID(path string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	return hex.EncodeToString(sum[:])[:idLength]
}

// ShortenIDs returns display ids: the shortest common prefix length (at least
// minShortID) that keeps every id distinct, applied uniformly. With a typical
// handful of worktrees this is minShortID characters.
func ShortenIDs(ids []string) []string {
	out := make([]string, len(ids))
	if len(ids) == 0 {
		return out
	}

	n := minShortID
	for n < idLength {
		seen := make(map[string]bool, len(ids))
		clash := false
		for _, id := range ids {
			p := prefix(id, n)
			if seen[p] {
				clash = true
				break
			}
			seen[p] = true
		}
		if !clash {
			break
		}
		n++
	}

	for i, id := range ids {
		out[i] = prefix(id, n)
	}
	return out
}

// isHexID reports whether s could be the prefix of a worktree id (lowercase
// hex). Used to decide whether a remove argument should be treated as an id.
func isHexID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func prefix(s string, n int) string {
	if n > len(s) {
		n = len(s)
	}
	return s[:n]
}
