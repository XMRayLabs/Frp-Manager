package logger

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestConfigureFileOutputWritesPlainRotatingLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	require.NoError(t, os.WriteFile(path, make([]byte, maxLogFileSize), 0600))

	configureFileOutput(path)
	Logger(context.Background()).Info("service diagnostic")

	t.Cleanup(func() {
		logFileMu.Lock()
		if logFile != nil {
			_ = logFile.Close()
			logFile = nil
		}
		Instance().SetOutput(os.Stderr)
		logrus.SetOutput(os.Stderr)
		logFileMu.Unlock()
	})

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), "service diagnostic")
	require.NotContains(t, string(content), "\x1b[")
	_, err = os.Stat(path + ".1")
	require.NoError(t, err)
}
