package worktree

import "testing"

func TestID_StableAndDistinct(t *testing.T) {
	a := ID("/work/acme/worktrees/web/feature-x")
	again := ID("/work/acme/worktrees/web/feature-x")
	b := ID("/work/acme/worktrees/web/feature-y")

	if a != again {
		t.Fatalf("ID should be stable: %q vs %q", a, again)
	}
	if a == b {
		t.Fatalf("different paths should yield different ids, both %q", a)
	}
	if len(a) != idLength {
		t.Fatalf("id length = %d, want %d", len(a), idLength)
	}
	// Ignores trailing-slash differences (paths are cleaned first).
	if ID("/work/acme/worktrees/web/feature-x/") != a {
		t.Fatal("ID should be insensitive to a trailing slash")
	}
}

func TestShortenIDs(t *testing.T) {
	// Distinct at the minimum length.
	got := ShortenIDs([]string{"aaaa1111", "bbbb2222"})
	if len(got) != 2 || got[0] != "aaaa" || got[1] != "bbbb" {
		t.Fatalf("ShortenIDs distinct = %v, want [aaaa bbbb]", got)
	}

	// Share the 4-char prefix; must grow to stay unique.
	got = ShortenIDs([]string{"aaaa1111", "aaaa2222"})
	if len(got) != 2 || got[0] != "aaaa1" || got[1] != "aaaa2" {
		t.Fatalf("ShortenIDs clashing = %v, want [aaaa1 aaaa2]", got)
	}

	if got := ShortenIDs(nil); len(got) != 0 {
		t.Fatalf("ShortenIDs(nil) = %v, want empty", got)
	}
}

func TestIsHexID(t *testing.T) {
	for _, s := range []string{"a3f9", "0", "abcdef0123"} {
		if !isHexID(s) {
			t.Fatalf("isHexID(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "feature/x", "ABC", "g123", "a-b"} {
		if isHexID(s) {
			t.Fatalf("isHexID(%q) = true, want false", s)
		}
	}
}
