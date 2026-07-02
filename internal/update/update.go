// Package update checks GitHub releases for newer versions of ztutor and
// caches the result in the database so that the TUI can show a notification.
package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	repoOwner = "Zuhaitz-dev"
	repoName  = "ztutor"
	cacheKey  = "update_last_check"
	cacheTTL  = 24 * time.Hour
	userAgent = "ztutor-update-check/1.0"
)

var apiURL = "https://api.github.com/repos/Zuhaitz-dev/ztutor/releases/latest"

// LatestInfo holds information about the newest available release.
type LatestInfo struct {
	Version     string `json:"tag_name"`     // e.g. "v1.5.0"
	PublishedAt string `json:"published_at"` // ISO 8601
	ReleaseURL  string `json:"html_url"`     // GitHub URL
	ReleaseBody string `json:"body"`         // release notes
}

// Cache is the minimal interface needed to persist the last check time.
// *db.DB satisfies it via GetUserSetting/SetUserSetting.
type Cache interface {
	GetUserSetting(username, key string) (string, error)
	SetUserSetting(username, key, value string) error
}

// CheckLatest fetches the latest release from GitHub and returns it if it is
// newer than currentVersion. Results are cached in the db for cacheTTL to
// avoid hitting GitHub's rate limit on every startup.
// Returns nil when up to date, on error, or when currentVersion is "dev".
// When c is nil, the check always hits the network (no caching).
func CheckLatest(currentVersion string, c Cache, username string) (*LatestInfo, error) {
	if currentVersion == "dev" || strings.HasPrefix(currentVersion, "dev-") {
		return nil, nil
	}

	// Check cache.
	if c != nil {
		lastCheck, err := c.GetUserSetting(username, cacheKey)
		if err == nil && lastCheck != "" {
			ts, err := strconv.ParseInt(lastCheck, 10, 64)
			if err == nil && time.Since(time.Unix(ts, 0)) < cacheTTL {
				return nil, nil
			}
		}
	}

	info, err := fetchLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	// Update cache timestamp regardless of result.
	if c != nil {
		now := strconv.FormatInt(time.Now().Unix(), 10)
		if setErr := c.SetUserSetting(username, cacheKey, now); setErr != nil {
			return nil, fmt.Errorf("cache write: %w (fetch: %v)", setErr, err)
		}
	}

	currentTag := currentVersion
	if !strings.HasPrefix(currentVersion, "v") {
		currentTag = "v" + currentVersion
	}

	if semverCompare(info.Version, currentTag) <= 0 {
		return nil, nil
	}

	return info, nil
}

// fetchLatestRelease calls the GitHub API for the latest release.
func fetchLatestRelease() (*LatestInfo, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var info LatestInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	if info.Version == "" {
		return nil, fmt.Errorf("empty version in API response")
	}

	return &info, nil
}

// semverCompare compares two semantic version tags (e.g. "v1.2.3").
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func semverCompare(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")

	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")

	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}

	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(ap) {
			av, _ = strconv.Atoi(ap[i])
		}
		if i < len(bp) {
			bv, _ = strconv.Atoi(bp[i])
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}
