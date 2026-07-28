package picker

import (
	"strings"
	"testing"
)

func labelsFor(choices []WorktreeChoice) []string {
	options := buildWorktreeOptions(choices)
	labels := make([]string, len(options))
	for i, o := range options {
		labels[i] = o.Key
	}
	return labels
}

func TestBuildWorktreeOptions_MarksState(t *testing.T) {
	labels := labelsFor([]WorktreeChoice{
		{ID: "a", Repo: "web", Branch: "feature/x"},
		{ID: "b", Repo: "api", Branch: "feature/y", Unpushed: true},
		{ID: "c", Repo: "infra", Branch: "spike", Dirty: true},
		{ID: "d", Repo: "web", Branch: "old", Dirty: true, Unpushed: true},
	})

	for i, want := range []string{"", "[unpushed]", "[dirty]", "[dirty] [unpushed]"} {
		if want == "" {
			if strings.Contains(labels[i], "[") {
				t.Fatalf("label %q should carry no marker", labels[i])
			}
			continue
		}
		if !strings.HasSuffix(labels[i], want) {
			t.Fatalf("label %q should end with %q", labels[i], want)
		}
	}
}

func TestBuildWorktreeOptions_KeysAreWorktreeIDs(t *testing.T) {
	options := buildWorktreeOptions([]WorktreeChoice{
		{ID: "abc123", Repo: "web", Branch: "feature/x"},
	})
	if len(options) != 1 || options[0].Value != "abc123" {
		t.Fatalf("options = %+v, want the worktree id as the value", options)
	}
}

func TestBuildWorktreeOptions_LabelsDetachedHead(t *testing.T) {
	labels := labelsFor([]WorktreeChoice{{ID: "a", Repo: "web", Branch: ""}})
	if !strings.Contains(labels[0], "detached") {
		t.Fatalf("label = %q, want it to say detached", labels[0])
	}
}
