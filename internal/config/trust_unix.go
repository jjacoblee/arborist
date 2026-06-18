//go:build unix

package config

import (
	"fmt"
	"os"
	"syscall"
)

// checkTrust rejects a config file that is writable by group or others, or that
// is not owned by the current user — a "dubious ownership" check in the spirit
// of git's safe.directory and ssh's key-permission checks. Arborist discovers
// .arborist.json automatically by walking up from the current directory, so a
// file another user could write would otherwise be trusted to redirect file
// operations and launch the configured editor.
func checkTrust(path string, info os.FileInfo) error {
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%w: %s is writable by group or others (run: chmod go-w %q)",
			ErrUntrustedConfig, path, path)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil // platform detail unavailable; the permission check above still applied
	}
	if int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%w: %s is not owned by the current user", ErrUntrustedConfig, path)
	}
	return nil
}
