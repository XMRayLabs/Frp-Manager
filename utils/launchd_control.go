package utils

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type launchdRunner func(...string) (string, error)

var launchdPID = regexp.MustCompile("(?m)^[ \t]*pid[ \t]*=[ \t]*[1-9][0-9]*[ \t]*$")

func runLaunchctl(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/bin/launchctl", args...).CombinedOutput()
	if ctx.Err() != nil {
		return string(output), ctx.Err()
	}
	return string(output), err
}

// Use explicit system targets; loading an already registered job is not a start operation.
func controlLaunchd(name, action string, run launchdRunner, pause func()) error {
	target := "system/" + name
	plist := "/Library/LaunchDaemons/" + name + ".plist"
	invoke := func(args ...string) error {
		out, err := run(args...)
		if err != nil {
			return fmt.Errorf("launchctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(out))
		}
		return nil
	}
	state := func() (string, bool, error) {
		out, err := run("print", target)
		if err == nil {
			return out, true, nil
		}
		// launchctl print returns ESRCH (113) for a job absent from the specified domain.
		if e, ok := err.(interface{ ExitCode() int }); ok && e.ExitCode() == 113 {
			return out, false, nil
		}
		return out, false, fmt.Errorf("cannot inspect %s: %w: %s", target, err, strings.TrimSpace(out))
	}
	if action == "stop" || action == "uninstall" {
		_, loaded, err := state()
		if err != nil {
			return err
		}
		if !loaded {
			return nil
		}
		if err := invoke("bootout", target); err != nil {
			return err
		}
		for i := 0; i < 20; i++ {
			_, loaded, err = state()
			if err != nil {
				return err
			}
			if !loaded {
				return nil
			}
			pause()
		}
		return fmt.Errorf("%s did not finish unloading", target)
	}
	out, loaded, err := state()
	if err != nil {
		return err
	}
	if !loaded {
		if err := invoke("enable", target); err != nil {
			return err
		}
		if err := invoke("bootstrap", "system", plist); err != nil {
			return err
		}
	} else if action == "restart" {
		if err := invoke("kickstart", "-k", target); err != nil {
			return err
		}
	} else if !launchdPID.MatchString(out) {
		if err := invoke("kickstart", target); err != nil {
			return err
		}
	}
	// Require a PID in two successive samples, rather than only a successful load.
	stable := 0
	for i := 0; i < 20; i++ {
		out, loaded, err = state()
		if err != nil {
			return err
		}
		if loaded && launchdPID.MatchString(out) {
			stable++
		} else {
			stable = 0
		}
		if stable >= 2 {
			return nil
		}
		pause()
	}
	return fmt.Errorf("%s did not stay running; inspect /var/log/%s.err.log and /var/log/%s.out.log", target, name, name)
}
