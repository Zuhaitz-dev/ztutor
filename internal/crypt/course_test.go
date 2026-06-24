package crypt

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryCourse_WriteReadRoundTrip(t *testing.T) {
	// 1. Create publisher keypair.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// 2. Create CourseManifest.
	m := CourseManifest{
		CourseID: "course-complete-test",
		Version:  "2.1.0",
		Title:    "Complete Round-Trip Test",
		Language: "en",
	}

	// 3. Sign manifest.
	manifestJSON, sig, err := SignManifest(m, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	// 4. Create temp test directory with course files.
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "course.yaml"), []byte("title: Complete Course\ndescription: Test\n"), 0644); err != nil {
		t.Fatalf("write course.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "lesson.md"), []byte("# Lesson 1\n\nWelcome to the course.\n"), 0644); err != nil {
		t.Fatalf("write lesson.md: %v", err)
	}

	// 5. Build tar.gz of the test directory.
	var tarBuf bytes.Buffer
	if err := BuildTarGz(&tarBuf, srcDir); err != nil {
		t.Fatalf("BuildTarGz: %v", err)
	}

	// 6. Generate random AES key.
	var aesKey [32]byte
	if _, err := rand.Read(aesKey[:]); err != nil {
		t.Fatalf("generate AES key: %v", err)
	}

	// 7. Encrypt the tar.gz.
	encrypted, err := Encrypt(tarBuf.Bytes(), aesKey[:], m.CourseID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// 8. Create the .course file.
	coursePath := filepath.Join(t.TempDir(), "test.course")
	bc, err := Create(coursePath, manifestJSON)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 9. WritePayload.
	if err := bc.WritePayload(encrypted); err != nil {
		bc.Close()
		t.Fatalf("WritePayload: %v", err)
	}

	// 10. Close.
	if err := bc.Close(); err != nil {
		t.Fatalf("Close (write): %v", err)
	}

	// 11. Re-open.
	bc2, err := Open(coursePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer bc2.Close()

	// 12. Verify manifest matches.
	if !bytes.Equal(bc2.Manifest(), manifestJSON) {
		t.Fatalf("manifest mismatch:\n  got:  %s\n  want: %s", bc2.Manifest(), manifestJSON)
	}

	// 13. DecodeManifest to verify signature.
	decodedManifest, err := bc2.DecodeManifest(pub, sig)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if decodedManifest.CourseID != m.CourseID {
		t.Fatalf("CourseID mismatch: got %q, want %q", decodedManifest.CourseID, m.CourseID)
	}

	// 14. ReadPayload with the AES key.
	decrypted, err := bc2.ReadPayload(aesKey, m.CourseID)
	if err != nil {
		t.Fatalf("ReadPayload: %v", err)
	}

	// 15. ExtractTarGz the decrypted payload.
	extractDir := t.TempDir()
	if err := ExtractTarGz(bytes.NewReader(decrypted), extractDir); err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}

	// 16. Verify extracted files.
	if _, err := os.Stat(filepath.Join(extractDir, "course.yaml")); err != nil {
		t.Fatalf("course.yaml not found in extracted output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(extractDir, "lesson.md")); err != nil {
		t.Fatalf("lesson.md not found in extracted output: %v", err)
	}

	// 17. Verify content of extracted files.
	courseYAML, err := os.ReadFile(filepath.Join(extractDir, "course.yaml"))
	if err != nil {
		t.Fatalf("read extracted course.yaml: %v", err)
	}
	if !strings.Contains(string(courseYAML), "title: Complete Course") {
		t.Fatalf("unexpected course.yaml content: %s", courseYAML)
	}
	lessonMD, err := os.ReadFile(filepath.Join(extractDir, "lesson.md"))
	if err != nil {
		t.Fatalf("read extracted lesson.md: %v", err)
	}
	if !strings.Contains(string(lessonMD), "# Lesson 1") {
		t.Fatalf("unexpected lesson.md content: %s", lessonMD)
	}
}

func TestBinaryCourse_BadMagic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.course")

	// Write a file with wrong magic bytes.
	if err := os.WriteFile(path, []byte("NOTZ\x00\x00\x00\x00\x00\x00\x00\x00"), 0644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}

	_, err := Open(path)
	if err == nil {
		t.Fatal("expected error for bad magic bytes")
	}
	if !strings.Contains(err.Error(), "bad magic") {
		t.Fatalf("expected 'bad magic' error, got: %v", err)
	}
}

func TestBinaryCourse_ExtractTarGz(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "hello.txt"), []byte("hello world\n"), 0644); err != nil {
		t.Fatalf("write hello.txt: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "nested.txt"), []byte("nested file\n"), 0644); err != nil {
		t.Fatalf("write nested.txt: %v", err)
	}

	var buf bytes.Buffer
	if err := BuildTarGz(&buf, srcDir); err != nil {
		t.Fatalf("BuildTarGz: %v", err)
	}

	extractDir := t.TempDir()
	if err := ExtractTarGz(&buf, extractDir); err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}

	// Verify files exist.
	checkFile := func(name, expectedContent string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(extractDir, name))
		if err != nil {
			t.Fatalf("file %s not found: %v", name, err)
		}
		if string(data) != expectedContent {
			t.Fatalf("file %s content mismatch: got %q, want %q", name, data, expectedContent)
		}
	}
	checkFile("hello.txt", "hello world\n")
	checkFile(filepath.Join("subdir", "nested.txt"), "nested file\n")
}

