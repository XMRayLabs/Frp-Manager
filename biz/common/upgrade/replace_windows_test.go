//go:build windows

package upgrade

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplaceStagedToTargetWithBackup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "frp-manager.exe")
	staged := stagePathForTarget(target)
	require.NoError(t, os.WriteFile(target, []byte("old"), 0600))
	require.NoError(t, os.WriteFile(staged, []byte("new"), 0600))

	require.NoError(t, replaceStagedToTarget(context.Background(), staged, target, true))
	current, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "new", string(current))
	backup, err := os.ReadFile(target + ".bak")
	require.NoError(t, err)
	require.Equal(t, "old", string(backup))
}
