package upgrade

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const latestReleaseDownloadBase = "https://github.com/XMRayLabs/Frp-Manager/releases/latest/download"

var semanticVersionPattern = regexp.MustCompile(`^[vV]?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$`)

type ReleaseInfo struct {
	TagName string
	Version string
}

func CheckForUpdate(ctx context.Context, currentVersion string, clientOnly bool, httpProxy string) (ReleaseInfo, bool, error) {
	asset, err := detectAssetName(clientOnly)
	if err != nil {
		return ReleaseInfo{}, false, err
	}
	return checkForUpdateAt(ctx, currentVersion, latestReleaseDownloadBase+"/"+asset, httpProxy)
}

func checkForUpdateAt(ctx context.Context, currentVersion, latestURL, httpProxy string) (ReleaseInfo, bool, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if strings.TrimSpace(httpProxy) != "" {
		proxyURL, err := url.Parse(strings.TrimSpace(httpProxy))
		if err != nil {
			return ReleaseInfo{}, false, fmt.Errorf("parse update proxy: %w", err)
		}
		if proxyURL.Scheme != "http" && proxyURL.Scheme != "https" {
			return ReleaseInfo{}, false, fmt.Errorf("update proxy must use http or https")
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, latestURL, nil)
	if err != nil {
		return ReleaseInfo{}, false, err
	}
	req.Header.Set("User-Agent", "frp-manager-update-check")

	resp, err := client.Do(req)
	if err != nil {
		return ReleaseInfo{}, false, fmt.Errorf("check latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return ReleaseInfo{}, false, fmt.Errorf("latest release returned status %d", resp.StatusCode)
	}

	tagName, err := releaseTagFromLocation(resp.Header.Get("Location"))
	if err != nil {
		return ReleaseInfo{}, false, err
	}
	latest, err := parseSemanticVersion(tagName)
	if err != nil {
		return ReleaseInfo{}, false, fmt.Errorf("parse latest version: %w", err)
	}
	current, err := parseSemanticVersion(currentVersion)
	if err != nil {
		if strings.EqualFold(strings.TrimSpace(currentVersion), "alpha") {
			current = semanticVersion{prerelease: "alpha"}
		} else {
			return ReleaseInfo{}, false, fmt.Errorf("parse current version: %w", err)
		}
	}

	info := ReleaseInfo{TagName: tagName, Version: latest.normalized()}
	return info, latest.compare(current) > 0, nil
}

func releaseTagFromLocation(rawLocation string) (string, error) {
	location, err := url.Parse(strings.TrimSpace(rawLocation))
	if err != nil {
		return "", fmt.Errorf("parse latest release redirect: %w", err)
	}
	if location.Scheme != "https" || !strings.EqualFold(location.Hostname(), "github.com") {
		return "", fmt.Errorf("latest release redirect must target https://github.com")
	}
	parts := strings.Split(strings.Trim(location.Path, "/"), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "releases" && parts[i+1] == "download" {
			tag := parts[i+2]
			if _, err := parseSemanticVersion(tag); err != nil {
				return "", err
			}
			return tag, nil
		}
	}
	return "", fmt.Errorf("latest release redirect does not contain a version tag")
}

type semanticVersion struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func parseSemanticVersion(raw string) (semanticVersion, error) {
	match := semanticVersionPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if match == nil {
		return semanticVersion{}, fmt.Errorf("invalid semantic version %q", raw)
	}
	values := make([]int, 3)
	for i := range values {
		value, err := strconv.Atoi(match[i+1])
		if err != nil {
			return semanticVersion{}, err
		}
		values[i] = value
	}
	return semanticVersion{major: values[0], minor: values[1], patch: values[2], prerelease: match[4]}, nil
}

func (v semanticVersion) normalized() string {
	version := fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
	if v.prerelease != "" {
		version += "-" + v.prerelease
	}
	return version
}

func (v semanticVersion) compare(other semanticVersion) int {
	left := []int{v.major, v.minor, v.patch}
	right := []int{other.major, other.minor, other.patch}
	for i := range left {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	if v.prerelease == other.prerelease {
		return 0
	}
	if v.prerelease == "" {
		return 1
	}
	if other.prerelease == "" {
		return -1
	}
	return strings.Compare(v.prerelease, other.prerelease)
}
