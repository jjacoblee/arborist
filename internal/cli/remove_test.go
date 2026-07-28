package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/exectest"
	"github.com/jjacoblee/arborist/internal/pickertest"
	"github.com/jjacoblee/arborist/internal/worktree"
)

func runRemove(t *testing.T, fake *exectest.Fake, conf *pickertest.FakeConfirmer, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd("dev", deps{runner: fake, confirmer: conf})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// removeFixture sets up a workspace with one worktree for branch "feature/x"
// and returns the workspace dir, the worktree path, and a runner pre-wired for
// it.
func removeFixture(t *testing.T, dirty bool) (dir, wtPath string, fake *exectest.Fake) {
	t.Helper()
	dir = writeWorkspace(t, "acme")
	wtPath = filepath.Join(workspaceWorktreeRoot(dir), "web", "feature-x") // nested layout
	mkWorktree(t, wtPath)
	baseRepo := filepath.Join(dir, "web")

	status := exectest.Result{} // clean
	if dirty {
		status = exectest.Result{Out: []byte(" M file.go\n")}
	}
	fake = &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", wtPath, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(baseRepo + "/.git\n")},
			exectest.Key("git", "-C", wtPath, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", wtPath, "status", "--porcelain"):                                   status,
		},
	}
	return dir, wtPath, fake
}

func calledWorktreeRemove(fake *exectest.Fake) bool {
	for _, c := range fake.Calls {
		if c.Name == "git" && len(c.Args) >= 4 && c.Args[2] == "worktree" && c.Args[3] == "remove" {
			return true
		}
	}
	return false
}

// branchDeleteFlag returns the flag git was asked to delete branch with ("-d"
// or "-D"), or "" if no branch deletion was attempted.
func branchDeleteFlag(fake *exectest.Fake) string {
	for _, c := range fake.Calls {
		if c.Name == "git" && len(c.Args) >= 5 && c.Args[2] == "branch" {
			return c.Args[3]
		}
	}
	return ""
}

func TestRemove_DeleteBranchFlag(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{} // unused due to --yes

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir, "--delete-branch", "--yes")
	if err != nil {
		t.Fatalf("remove --delete-branch: %v\n%s", err, out)
	}
	if conf.Calls != 0 {
		t.Fatalf("--delete-branch with --yes should not prompt, got %d prompts", conf.Calls)
	}
	if got := branchDeleteFlag(fake); got != "-d" {
		t.Fatalf("branch delete flag = %q, want -d; calls: %+v", got, fake.Calls)
	}
	if !strings.Contains(out, "Deleted branch feature/x") {
		t.Fatalf("expected a branch deletion summary, got:\n%s", out)
	}
}

func TestRemove_PromptsToDeleteOrphanedBranch(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{Result: true} // yes to both prompts

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir)
	if err != nil {
		t.Fatalf("remove: %v\n%s", err, out)
	}
	if len(conf.Prompts) != 2 {
		t.Fatalf("expected removal + branch prompts, got %q", conf.Prompts)
	}
	if !strings.Contains(conf.Prompts[1], "Delete the local branch") {
		t.Fatalf("second prompt = %q, want it to offer branch deletion", conf.Prompts[1])
	}
	if !strings.Contains(out, "Branch feature/x in web now has no worktrees") {
		t.Fatalf("expected the orphaned branch to be named, got:\n%s", out)
	}
	if got := branchDeleteFlag(fake); got != "-d" {
		t.Fatalf("branch delete flag = %q, want -d; calls: %+v", got, fake.Calls)
	}
}

func TestRemove_KeepsBranchStillInUse(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	// Another worktree in the same repo is still on feature/x, so the branch is
	// not orphaned and must never be offered for deletion.
	baseRepo := filepath.Join(dir, "web")
	fake.Responses[exectest.Key("git", "-C", baseRepo, "worktree", "list", "--porcelain")] = exectest.Result{
		Out: []byte("worktree /elsewhere/web-feature-x\nHEAD abc123\nbranch refs/heads/feature/x\n\n"),
	}
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir)
	if err != nil {
		t.Fatalf("remove: %v\n%s", err, out)
	}
	if len(conf.Prompts) != 1 {
		t.Fatalf("expected only the removal prompt, got %q", conf.Prompts)
	}
	if branchDeleteFlag(fake) != "" {
		t.Fatalf("must not delete a branch another worktree still has; calls: %+v", fake.Calls)
	}
}

