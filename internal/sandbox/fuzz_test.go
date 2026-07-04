package sandbox

import (
	"path/filepath"
	"strings"
	"testing"
)

func FuzzSafeWritePath(f *testing.F) {
	f.Add("test.txt", "aaaa")
	f.Add("../evil", "aaaa")
	f.Add("/etc/passwd", "aaaa")
	f.Add("foo/../../../etc/cron", "aaaa")
	f.Add("", "aaaa")
	f.Add("deep/sub/dir/file.c", "aaaa")

	f.Fuzz(func(t *testing.T, name, content string) {
		if len(name) > 1024 || len(content) > 1024*1024 {
			return
		}

		dir := t.TempDir()
		p, err := safeWritePath(dir, name)
		if err != nil {
			// Rejection is expected for path traversal.
			return
		}

		cleanDir := filepath.Clean(dir) + string(filepath.Separator)
		if !strings.HasPrefix(filepath.Clean(p), cleanDir) {
			t.Errorf("safeWritePath returned path outside dir: %q not in %q", p, dir)
		}
	})
}
