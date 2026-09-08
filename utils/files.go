package utils

import (
	"compress/gzip"
	"context"
	crand "crypto/rand"
	"fmt"
	"io"
	"net"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"github.com/imroc/req/v3"
	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar/v3"
	"go.uber.org/multierr"
)

func EnsureDirectoryExists(filePath string) error {
	directory := filepath.Dir(filePath)

	if _, err := os.Stat(directory); os.IsNotExist(err) {
		err = os.MkdirAll(directory, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

func FindExecutableNames(filter func(name string) bool, extraPaths ...string) ([]string, error) {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil, fmt.Errorf("PATH environment variable is empty")
	}

	var results []string
	seen := make(map[string]struct{})
	var errs error

	pathToCheck := extraPaths
	pathToCheck = append(pathToCheck, filepath.SplitList(pathEnv)...)

	for _, dir := range pathToCheck {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			if _, dup := seen[name]; dup {
				continue
			}
			if !filter(name) {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				errs = multierr.Append(errs, err)
				continue
			}

			if info.IsDir() || info.Mode()&0111 == 0 {
				continue
			}

			results = append(results, path.Join(dir, name))
			seen[name] = struct{}{}
		}
	}

	if len(results) > 0 {
		return results, nil
	}
	if errs != nil {
		return nil, errs
	}
	return nil, nil
}

var TmpFileDir = path.Join(os.TempDir(), "sakura-frp-manager-download")

func DownloadFile(ctx context.Context, rawURL string, proxyUrl string) (string, error) {
	if err := validateDownloadURL(rawURL); err != nil {
		return "", err
	}
	if err := os.MkdirAll(TmpFileDir, 0700); err != nil {
		return "", err
	}

	tmpPath, err := os.MkdirTemp(TmpFileDir, "downloads")
	if err != nil {
		return "", err
	}

	tmpFileName := generateRandomFileName("download", ".tmp")
	fileFullPath := path.Join(tmpPath, tmpFileName)

	cli := req.C()
	if len(proxyUrl) > 0 {
		cli = cli.SetProxyURL(proxyUrl)
	}

	logger.Logger(ctx).Infof("Downloading file from url: %s with proxy: %s", rawURL, proxyUrl)

	showProgress := isatty.IsTerminal(os.Stderr.Fd())
	retryPolicy := retrypolicy.NewBuilder[any]().
		HandleIf(func(_ any, err error) bool { return isRetryableDownloadErr(err) }).
		WithMaxRetries(2).
		WithBackoff(500*time.Millisecond, 2*time.Second).
		Build()

	runAttempt := func() error {
		_ = os.Remove(fileFullPath)

		if showProgress {
			var (
				mu        sync.Mutex
				bar       *progressbar.ProgressBar
				lastBytes int64
			)
			callback := func(info req.DownloadInfo) {
				if info.Response == nil || info.Response.Response == nil {
					return
				}
				mu.Lock()
				defer mu.Unlock()

				if bar == nil {
					max := info.Response.ContentLength
					if max <= 0 {
						max = -1
					}
					bar = progressbar.NewOptions64(
						max,
						progressbar.OptionSetWriter(os.Stderr),
						progressbar.OptionShowBytes(true),
						progressbar.OptionSetWidth(24),
						progressbar.OptionSetDescription("Downloading..."),
						progressbar.OptionThrottle(200*time.Millisecond),
						progressbar.OptionClearOnFinish(),
					)
				}

				delta := info.DownloadedSize - lastBytes
				if delta > 0 {
					_ = bar.Add64(delta)
					lastBytes = info.DownloadedSize
				}
			}

			resp, err := cli.R().
				SetContext(ctx).
				SetOutputFile(fileFullPath).
				SetDownloadCallbackWithInterval(callback, 200*time.Millisecond).
				SetRetryCount(0).
				Get(rawURL)

			if bar != nil {
				_ = bar.Finish()
				_, _ = fmt.Fprintln(os.Stderr)
			}
			if err != nil {
				return err
			}
			if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
				return fmt.Errorf("unexpected download status: %d", resp.StatusCode)
			}
			return validateDownloadedFileSize(fileFullPath)
		}

		resp, err := cli.R().
			SetContext(ctx).
			SetOutputFile(fileFullPath).
			SetRetryCount(0).
			Get(rawURL)
		if err != nil {
			return err
		}
		if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
			return fmt.Errorf("unexpected download status: %d", resp.StatusCode)
		}

		return validateDownloadedFileSize(fileFullPath)
	}

	if err := failsafe.With(retryPolicy).Run(runAttempt); err != nil {
		logger.Logger(ctx).WithError(err).Error("download file from url error")
		return "", err
	}
	return fileFullPath, nil
}

