package license

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type keyFile struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

func devPublicKey(t *testing.T) []byte {
	t.Helper()
	root := findRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "dev_license_keys.json"))
	if err != nil {
		t.Fatalf("read dev_license_keys.json: %v", err)
	}
	var kf keyFile
	if err := json.Unmarshal(data, &kf); err != nil {
		t.Fatalf("parse dev_license_keys.json: %v", err)
	}
	b, err := hex.DecodeString(kf.PublicKey)
	if err != nil {
		t.Fatalf("decode dev public key: %v", err)
	}
	return b
}

func TestLicensesInRepo_VerifyWithDevKey(t *testing.T) {
	pub := devPublicKey(t)
	SetPublicKey(pub)

	repoRoot := findRepoRoot(t)
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		t.Fatalf("ReadDir repo root: %v", err)
	}

	var keyFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".key") {
			keyFiles = append(keyFiles, filepath.Join(repoRoot, e.Name()))
		}
	}
	if len(keyFiles) == 0 {
		t.Fatal("no .key files found in repo root")
	}

	for _, kf := range keyFiles {
		t.Run(filepath.Base(kf), func(t *testing.T) {
			state, err := Check(kf)
			if err != nil {
				t.Fatalf("Check(%s): %v", kf, err)
			}
			if !state.Licensed {
				t.Error("State.Licensed = false, want true")
			}
			t.Logf("  licensee=%q licensed=%v courses=%v features=%v expires=%v",
				state.Licensee, state.Licensed, state.UnlockedCourses, state.HasMultiUser, state.ExpiresAt)
		})
	}
}

func TestLicenseKey_UnlockedCourses(t *testing.T) {
	pub := devPublicKey(t)
	SetPublicKey(pub)

	repoRoot := findRepoRoot(t)
	entries, _ := os.ReadDir(repoRoot)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".key") {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			SetPublicKey(pub)
			state, err := Check(filepath.Join(repoRoot, e.Name()))
			if err != nil {
				t.Skipf("skip (verification failed: %v)", err)
			}
			if len(state.UnlockedCourses) == 0 {
				t.Log("no unlocked courses (open license)")
			}
			for _, cid := range state.UnlockedCourses {
				if cid == "*" {
					t.Log("wildcard: all courses unlocked")
					continue
				}
				if !state.CanAccessCourse(cid) {
					t.Errorf("CanAccessCourse(%q) = false but %q is in UnlockedCourses", cid, cid)
				}
			}
		})
	}
}

func TestLicenseKey_CourseKeyDecodes(t *testing.T) {
	pub := devPublicKey(t)
	SetPublicKey(pub)

	repoRoot := findRepoRoot(t)
	entries, _ := os.ReadDir(repoRoot)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".key") {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			SetPublicKey(pub)
			state, err := Check(filepath.Join(repoRoot, e.Name()))
			if err != nil {
				t.Skipf("skip (verification failed: %v)", err)
			}
			if state.CourseKey == nil {
				t.Log("no course key")
				return
			}
			if len(state.CourseKey) != 32 {
				t.Errorf("CourseKey length = %d, want 32 bytes", len(state.CourseKey))
			}
		})
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}
