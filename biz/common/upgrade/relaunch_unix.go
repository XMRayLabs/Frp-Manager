//go:build !windows

package upgrade

import (
	"os"
	"syscall"
)

func relaunchCurrent(executable string, args []string) error {
	argv := append([]string{executable}, args...)
	return syscall.Exec(executable, argv, os.Environ())
}
