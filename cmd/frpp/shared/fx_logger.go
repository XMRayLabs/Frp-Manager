package shared

import (
	"context"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

type startupErrorLogger struct{ report func(error) }

func (l startupErrorLogger) LogEvent(event fxevent.Event) {
	var err error
	switch e := event.(type) {
	case *fxevent.Invoked:
		err = e.Err
	case *fxevent.Started:
		err = e.Err
	case *fxevent.Stopped:
		err = e.Err
	case *fxevent.OnStartExecuted:
		err = e.Err
	case *fxevent.OnStopExecuted:
		err = e.Err
	}
	if err != nil {
		l.report(err)
	}
}
func nodeErrorLogging() fx.Option {
	return fx.WithLogger(func() fxevent.Logger {
		return startupErrorLogger{report: func(err error) {
			logger.Logger(context.Background()).Errorf("application startup/lifecycle failed: %v", err)
		}}
	})
}
