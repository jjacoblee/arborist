//go:build !unix

package config

import (
	"fmt"
	"os"
)

// checkTrust applies the portable part of the trust check (rejecting files
// writable by group or others). Owner verification is Unix-specific and handled
// in trust_unix.go; on other platforms the permission check is the backstop.
func checkTrust(path string, info os.FileInfo) error {
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%w: %s is writable by group or others", ErrUntrustedConfig, path)
	}
	return nil
}
