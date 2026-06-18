package picker

import (
	"context"
	"errors"
	"testing"

	"github.com/jjacoblee/arborist/internal/github"
)

func sampleRepos() []github.Repository {
	return []github.Repository{
		{Name: "web-app", NameWithOwner: "acme/web-app", Owner: "acme", IsPrivate: true},
		{Name: "docs", NameWithOwner: "acme/docs", Owner: "acme", IsPrivate: false},
	}
}

func TestSelect_EmptyReturnsErr(t *testing.T) {
	_, err := Huh{}.Select(context.Background(), "feature/x", nil)
	if !errors.Is(err, ErrNoRepositories) {
		t.Fatalf("Select(empty) = %v, want ErrNoRepositories", err)
	}
}

func TestBuildOptions(t *testing.T) {
	repos := sampleRepos()
	options, index := buildOptions(repos)

	if len(options) != 2 {
		t.Fatalf("got %d options, want 2", len(options))
	}
	// Private repos are labeled; the option value is the stable key.
	if options[0].Key != "acme/web-app (private)" {
		t.Fatalf("option[0].Key = %q, want private label", options[0].Key)
	}
	if options[0].Value != "acme/web-app" {
		t.Fatalf("option[0].Value = %q, want acme/web-app", options[0].Value)
	}
	if options[1].Key != "acme/docs" {
		t.Fatalf("option[1].Key = %q, want plain label (public)", options[1].Key)
	}
	if got, ok := index["acme/web-app"]; !ok || got.Name != "web-app" {
		t.Fatalf("index lookup failed: %+v ok=%v", got, ok)
	}
}

func TestResolve(t *testing.T) {
	_, index := buildOptions(sampleRepos())

	got := resolve([]string{"acme/docs", "acme/web-app"}, index)
	if len(got) != 2 {
		t.Fatalf("got %d repos, want 2", len(got))
	}
	// Order is preserved as selected.
	if got[0].NameWithOwner != "acme/docs" || got[1].NameWithOwner != "acme/web-app" {
		t.Fatalf("resolve did not preserve order: %+v", got)
	}

	// Unknown keys are skipped.
	if r := resolve([]string{"nope/missing"}, index); len(r) != 0 {
		t.Fatalf("resolve(unknown) = %+v, want empty", r)
	}
}
