package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/exec"
	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/worktree"
)

// newOpenCmd builds "arb open <id-or-branch>".
func newOpenCmd(d deps) *cobra.Command {
	var (
		dir       string
		useCursor bool
		useCode   bool
		editor    string
		printPath bool
	)

	cmd := &cobra.Command{
		Use:   "open <id-or-branch>",
		Short: "Open a worktree in your editor (or print its path)",
		Long: `Open a worktree, identified by the short id from "arb list" or by a branch
name that matches exactly one worktree.

The editor is chosen, in order, from: --cursor / --code / --editor, the "editor"
config value, then the $EDITOR environment variable. Use --print to output the
worktree's path instead of opening it (handy for cd).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			g := git.New(d.runner)
			if err := requireGit(ctx, g); err != nil {
				return err
			}
			ws, err := requireWorkspace(dir)
			if err != nil {
				return err
			}
			svc, err := newWorktreeService(g, github.New(d.runner), ws)
			if err != nil {
				return err
			}

			wt, err := resolveOne(ctx, svc, out, ref)
			if err != nil {
				return err
			}

			if printPath {
				fmt.Fprintln(out, wt.Path)
				return nil
			}

			editorCmd, err := resolveEditor(useCursor, useCode, editor, ws.Config.Editor)
			if err != nil {
				return err
			}
			name, eargs := splitEditor(editorCmd)
			eargs = append(eargs, wt.Path)
			if err := d.launcher.Launch(ctx, name, eargs...); err != nil {
				if errors.Is(err, exec.ErrNotFound) {
					return fmt.Errorf("editor %q was not found on your PATH.\n\n"+
						"Install it, choose another with\n  arb config set editor <command>\n"+
						"or pass --cursor / --code / --editor <command>.", name)
				}
				return err
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&useCursor, "cursor", false, "open in Cursor")
	cmd.Flags().BoolVar(&useCode, "code", false, "open in VS Code")
	cmd.Flags().StringVar(&editor, "editor", "", "open with a specific editor command")
	cmd.Flags().BoolVarP(&printPath, "print", "p", false, "print the worktree path instead of opening it")
	cmd.MarkFlagsMutuallyExclusive("cursor", "code", "editor", "print")
	addDirFlag(cmd, &dir)
	return cmd
}

// resolveOne resolves ref to exactly one worktree, printing guidance and
// returning an error when zero or several match.
func resolveOne(ctx context.Context, svc worktree.Service, out io.Writer, ref string) (worktree.ManagedWorktree, error) {
	matches, err := svc.Find(ctx, ref)
	if err != nil {
		var amb *worktree.AmbiguousIDError
		if errors.As(err, &amb) {
			printCandidates(out, ref, amb.Matches)
			return worktree.ManagedWorktree{}, fmt.Errorf("ambiguous worktree id %q; use more characters", ref)
		}
		return worktree.ManagedWorktree{}, err
	}

	switch len(matches) {
	case 0:
		return worktree.ManagedWorktree{}, fmt.Errorf("no worktree found for %q", ref)
	case 1:
		return matches[0], nil
	default:
		printCandidates(out, ref, matches)
		return worktree.ManagedWorktree{}, fmt.Errorf("%q matches %d worktrees; use a worktree id instead", ref, len(matches))
	}
}

func printCandidates(out io.Writer, ref string, matches []worktree.ManagedWorktree) {
	fmt.Fprintf(out, "%q matches several worktrees:\n\n", ref)
	ids := make([]string, len(matches))
	for i, wt := range matches {
		ids[i] = wt.ID
	}
	short := worktree.ShortenIDs(ids)
	for i, wt := range matches {
		fmt.Fprintf(out, "  %s  %s/%s  %s\n", short[i], wt.Owner, wt.Repo, wt.Branch)
	}
	fmt.Fprintln(out)
}

// resolveEditor picks the editor command from flags, then config, then $EDITOR.
func resolveEditor(useCursor, useCode bool, editorFlag, configEditor string) (string, error) {
	switch {
	case useCursor:
		return "cursor", nil
	case useCode:
		return "code", nil
	case editorFlag != "":
		return editorFlag, nil
	case configEditor != "":
		return configEditor, nil
	}
	if env := os.Getenv("EDITOR"); env != "" {
		return env, nil
	}
	return "", errors.New("no editor configured.\n\n" +
		"Pass --cursor, --code, or --editor <command>, set a default with\n" +
		"  arb config set editor <command>\n" +
		"or set the $EDITOR environment variable.")
}

// splitEditor splits an editor command into the program and its leading
// arguments, e.g. "code --wait" -> ("code", ["--wait"]).
func splitEditor(s string) (string, []string) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}
