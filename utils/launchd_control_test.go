package utils

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type launchExit int

func (e launchExit) Error() string { return fmt.Sprint(int(e)) }
func (e launchExit) ExitCode() int { return int(e) }
func TestLaunchdTransitions(t *testing.T) {
	for _, tc := range []struct {
		name, action    string
		loaded, running bool
		want            []string
	}{
		{"repeat start", "start", true, true, []string{}},
		{"cold start", "start", false, false, []string{"enable system/frpp", "bootstrap system /Library/LaunchDaemons/frpp.plist"}},
		{"loaded stopped", "start", true, false, []string{"kickstart system/frpp"}},
		{"restart", "restart", true, true, []string{"kickstart -k system/frpp"}},
		{"stop", "stop", true, true, []string{"bootout system/frpp"}},
		{"already stopped", "stop", false, false, []string{}},
		{"uninstall", "uninstall", true, true, []string{"bootout system/frpp"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loaded, running := tc.loaded, tc.running
			calls := []string{}
			run := func(args ...string) (string, error) {
				if args[0] == "print" {
					if !loaded {
						return "not found", launchExit(113)
					}
					if running {
						return "state = running\n pid = 123\n", nil
					}
					return "state = waiting\n", nil
				}
				calls = append(calls, strings.Join(args, " "))
				switch args[0] {
				case "bootstrap", "kickstart":
					loaded = true
					running = true
				case "bootout":
					loaded = false
					running = false
				}
				return "", nil
			}
			if err := controlLaunchd("frpp", tc.action, run, func() {}); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(calls, tc.want) {
				t.Fatalf("got %v want %v", calls, tc.want)
			}
		})
	}
}
func TestLaunchdRejectsRealFailureAndNoRunningProcess(t *testing.T) {
	for _, mode := range []string{"permission", "bootstrap", "never-running", "bootout"} {
		t.Run(mode, func(t *testing.T) {
			action := "start"
			if mode == "bootout" {
				action = "stop"
			}
			run := func(args ...string) (string, error) {
				switch args[0] {
				case "print":
					if mode == "permission" {
						return "permission denied", launchExit(1)
					}
					if mode == "bootstrap" {
						return "absent", launchExit(113)
					}
					return "state = waiting\nlast exit code = 1", nil
				case "bootstrap", "bootout":
					return "Input/output error", launchExit(5)
				}
				return "", nil
			}
			if err := controlLaunchd("frpp", action, run, func() {}); err == nil {
				t.Fatal("failure reported success")
			}
		})
	}
}
func TestLaunchdWaitsForUnload(t *testing.T) {
	printed := 0
	pauses := 0
	run := func(args ...string) (string, error) {
		if args[0] == "print" {
			printed++
			if printed < 4 {
				return "pid = 123", nil
			}
			return "absent", launchExit(113)
		}
		return "", nil
	}
	if err := controlLaunchd("frpp", "stop", run, func() { pauses++ }); err != nil {
		t.Fatal(err)
	}
	if pauses != 2 {
		t.Fatalf("did not wait for unload: %d", pauses)
	}
}
