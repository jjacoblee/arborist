package github

import (
	"errors"
	"strings"
	"testing"
)

func testRepos() []Repository {
	return []Repository{
		{Name: "api", NameWithOwner: "acme/api", Owner: "acme"},
		{Name: "web", NameWithOwner: "acme/web", Owner: "acme"},
		{Name: "infra", NameWithOwner: "acme/infra", Owner: "acme"},
	}
}

func TestMatchRepos_ByBareName(t *testing.T) {
	got, err := MatchRepos(testRepos(), []string{"web", "api"})
	if err != nil {
		t.Fatalf("MatchRepos: %v", err)
	}
	if len(got) != 2 || got[0].Name != "web" || got[1].Name != "api" {
		t.Fatalf("matched %+v, want web then api (the order given)", got)
	}
}

func TestMatchRepos_ByOwnerQualifiedName(t *testing.T) {
	got, err := MatchRepos(testRepos(), []string{"acme/infra"})
	if err != nil {
		t.Fatalf("MatchRepos: %v", err)
	}
	if len(got) != 1 || got[0].NameWithOwner != "acme/infra" {
		t.Fatalf("matched %+v, want acme/infra", got)
	}
}

func TestMatchRepos_IsCaseInsensitive(t *testing.T) {
	got, err := MatchRepos(testRepos(), []string{"API", "ACME/Web"})
	if err != nil {
		t.Fatalf("MatchRepos: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("matched %+v, want both", got)
	}
}

func TestMatchRepos_UnknownNamesAreReportedTogether(t *testing.T) {
	_, err := MatchRepos(testRepos(), []string{"api", "wbe", "nope"})

	var unknown *UnknownRepositoriesError
	if !errors.As(err, &unknown) {
		t.Fatalf("err = %v, want UnknownRepositoriesError", err)
	}
	if len(unknown.Names) != 2 {
		t.Fatalf("unknown = %v, want both bad names in one error", unknown.Names)
	}
	if !strings.Contains(err.Error(), "wbe") || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("error %q should name every unknown repository", err)
	}
}
