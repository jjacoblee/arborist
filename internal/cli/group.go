package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/config"
	"github.com/jjacoblee/arborist/internal/paths"
	"github.com/jjacoblee/arborist/internal/worktree"
)

// errGroupDeclined indicates the user chose not to create a new group. It
// unwinds the command without being a failure — the caller reports a clean stop.
var errGroupDeclined = errors.New("group not created")

// resolveGroup decides which group new worktrees belong to, and confirms an
// unfamiliar group name before a folder is created for it.
//
// The flag wins over the workspace's defaultGroup, and an explicitly empty
// --group opts out of that default — which is why the flag's presence is read
// from Cobra rather than inferred from an empty value.
//
// Groups are free-form and created on demand, so nothing stops a typo becoming
// a near-duplicate folder ("reveiw" next to "review") that goes unnoticed until
// it has accumulated work. A name that is neither already in use nor declared in
// the config is therefore queried once, listing the groups that do exist. --yes
// skips the question, keeping scripted runs unattended.
func resolveGroup(cmd *cobra.Command, d deps, svc worktree.Service, cfg config.Config,
	flagValue string, assumeYes bool) (string, error) {
	name := flagValue
	if !cmd.Flags().Changed("group") {
		name = cfg.DefaultGroup
	}
	if name == "" {
		return "", nil
	}

	group, err := paths.SanitizeGroupName(name)
	if err != nil {
		return "", err
	}
	if cfg.KnownGroup(group) || assumeYes {
		return group, nil
	}

	existing, err := svc.Groups()
	if err != nil {
		return "", err
	}
	for _, g := range existing {
		if g == group {
			return group, nil
		}
	}

	out := cmd.OutOrStdout()
	if len(existing) == 0 {
		fmt.Fprintf(out, "%q is a new group; no groups exist yet.\n", group)
	} else {
		fmt.Fprintf(out, "%q is a new group (existing: %s).\n", group, strings.Join(existing, ", "))
	}
	ok, err := d.confirmer.Confirm(cmd.Context(), fmt.Sprintf("Create the group %q?", group))
	if err != nil {
		return "", err
	}
	if !ok {
		fmt.Fprintln(out, "Aborted. Nothing was created.")
		return "", errGroupDeclined
	}
	return group, nil
}
