package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/github"
)

// newRepoCmd builds the "arb repo" command group.
func newRepoCmd(d deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Work with GitHub repositories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newRepoListCmd(d))
	return cmd
}

// newRepoListCmd builds "arb repo list", which discovers repositories for the
// current workspace's owner through the GitHub CLI.
func newRepoListCmd(d deps) *cobra.Command {
	var (
		limit int
		dir   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List GitHub repositories (via the GitHub CLI)",
		Long: `List the repositories for this workspace's owner.

Run inside an owner workspace; the owner is taken from the workspace's
.arborist.json. Requires the GitHub CLI (gh) to be installed and authenticated:

  gh auth login`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			ws, err := requireWorkspace(dir)
			if err != nil {
				return err
			}

			h := github.New(d.runner)
			if err := requireGitHubCLI(ctx, h); err != nil {
				return err
			}

			repos, err := h.ListRepos(ctx, ws.Config.Owner, limit)
			if err != nil {
				return err
			}
			return printRepos(cmd.OutOrStdout(), repos)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", github.DefaultRepoLimit,
		"maximum number of repositories to fetch")
	addDirFlag(cmd, &dir)
	return cmd
}

// printRepos writes repositories as an aligned table.
func printRepos(w io.Writer, repos []github.Repository) error {
	if len(repos) == 0 {
		_, err := fmt.Fprintln(w, "No repositories found.")
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "REPOSITORY\tVISIBILITY")
	for _, r := range repos {
		visibility := "public"
		if r.IsPrivate {
			visibility = "private"
		}
		fmt.Fprintf(tw, "%s\t%s\n", r.NameWithOwner, visibility)
	}
	return tw.Flush()
}
