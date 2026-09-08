package upgrade

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Sakurame1/frp-manager/utils"
)

func verifyOfficialChecksum(ctx context.Context, opt Options, assetName, downloadPath string) error {
	if strings.TrimSpace(opt.DownloadURL) != "" {
		return nil
	}

	checksumURL := releaseBaseURL(opt.Version) + "/SHA256SUMS-core.txt"
	if opt.UseGithubProxy && strings.TrimSpace(opt.GithubProxy) != "" {
		checksumURL = strings.TrimRight(strings.TrimSpace(opt.GithubProxy), "/") + "/" + checksumURL
	}
	manifestPath, err := utils.DownloadFile(ctx, checksumURL, strings.TrimSpace(opt.HTTPProxy))
	if err != nil {
		return fmt.Errorf("download checksum manifest: %w", err)
	}
	manifest, err := os.Open(manifestPath)
	if err != nil {
		return fmt.Errorf("open checksum manifest: %w", err)
	}
	defer manifest.Close()

	return verifyChecksumManifest(downloadPath, manifest, assetName)
}

func verifyChecksumManifest(downloadPath string, manifest io.Reader, assetName string) error {
	expected, err := checksumFromManifest(manifest, assetName)
	if err != nil {
		return err
	}
	actual, err := fileSHA256(downloadPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("SHA256 mismatch for %s", assetName)
	}
	return nil
}

func checksumFromManifest(manifest io.Reader, assetName string) (string, error) {
	expected := ""
	scanner := bufio.NewScanner(manifest)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == assetName {
			expected = strings.ToLower(fields[0])
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read checksum manifest: %w", err)
	}
	if len(expected) != sha256.Size*2 {
		return "", fmt.Errorf("checksum for %s was not found", assetName)
	}
	return expected, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open binary for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash binary: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
