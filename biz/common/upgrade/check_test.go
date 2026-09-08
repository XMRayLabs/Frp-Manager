package upgrade

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckForUpdateAt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://github.com/XMRayLabs/Frp-Manager/releases/download/v1.2.3/frp-manager-linux-amd64")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	info, available, err := checkForUpdateAt(context.Background(), "1.2.2", server.URL, "")
	require.NoError(t, err)
	require.True(t, available)
	require.Equal(t, "v1.2.3", info.TagName)
	require.Equal(t, "1.2.3", info.Version)

	_, available, err = checkForUpdateAt(context.Background(), "1.2.3", server.URL, "")
	require.NoError(t, err)
	require.False(t, available)

	_, available, err = checkForUpdateAt(context.Background(), "alpha", server.URL, "")
	require.NoError(t, err)
	require.True(t, available)
}

func TestSemanticVersionComparison(t *testing.T) {
	tests := []struct {
		left, right string
		expected    int
	}{
		{"1.0.1", "1.0.0", 1},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.0-alpha", 1},
		{"1.0.0-alpha", "1.0.0", -1},
	}
	for _, test := range tests {
		left, err := parseSemanticVersion(test.left)
		require.NoError(t, err)
		right, err := parseSemanticVersion(test.right)
		require.NoError(t, err)
		require.Equal(t, test.expected, left.compare(right))
	}
}

func TestVerifyChecksumManifest(t *testing.T) {
	content := []byte("frp-manager update")
	path := t.TempDir() + "/frp-manager"
	require.NoError(t, os.WriteFile(path, content, 0600))
	sum := sha256.Sum256(content)
	manifest := fmt.Sprintf("%x  frp-manager-linux-amd64\n", sum)

	require.NoError(t, verifyChecksumManifest(path, strings.NewReader(manifest), "frp-manager-linux-amd64"))
	require.Error(t, verifyChecksumManifest(path, strings.NewReader(manifest), "frp-manager-linux-arm64"))
	actual, err := fileSHA256(path)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%x", sum), actual)
}

func TestBuildDownloadURLUsesLatestAndBinaryType(t *testing.T) {
	fullAsset, err := detectAssetName(false)
	require.NoError(t, err)
	clientAsset, err := detectAssetName(true)
	require.NoError(t, err)

	fullURL, err := buildDownloadURL(Options{Version: "latest"})
	require.NoError(t, err)
	require.Equal(t, latestReleaseDownloadBase+"/"+fullAsset, fullURL)

	clientURL, err := buildDownloadURL(Options{Version: "v1.2.3", ClientOnly: true})
	require.NoError(t, err)
	require.Equal(t, "https://github.com/XMRayLabs/Frp-Manager/releases/download/v1.2.3/"+clientAsset, clientURL)
}