func TestRemove_UnmergedBranchIsSkippedWithForceHint(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	baseRepo := filepath.Join(dir, "web")
	fake.Responses[exectest.Key("git", "-C", baseRepo, "branch", "-d", "feature/x")] = exectest.Result{
		Err: errors.New("run git: exit status 1: error: the branch 'feature/x' is not fully merged"),
	}
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir, "--delete-branch")
	if err != nil {
		t.Fatalf("a refused branch deletion is not a command failure: %v\n%s", err, out)
	}
	if !strings.Contains(out, "unmerged commits") || !strings.Contains(out, "--force") {
		t.Fatalf("expected a skip explaining --force, got:\n%s", out)
	}
	if !strings.Contains(out, "Removed acme/web") {
		t.Fatalf("the worktree removal must still be reported, got:\n%s", out)
	}
}

func TestRemove_DeleteBranchWithForceUsesCapitalD(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{}

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir, "--delete-branch", "--force", "--yes")
	if err != nil {
		t.Fatalf("remove --delete-branch --force: %v\n%s", err, out)
	}
	if got := branchDeleteFlag(fake); got != "-D" {
		t.Fatalf("branch delete flag = %q, want -D; calls: %+v", got, fake.Calls)
	}
}

func TestRemove_YesWithoutDeleteBranchKeepsBranch(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir, "--yes")
	if err != nil {
		t.Fatalf("remove --yes: %v\n%s", err, out)
	}
	if conf.Calls != 0 {
		t.Fatalf("--yes must not prompt at all, got %q", conf.Prompts)
	}
	if branchDeleteFlag(fake) != "" {
		t.Fatalf("an unattended run must not delete a branch nobody asked for; calls: %+v", fake.Calls)
	}
}

func TestRemove_Confirmed(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	// Yes to the removal, no to the follow-up branch prompt: this test is about
	// the worktree going away, not the branch.
	conf := &pickertest.FakeConfirmer{Results: []bool{true, false}}

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir)
	if err != nil {
		t.Fatalf("remove: %v\n%s", err, out)
	}
	if len(conf.Prompts) == 0 || !strings.Contains(conf.Prompts[0], "Remove 1 worktree") {
		t.Fatalf("first prompt should confirm the removal, got %q", conf.Prompts)
	}
	if branchDeleteFlag(fake) != "" {
		t.Fatalf("must not delete the branch when the user declines; calls: %+v", fake.Calls)
	}
	if !strings.Contains(out, "Removed acme/web") {
		t.Fatalf("expected a removal summary, got:\n%s", out)
	}
	if !calledWorktreeRemove(fake) {
		t.Fatalf("expected git worktree remove to be called; calls: %+v", fake.Calls)
	}
}

func TestRemove_ByID(t *testing.T) {
	dir, wtPath, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{Result: true}

	id := worktree.ID(wtPath)
	out, err := runRemove(t, fake, conf, "remove", id[:6], "--dir", dir)
	if err != nil {
		t.Fatalf("remove by id: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Removed acme/web") {
		t.Fatalf("expected a removal summary, got:\n%s", out)
	}
	if !calledWorktreeRemove(fake) {
		t.Fatalf("expected git worktree remove; calls: %+v", fake.Calls)
	}
}

func TestRemove_Aborted(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{Result: false}

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out, "Aborted") {
		t.Fatalf("expected an abort message, got:\n%s", out)
	}
	if calledWorktreeRemove(fake) {
		t.Fatal("must not remove anything when the user declines")
	}
}

func TestRemove_NoMatches(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runRemove(t, fake, conf, "remove", "does/not-exist", "--dir", dir)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out, "No worktrees found") {
		t.Fatalf("expected a no-matches message, got:\n%s", out)
	}
	if conf.Calls != 0 {
		t.Fatal("should not prompt when there are no matches")
	}
}

func TestRemove_DirtySkippedWithoutForce(t *testing.T) {
	dir, _, fake := removeFixture(t, true) // dirty
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out, "uncommitted changes") {
		t.Fatalf("expected a dirty warning, got:\n%s", out)
	}
	if conf.Calls != 0 {
		t.Fatal("should not prompt when nothing is removable")
	}
	if calledWorktreeRemove(fake) {
		t.Fatal("must never remove a dirty worktree without --force")
	}
}

