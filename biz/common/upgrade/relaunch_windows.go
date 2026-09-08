//go:build windows

package upgrade

import "fmt"

func relaunchCurrent(_ string, _ []string) error {
	return fmt.Errorf("Windows relaunch is handled by the upgrade worker")
}
