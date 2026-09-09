package shared

import (
	"context"
	"errors"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeConfigDirectoryFallback(t *testing.T) {
	missing := func() (string, error) { return "", errors.New("HOME is not defined") }
	root := t.TempDir()
	got, err := nodeConfigDir("darwin", missing, func() (string, error) { return root, nil })
	if err != nil || got != filepath.Join(root, "Library", "Application Support") {
		t.Fatal(got, err)
	}
	got, err = nodeConfigDir("darwin", func() (string, error) { return "existing", nil }, func() (string, error) { t.Fatal("unnecessary lookup"); return "", nil })
	if err != nil || got != "existing" {
		t.Fatal(got, err)
	}
	if _, err = nodeConfigDir("linux", missing, func() (string, error) { t.Fatal("unexpected lookup"); return "", nil }); err == nil {
		t.Fatal("masked configuration failure")
	}
}
func TestStartupFailureReportedWithoutDebug(t *testing.T) {
	for _, hook := range []bool{false, true} {
		var messages []string
		opts := []fx.Option{fx.WithLogger(func() fxevent.Logger {
			return startupErrorLogger{report: func(err error) { messages = append(messages, err.Error()) }}
		})}
		if hook {
			opts = append(opts, fx.Invoke(func(lc fx.Lifecycle) {
				lc.Append(fx.Hook{OnStart: func(context.Context) error { return errors.New("test startup failure") }})
			}))
		} else {
			opts = append(opts, fx.Invoke(func() error { return errors.New("test startup failure") }))
		}
		a := fx.New(opts...)
		if err := a.Start(context.Background()); err == nil {
			t.Fatal("expected failure")
		}
		if !strings.Contains(strings.Join(messages, " "), "test startup failure") {
			t.Fatal("startup error silently discarded")
		}
	}
}