func TestRemove_DirtyWithForceAndYes(t *testing.T) {
	dir, _, fake := removeFixture(t, true) // dirty
	conf := &pickertest.FakeConfirmer{}    // unused due to --yes

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir, "--force", "--yes")
	if err != nil {
		t.Fatalf("remove --force --yes: %v\n%s", err, out)
	}
	if conf.Calls != 0 {
		t.Fatal("--yes should skip the confirmation prompt")
	}
	if !calledWorktreeRemove(fake) {
		t.Fatal("expected removal with --force")
	}
	if !strings.Contains(out, "Removed acme/web") {
		t.Fatalf("expected removal summary, got:\n%s", out)
	}
}

// pruneFixture sets up a workspace with one base repo whose local branches are
// the given names and which has no worktrees at all.
func pruneFixture(t *testing.T, branches ...string) (dir string, fake *exectest.Fake) {
	t.Helper()
	dir = writeWorkspace(t, "acme")
	webRepo := filepath.Join(dir, "web")
	if err := os.MkdirAll(webRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	fake = &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", webRepo, "for-each-ref", "--format=%(refname:short)", "refs/heads"): {
				Out: []byte(strings.Join(branches, "\n") + "\n"),
			},
		},
	}
	return dir, fake
}

func TestPrune_OffersToDeleteOrphanedBranches(t *testing.T) {
	dir, fake := pruneFixture(t, "main", "feature/x")
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runRemove(t, fake, conf, "prune", "--dir", dir)
	if err != nil {
		t.Fatalf("prune: %v\n%s", err, out)
	}
	if !strings.Contains(out, "feature/x") {
		t.Fatalf("expected the orphaned branch to be reported, got:\n%s", out)
	}
	if strings.Contains(out, "main") {
		t.Fatalf("the default branch must never be listed as orphaned, got:\n%s", out)
	}
	if conf.Calls != 1 {
		t.Fatalf("expected one prompt offering deletion, got %q", conf.Prompts)
	}
	if got := branchDeleteFlag(fake); got != "-d" {
		t.Fatalf("branch delete flag = %q, want -d; calls: %+v", got, fake.Calls)
	}
	if !strings.Contains(out, "Deleted branch feature/x") {
		t.Fatalf("expected a deletion summary, got:\n%s", out)
	}
}

func TestPrune_DeclinedLeavesBranchesAlone(t *testing.T) {
	dir, fake := pruneFixture(t, "main", "feature/x")
	conf := &pickertest.FakeConfirmer{Result: false}

	out, err := runRemove(t, fake, conf, "prune", "--dir", dir)
	if err != nil {
		t.Fatalf("prune: %v\n%s", err, out)
	}
	if branchDeleteFlag(fake) != "" {
		t.Fatalf("must not delete when declined; calls: %+v", fake.Calls)
	}
}

func TestPrune_YesDoesNotPromptOrDelete(t *testing.T) {
	dir, fake := pruneFixture(t, "main", "feature/x")
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runRemove(t, fake, conf, "prune", "--dir", dir, "--yes")
	if err != nil {
		t.Fatalf("prune --yes: %v\n%s", err, out)
	}
	if conf.Calls != 0 {
		t.Fatalf("--yes must keep prune non-interactive, got %q", conf.Prompts)
	}
	if branchDeleteFlag(fake) != "" {
		t.Fatalf("an unattended prune must not delete branches; calls: %+v", fake.Calls)
	}
	if !strings.Contains(out, "feature/x") {
		t.Fatalf("orphaned branches should still be reported, got:\n%s", out)
	}
}

func TestPrune_DeleteBranchesFlagSkipsPrompt(t *testing.T) {
	dir, fake := pruneFixture(t, "main", "feature/x")
	conf := &pickertest.FakeConfirmer{}

	out, err := runRemove(t, fake, conf, "prune", "--dir", dir, "--delete-branches", "--yes")
	if err != nil {
		t.Fatalf("prune --delete-branches: %v\n%s", err, out)
	}
	if conf.Calls != 0 {
		t.Fatalf("--delete-branches with --yes should not prompt, got %q", conf.Prompts)
	}
	if got := branchDeleteFlag(fake); got != "-d" {
		t.Fatalf("branch delete flag = %q, want -d; calls: %+v", got, fake.Calls)
	}
}

func TestPrune(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	if err := os.MkdirAll(filepath.Join(dir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := &exectest.Fake{} // Default success: git --version, IsRepo true, prune ok

	out, err := runRemove(t, fake, &pickertest.FakeConfirmer{}, "prune", "--dir", dir)
	if err != nil {
		t.Fatalf("prune: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Pruned 1 repository") {
		t.Fatalf("expected prune summary, got:\n%s", out)
	}
}
