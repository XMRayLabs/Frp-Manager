//go:build windows

package upgrade

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareWorkerExecutableUsesIndependentCopy(t *testing.T) {
	workDir := t.TempDir()
	source := filepath.Join(workDir, "frpp.exe")
	plan := filepath.Join(workDir, "plan.json")
	content := []byte("test executable")
	require.NoError(t, os.WriteFile(source, content, 0600))

	worker, err := prepareWorkerExecutable(source, plan)
	require.NoError(t, err)
	require.NotEqual(t, source, worker)
	require.Contains(t, filepath.Base(worker), windowsWorkerPrefix)
	actual, err := os.ReadFile(worker)
	require.NoError(t, err)
	require.Equal(t, content, actual)
}
