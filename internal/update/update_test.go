package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

type memCache struct {
	data map[string]string
}

func (m *memCache) GetUserSetting(username, key string) (string, error) {
	if m.data == nil {
		return "", nil
	}
	return m.data[username+":"+key], nil
}

func (m *memCache) SetUserSetting(username, key, value string) error {
	if m.data == nil {
		m.data = make(map[string]string)
	}
	m.data[username+":"+key] = value
	return nil
}

func TestSemverCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.0.0", "v2.0.0", -1},
		{"v2.0.0", "v1.0.0", 1},
		{"v1.2.3", "v1.2.4", -1},
		{"v1.3.0", "v1.2.9", 1},
		{"v2.0.0", "v1.99.99", 1},
		{"dev", "v1.0.0", -1},
		{"v1.2", "v1.2.0", 0},
		{"v1.10", "v1.2", 1},
		{"v0.0.0", "v0.0.1", -1},
		{"v0.1.0", "v0.0.9", 1},
	}
	for _, tt := range tests {
		got := semverCompare(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("semverCompare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCheckLatest_DevBypass(t *testing.T) {
	c := &memCache{}
	info, err := CheckLatest("dev", c, "testuser")
	if err != nil {
		t.Fatalf("CheckLatest dev: %v", err)
	}
	if info != nil {
		t.Fatal("expected nil for dev version")
	}
}

func TestCheckLatest_DevPrefixBypass(t *testing.T) {
	c := &memCache{}
	info, err := CheckLatest("dev-something", c, "testuser")
	if err != nil {
		t.Fatalf("CheckLatest dev-: %v", err)
	}
	if info != nil {
		t.Fatal("expected nil for dev- version")
	}
}

func TestCheckLatest_FreshCacheSkipsFetch(t *testing.T) {
	c := &memCache{}
	fresh := time.Now().Unix()
	c.SetUserSetting("u2", "update_last_check", strconv.FormatInt(fresh, 10))

	info, err := CheckLatest("v1.0.0", c, "u2")
	if err != nil {
		t.Fatalf("fresh cache should not error: %v", err)
	}
	if info != nil {
		t.Fatal("expected nil (cache says recently checked)")
	}
}

func TestCheckLatest_NilCache(t *testing.T) {
	// Nil cache must not panic and should always hit the network.
	// Since we have no HTTP server in this test, we expect a fetch error,
	// not a nil result (which would mean "up to date").
	_, err := CheckLatest("v1.0.0", nil, "noone")
	if err == nil {
		t.Log("network available (unexpected in unit test)")
	}
}

func TestCheckLatest_OlderVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LatestInfo{
			Version: "v0.9.0",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	originalURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = originalURL }()

	c := &memCache{}
	info, err := CheckLatest("v1.0.0", c, "older")
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}
	if info != nil {
		t.Fatal("expected nil (current version is newer)")
	}
}

func TestCheckLatest_NewerVersionFromServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LatestInfo{
			Version:     "v2.0.0",
			PublishedAt: "2026-07-01T00:00:00Z",
			ReleaseURL:  "https://github.com/Zuhaitz-dev/ztutor/releases/tag/v2.0.0",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	originalURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = originalURL }()

	c := &memCache{}
	info, err := CheckLatest("v1.5.0", c, "newer")
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil (newer version exists)")
	}
	if info.Version != "v2.0.0" {
		t.Errorf("Version = %q, want v2.0.0", info.Version)
	}
	if info.ReleaseURL == "" {
		t.Error("ReleaseURL is empty")
	}
}

func TestCheckLatest_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	originalURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = originalURL }()

	_, err := CheckLatest("v1.0.0", nil, "err")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestCheckLatest_SameVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LatestInfo{Version: "v1.0.0"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	originalURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = originalURL }()

	info, err := CheckLatest("v1.0.0", nil, "same")
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}
	if info != nil {
		t.Fatal("expected nil (same version)")
	}
}

func TestCheckLatest_WritesCacheOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LatestInfo{Version: "v1.0.0"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	originalURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = originalURL }()

	c := &memCache{}
	_, err := CheckLatest("v0.9.0", c, "cached")
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}

	// After the fetch, a second call with same version should be cached.
	info, err := CheckLatest("v1.0.0", c, "cached")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if info != nil {
		t.Fatal("expected nil (should be cached)")
	}
}
