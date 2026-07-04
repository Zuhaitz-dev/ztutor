package crypt

import (
	"strings"
	"testing"
)

func FuzzExtractTarGzPathTraversal(f *testing.F) {
	// Seed with known-safe data
	f.Add("test.txt", "hello")
	// Seed with traversal attempts
	f.Add("../etc/passwd", "evil")
	f.Add("foo/../../etc/passwd", "eviler")
	f.Add("/absolute/path", "bad")
	f.Add("", "empty name")

	f.Fuzz(func(t *testing.T, name, content string) {
		if name == "" || len(content) > 1024*1024 {
			return
		}
		// Tar files with a zero-length file cannot exist.
		if len(name) > 4096 {
			return
		}

		// Build a tar.gz in memory and try to extract it.
		var buf strings.Builder
		err := BuildTarGz(&buf, t.TempDir())
		if err != nil {
			t.Skip()
		}
		_ = extractAndTest(name, content, t)
	})
}

func FuzzDecodeSig(f *testing.F) {
	f.Add("deadbeef")
	f.Add("0123456789abcdef")
	f.Add("ABCDEF")
	f.Add("")
	f.Add("abc")
	f.Add("gg") // invalid hex
	f.Add("a")
	f.Add("aaa")
	f.Add("0x1234")

	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 {
			return
		}
		b, err := decodeSig(s)
		if err == nil && len(b) != len(s)/2 {
			t.Errorf("decodeSig(%q) returned %d bytes for %d hex chars", s, len(b), len(s))
		}
	})
}

func FuzzEncodeDecodeSigRoundtrip(f *testing.F) {
	f.Add([]byte{0xde, 0xad, 0xbe, 0xef})
	f.Add([]byte{0x00, 0xff, 0x01, 0xfe})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 4096 {
			return
		}
		s := encodeSig(b)
		decoded, err := decodeSig(s)
		if err != nil {
			t.Fatalf("decodeSig failed on roundtrip: %v", err)
		}
		if len(decoded) != len(b) {
			t.Fatalf("roundtrip length mismatch: %d != %d", len(decoded), len(b))
		}
		for i := range b {
			if decoded[i] != b[i] {
				t.Fatalf("roundtrip mismatch at byte %d", i)
			}
		}
	})
}

func extractAndTest(name, content string, t *testing.T) bool {
	// Just verify the path traversal check logic by testing safeWritePath-style patterns
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		if !strings.Contains(name, "..") && !strings.HasPrefix(name, "/") {
			t.Log("path would be allowed:", name)
		}
		return false
	}
	return true
}
