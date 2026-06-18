package cli

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/config"
)

// configKeys are the editable keys exposed by "arb config get/set".
const configKeys = "owner, worktreeRoot, copyEnvFiles, editor"

// newConfigCmd builds the "arb config" command group for viewing and editing
// the current workspace's .arborist.json without hand-editing the hidden file.
func newConfigCmd(_ deps) *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and edit the workspace configuration",
		Long: `View and edit the current workspace's .arborist.json.

With no subcommand it prints the resolved configuration. Use "get" and "set" to
read or change individual values, and "path" to print the config file location.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigList(cmd, dir)
		},
	}
	addDirFlag(cmd, &dir)

	cmd.AddCommand(newConfigListCmd())
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigPathCmd())
	return cmd
}

func newConfigListCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Print the resolved workspace configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigList(cmd, dir)
		},
	}
	addDirFlag(cmd, &dir)
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Print one configuration value (" + configKeys + ")",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := requireWorkspace(dir)
			if err != nil {
				return err
			}
			val, err := configValue(ws, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		},
	}
	addDirFlag(cmd, &dir)
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value (" + configKeys + ")",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := requireWorkspace(dir)
			if err != nil {
				return err
			}
			cfg, err := setConfigValue(ws.Config, args[0], args[1])
			if err != nil {
				return err
			}
			if err := config.Save(ws.Path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", args[0], args[1])
			return nil
		},
	}
	addDirFlag(cmd, &dir)
	return cmd
}

func newConfigPathCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Print the workspace config file path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, err := requireWorkspace(dir)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), ws.Path)
			return nil
		},
	}
	addDirFlag(cmd, &dir)
	return cmd
}

func runConfigList(cmd *cobra.Command, dir string) error {
	ws, err := requireWorkspace(dir)
	if err != nil {
		return err
	}
	printConfig(cmd.OutOrStdout(), ws)
	return nil
}

func printConfig(w io.Writer, ws config.Workspace) {
	fmt.Fprintf(w, "config:        %s\n", ws.Path)
	fmt.Fprintf(w, "workspaceRoot: %s\n", ws.Root)
	fmt.Fprintf(w, "owner:         %s\n", ws.Config.Owner)
	fmt.Fprintf(w, "worktreeRoot:  %s\n", ws.Config.ResolveWorktreeRoot(ws.Root))
	fmt.Fprintf(w, "copyEnvFiles:  %v\n", ws.Config.CopyEnvFiles)
	if len(ws.Config.CopyFiles) > 0 {
		fmt.Fprintf(w, "copyFiles:     %s\n", strings.Join(ws.Config.CopyFiles, ", "))
	}
	fmt.Fprintf(w, "editor:        %s\n", ws.Config.Editor)

	if len(ws.Config.Setup) == 0 {
		fmt.Fprintf(w, "setup:         (none)\n")
		return
	}
	fmt.Fprintf(w, "setup:\n")
	for _, repo := range sortedKeys(ws.Config.Setup) {
		fmt.Fprintf(w, "  %s:\n", repo)
		for _, c := range ws.Config.Setup[repo] {
			fmt.Fprintf(w, "    $ %s\n", c)
		}
	}
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// configValue returns the (resolved) value for a config key.
func configValue(ws config.Workspace, key string) (string, error) {
	switch key {
	case "owner":
		return ws.Config.Owner, nil
	case "worktreeRoot":
		return ws.Config.ResolveWorktreeRoot(ws.Root), nil
	case "copyEnvFiles":
		return strconv.FormatBool(ws.Config.CopyEnvFiles), nil
	case "editor":
		return ws.Config.Editor, nil
	default:
		return "", fmt.Errorf("unknown config key %q (valid: %s)", key, configKeys)
	}
}

// setConfigValue returns a copy of cfg with key set to value. The caller is
// responsible for persisting it (Save also re-validates).
func setConfigValue(cfg config.Config, key, value string) (config.Config, error) {
	switch key {
	case "owner":
		cfg.Owner = value
	case "worktreeRoot":
		// Stored raw; an empty value resets to the default <workspace>/worktrees.
		cfg.WorktreeRoot = value
	case "copyEnvFiles":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return config.Config{}, fmt.Errorf("copyEnvFiles must be true or false, got %q", value)
		}
		cfg.CopyEnvFiles = b
	case "editor":
		cfg.Editor = value
	default:
		return config.Config{}, fmt.Errorf("unknown config key %q (valid: %s)", key, configKeys)
	}
	return cfg, nil
}