func TestBinaryCourse_BuildAndExtract(t *testing.T) {
	srcDir := t.TempDir()

	// Create a deeper directory structure.
	dirs := []string{"a", "a/b", "a/b/c"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(srcDir, d), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	files := map[string]string{
		"course.yaml":    "title: Deep Test\n",
		"a/readme.md":    "# Section A\n",
		"a/b/info.txt":   "info B\n",
		"a/b/c/data.bin": string(make([]byte, 1024)),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	var buf bytes.Buffer
	if err := BuildTarGz(&buf, srcDir); err != nil {
		t.Fatalf("BuildTarGz: %v", err)
	}

	extractDir := t.TempDir()
	if err := ExtractTarGz(&buf, extractDir); err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}

	for name, expectedContent := range files {
		data, err := os.ReadFile(filepath.Join(extractDir, name))
		if err != nil {
			t.Fatalf("file %s not found: %v", name, err)
		}
		if string(data) != expectedContent {
			t.Fatalf("file %s content mismatch", name)
		}
	}
}

func TestBinaryCourse_PathTraversal(t *testing.T) {
	var malBuf bytes.Buffer
	gz := gzip.NewWriter(&malBuf)
	tw := tar.NewWriter(gz)

	// Write a malicious entry that tries to escape the target directory.
	hdr := &tar.Header{
		Name:     "../etc/passwd",
		Mode:     0644,
		Size:     int64(len("malicious content")),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write([]byte("malicious content")); err != nil {
		t.Fatalf("write tar body: %v", err)
	}

	// Write a benign entry to test that it doesn't interfere.
	benign := &tar.Header{
		Name:     "safe-file.txt",
		Mode:     0644,
		Size:     int64(len("safe")),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(benign); err != nil {
		t.Fatalf("write benign header: %v", err)
	}
	if _, err := tw.Write([]byte("safe")); err != nil {
		t.Fatalf("write benign body: %v", err)
	}

	tw.Close()
	gz.Close()

	extractDir := t.TempDir()
	err := ExtractTarGz(&malBuf, extractDir)
	if err == nil {
		t.Fatal("expected path traversal rejection error")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("expected 'path traversal' error, got: %v", err)
	}

	// Verify the safe file was NOT extracted (extraction stopped on first error).
	if _, err := os.Stat(filepath.Join(extractDir, "safe-file.txt")); err == nil {
		t.Fatal("safe-file.txt should not have been extracted after traversal rejection")
	}
}

func TestBinaryCourse_CompressionFlag(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	m := CourseManifest{
		CourseID: "course-compress",
		Version:  "1.0.0",
		Title:    "Compression Test",
		Language: "en",
	}

	manifestJSON, _, err := SignManifest(m, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	coursePath := filepath.Join(t.TempDir(), "compressed.course")
	bc, err := Create(coursePath, manifestJSON)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer bc.Close()

	if !bc.IsCompressed() {
		t.Fatal("expected IsCompressed() to be true after Create")
	}

	if bc.Version() != 1 {
		t.Fatalf("expected version 1, got %d", bc.Version())
	}
}

func TestBinaryCourse_OpenNonExistent(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "does-not-exist.course"))
	if err == nil {
		t.Fatal("expected error opening non-existent file")
	}
}

func TestBinaryCourse_ReadPayloadWrongKey(t *testing.T) {
	// Create a .course file with encrypted payload, then try to read with wrong key.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	_ = pub

	m := CourseManifest{
		CourseID: "course-wrong-key",
		Version:  "1.0.0",
		Title:    "Wrong Key Test",
		Language: "en",
	}

	manifestJSON, _, err := SignManifest(m, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("secret"), 0644)

	var tarBuf bytes.Buffer
	if err := BuildTarGz(&tarBuf, srcDir); err != nil {
		t.Fatalf("BuildTarGz: %v", err)
	}

	var aesKey [32]byte
	rand.Read(aesKey[:])

	encrypted, err := Encrypt(tarBuf.Bytes(), aesKey[:], m.CourseID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	coursePath := filepath.Join(t.TempDir(), "wrongkey.course")
	bc, err := Create(coursePath, manifestJSON)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	bc.WritePayload(encrypted)
	bc.Close()

	bc2, err := Open(coursePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer bc2.Close()

	var wrongKey [32]byte
	rand.Read(wrongKey[:])

	_, err = bc2.ReadPayload(wrongKey, m.CourseID)
	if err == nil {
		t.Fatal("expected ReadPayload to fail with wrong key")
	}
}

func TestBinaryCourse_ReadPayloadWrongCourseID(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	_ = pub

	m := CourseManifest{
		CourseID: "course-right-id",
		Version:  "1.0.0",
		Title:    "Wrong ID Test",
		Language: "en",
	}

	manifestJSON, _, err := SignManifest(m, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("secret"), 0644)

	var tarBuf bytes.Buffer
	if err := BuildTarGz(&tarBuf, srcDir); err != nil {
		t.Fatalf("BuildTarGz: %v", err)
	}

	var aesKey [32]byte
	rand.Read(aesKey[:])

	encrypted, err := Encrypt(tarBuf.Bytes(), aesKey[:], m.CourseID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	coursePath := filepath.Join(t.TempDir(), "wrongid.course")
	bc, err := Create(coursePath, manifestJSON)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	bc.WritePayload(encrypted)
	bc.Close()

	bc2, err := Open(coursePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer bc2.Close()

	_, err = bc2.ReadPayload(aesKey, "wrong-course-id")
	if err == nil {
		t.Fatal("expected ReadPayload to fail with wrong course ID")
	}
}

func TestBinaryCourse_OpenTruncated(t *testing.T) {
	// Write only header + partial manifest (we don't use the real manifest).
	path := filepath.Join(t.TempDir(), "truncated.course")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Write magic + version + flags + manifestLen (claiming 9999 bytes).
	hdr := make([]byte, 12)
	copy(hdr[0:4], "\x5a\x54\x43\x52")
	hdr[4] = 1 // version low byte (little-endian: 1)
	hdr[5] = 0 // version high byte
	hdr[6] = 1 // flags low byte (bit 0 = gzip)
	hdr[7] = 0 // flags high byte
	// manifestLen = 9999
	hdr[8] = 0x0f
	hdr[9] = 0x27
	hdr[10] = 0
	hdr[11] = 0
	f.Write(hdr)
	// Write only 5 bytes of manifest instead of 9999.
	f.Write([]byte("short"))
	f.Close()

	_, err = Open(path)
	if err == nil {
		t.Fatal("expected error opening truncated file")
	}
	if !strings.Contains(err.Error(), "manifest") && !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("expected manifest read error, got: %v", err)
	}
}

func TestBinaryCourse_ExtractTarGz_InvalidGzip(t *testing.T) {
	dir := t.TempDir()
	err := ExtractTarGz(bytes.NewReader([]byte("not a gzip stream")), dir)
	if err == nil {
		t.Fatal("expected error for invalid gzip data")
	}
}