func isRetryableDownloadErr(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return true
	}
	if ne, ok := err.(net.Error); ok {
		if ne.Timeout() || ne.Temporary() {
			return true
		}
	}
	msg := err.Error()
	switch {
	case msg == "unexpected EOF":
		return true
	case containsAny(msg,
		"connection reset by peer",
		"use of closed network connection",
		"TLS handshake timeout",
		"i/o timeout",
		"timeout",
		"temporary failure",
		"unexpected download status:",
	):
		return true
	default:
		return false
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) == 0 {
			continue
		}
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func generateRandomFileName(prefix, extension string) string {
	randomStr := randomString(8)
	fileName := fmt.Sprintf("%s_%s%s", prefix, randomStr, extension)
	return fileName
}

func randomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	if _, err := crand.Read(bytes); err != nil {
		now := time.Now().UnixNano()
		for i := range bytes {
			bytes[i] = charset[(int(now)+i)%len(charset)]
		}
		return string(bytes)
	}
	for i, b := range bytes {
		bytes[i] = charset[int(b)%len(charset)]
	}

	return string(bytes)
}

func validateDownloadURL(rawURL string) error {
	u, err := neturl.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid download url: %w", err)
	}
	if u.Host == "" {
		return fmt.Errorf("download url host is empty")
	}
	if u.User != nil {
		return fmt.Errorf("download url must not contain credentials")
	}

	host := u.Hostname()
	isLocalhost := strings.EqualFold(host, "localhost")
	if ip := net.ParseIP(host); ip != nil {
		isLocalhost = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && isLocalhost) {
		return fmt.Errorf("download url must use https (http is only allowed for localhost)")
	}
	return nil
}

func validateDownloadedFileSize(fileFullPath string) error {
	const maxExpectedSizeBytes = 1024 * 1024 * 1024 // 1 GiB
	st, err := os.Stat(fileFullPath)
	if err != nil {
		return err
	}
	if st.Size() > maxExpectedSizeBytes {
		return fmt.Errorf("downloaded file too large (%d bytes > %d bytes), refusing to proceed; please check DownloadURL/GithubProxy",
			st.Size(), int64(maxExpectedSizeBytes))
	}
	return nil
}

// ExtractGZTo decompresses the srcGZ file into a temporary directory,
// renames the extracted file to newName, moves it to destDir, and sets executable permissions (0755).
// It returns the full path of the final file on success.
func ExtractGZTo(srcGZ, newName, destDir string) (string, error) {
	if newName != filepath.Base(newName) {
		return "", fmt.Errorf("invalid output file name %q", newName)
	}

	f, err := os.Open(srcGZ)
	if err != nil {
		return "", fmt.Errorf("failed to open source gzip file %q: %w", srcGZ, err)
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader for %q: %w", srcGZ, err)
	}
	defer zr.Close()

	tmpDir, err := os.MkdirTemp("", "sakura-frp-manager-gz_extract_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	tmpFilePath := filepath.Join(tmpDir, newName)
	outFile, err := os.Create(tmpFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file %q: %w", tmpFilePath, err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, zr); err != nil {
		return "", fmt.Errorf("failed to write decompressed data to %q: %w", tmpFilePath, err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory %q: %w", destDir, err)
	}

	finalPath := filepath.Join(destDir, newName)
	if err := os.Rename(tmpFilePath, finalPath); err != nil {
		return "", fmt.Errorf("failed to move file to %q: %w", finalPath, err)
	}

	if err := os.Chmod(finalPath, 0755); err != nil {
		return "", fmt.Errorf("failed to set executable permission on %q: %w", finalPath, err)
	}

	return finalPath, nil
}
