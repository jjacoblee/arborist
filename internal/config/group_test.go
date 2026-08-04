package config

import "testing"

func TestValidate_RejectsUnusableGroupNames(t *testing.T) {
	if err := (Config{Owner: "acme", DefaultGroup: ".."}).Validate(); err == nil {
		t.Fatal("a defaultGroup that cannot be a folder name must be rejected")
	}
	if err := (Config{Owner: "acme", Groups: []string{"review", ""}}).Validate(); err == nil {
		t.Fatal("an empty declared group must be rejected")
	}
	if err := (Config{Owner: "acme", Groups: []string{"review"}, DefaultGroup: "review"}).Validate(); err != nil {
		t.Fatalf("a normal group config should validate: %v", err)
	}
}

func TestKnownGroup_ComparesSanitizedNames(t *testing.T) {
	c := Config{Owner: "acme", Groups: []string{"code review", "spike"}}

	// "code review" becomes the folder "code-review", so both spellings name
	// the same declared group.
	for _, name := range []string{"code review", "code-review", "spike"} {
		if !c.KnownGroup(name) {
			t.Fatalf("KnownGroup(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"reveiw", "", "other"} {
		if c.KnownGroup(name) {
			t.Fatalf("KnownGroup(%q) = true, want false", name)
		}
	}
}
