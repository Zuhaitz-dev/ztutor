package lesson

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"ztutor/internal/crypt"
)

func decodeHexKey(hexStr string) ([32]byte, error) {
	var key [32]byte
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return key, err
	}
	if len(b) != 32 {
		return key, fmt.Errorf("key length %d, want 32", len(b))
	}
	copy(key[:], b)
	return key, nil
}

func packTestCourseWithKey(t *testing.T, courseID, dir string, key [32]byte) (coursePath string, pub ed25519.PublicKey, err error) {
	t.Helper()
	pub, priv, genErr := ed25519.GenerateKey(rand.Reader)
	if genErr != nil {
		return "", nil, genErr
	}

	manifestJSON, _, genErr := crypt.SignManifest(crypt.CourseManifest{
		CourseID: courseID,
		Version:  "1.0",
		Title:    "Test Course",
		Language: "c",
	}, priv)
	if genErr != nil {
		return "", nil, genErr
	}

	var tarBuf bytes.Buffer
	if genErr := crypt.BuildTarGz(&tarBuf, dir); genErr != nil {
		return "", nil, genErr
	}

	encrypted, genErr := crypt.Encrypt(tarBuf.Bytes(), key[:], courseID)
	if genErr != nil {
		return "", nil, genErr
	}

	coursePath = filepath.Join(t.TempDir(), courseID+".course")
	bc, genErr := crypt.Create(coursePath, manifestJSON)
	if genErr != nil {
		return "", nil, genErr
	}
	defer bc.Close()

	if genErr := bc.WritePayload(encrypted); genErr != nil {
		return "", nil, genErr
	}

	return coursePath, pub, nil
}

func createTestCourseDirWithID(t *testing.T, dir, courseID string) {
	t.Helper()

	manifest := fmt.Sprintf(`id: %s
title: Test Course
language: c
order: 2
sections:
  - id: intro
    title: Introduction
    type: exercises
    dir: lessons/
`, courseID)
	os.WriteFile(filepath.Join(dir, "course.yaml"), []byte(manifest), 0644)

	lessonDir := filepath.Join(dir, "lessons", "01-intro")
	os.MkdirAll(lessonDir, 0755)
	os.WriteFile(filepath.Join(lessonDir, "lesson.md"), []byte("# 01: Introduction\n\nWelcome.\n"), 0644)
}

func createTestCourseDir(t *testing.T, dir string) {
	t.Helper()

	manifest := `id: test-course
title: Test Course
language: c
order: 1
sections:
  - id: intro
    title: Introduction
    type: exercises
    dir: lessons/
`
	os.WriteFile(filepath.Join(dir, "course.yaml"), []byte(manifest), 0644)

	// Lessons must be in individual subdirectories under the section dir.
	lessonDir := filepath.Join(dir, "lessons", "01-intro")
	os.MkdirAll(lessonDir, 0755)
	os.WriteFile(filepath.Join(lessonDir, "lesson.md"), []byte("# 01: Introduction\n\nWelcome to the course.\n"), 0644)
}

func packTestCourse(t *testing.T, courseID, tmpDir string) (coursePath string, key [32]byte, pubKey ed25519.PublicKey) {
	t.Helper()
	_, err := rand.Read(key[:])
	if err != nil {
		t.Fatalf("rand: %v", err)
	}
	coursePath, pubKey = packTestCourseKey(t, courseID, tmpDir, key)
	return
}

func packTestCourseKey(t *testing.T, courseID, tmpDir string, key [32]byte) (coursePath string, pubKey ed25519.PublicKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519: %v", err)
	}

	manifestJSON, _, err := crypt.SignManifest(crypt.CourseManifest{
		CourseID: courseID,
		Version:  "1.0",
		Title:    "Test Course",
		Language: "en",
	}, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	var tarBuf bytes.Buffer
	if err := crypt.BuildTarGz(&tarBuf, tmpDir); err != nil {
		t.Fatalf("BuildTarGz: %v", err)
	}

	encrypted, err := crypt.Encrypt(tarBuf.Bytes(), key[:], courseID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	coursePath = filepath.Join(t.TempDir(), courseID+".course")
	bc, err := crypt.Create(coursePath, manifestJSON)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer bc.Close()

	if err := bc.WritePayload(encrypted); err != nil {
		t.Fatalf("WritePayload: %v", err)
	}

	return coursePath, pub
}

func TestLoadEncryptedCourse_FullDecrypt(t *testing.T) {
	dir := t.TempDir()
	createTestCourseDir(t, dir)

	coursePath, key, _ := packTestCourse(t, "test-course", dir)

	c, err := LoadEncryptedCourse(coursePath, "en", key[:])
	if err != nil {
		t.Fatalf("LoadEncryptedCourse: %v", err)
	}
	if c == nil {
		t.Fatal("course is nil")
	}
	if !c.Encrypted {
		t.Error("Encrypted = false, want true")
	}
	if c.ID != "test-course" {
		t.Errorf("ID = %q, want test-course", c.ID)
	}
	if c.Title != "Test Course" {
		t.Errorf("Title = %q, want Test Course", c.Title)
	}
	if len(c.Sections) == 0 {
		t.Fatal("expected at least one section")
	}
	if len(c.Sections[0].Lessons) == 0 {
		t.Fatal("expected at least one lesson in first section")
	}
	if c.TotalLessons != 1 {
		t.Errorf("TotalLessons = %d, want 1", c.TotalLessons)
	}
}

func TestLoadEncryptedCourse_NoKey_PreviewMode(t *testing.T) {
	dir := t.TempDir()
	createTestCourseDir(t, dir)

	coursePath, _, _ := packTestCourse(t, "test-course", dir)

	c, err := LoadEncryptedCourse(coursePath, "en", nil)
	if err != nil {
		t.Fatalf("LoadEncryptedCourse (preview): %v", err)
	}
	if c == nil {
		t.Fatal("course is nil")
	}
	if !c.Encrypted {
		t.Error("Encrypted = false, want true")
	}
	if c.ID != "test-course" {
		t.Errorf("ID = %q, want test-course", c.ID)
	}
	if c.Title != "Test Course" {
		t.Errorf("Title = %q, want Test Course", c.Title)
	}
	if len(c.Sections) != 0 {
		t.Errorf("Sections = %d, want 0 (preview-only, no key)", len(c.Sections))
	}
}

func TestLoadEncryptedCourse_WrongKey(t *testing.T) {
	dir := t.TempDir()
	createTestCourseDir(t, dir)

	coursePath, _, _ := packTestCourse(t, "test-course", dir)

	var wrongKey [32]byte
	_, err := rand.Read(wrongKey[:])
	if err != nil {
		t.Fatalf("rand: %v", err)
	}

	_, err = LoadEncryptedCourse(coursePath, "en", wrongKey[:])
	if err == nil {
		t.Fatal("expected error with wrong key, got nil")
	}
}

func TestLoadEncryptedCourse_MissingCourseID(t *testing.T) {
	dir := t.TempDir()
	createTestCourseDir(t, dir)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519: %v", err)
	}

	manifestJSON, _, err := crypt.SignManifest(crypt.CourseManifest{
		CourseID: "",
		Version:  "1.0",
		Title:    "Test",
		Language: "en",
	}, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	var tarBuf bytes.Buffer
	if err := crypt.BuildTarGz(&tarBuf, dir); err != nil {
		t.Fatalf("BuildTarGz: %v", err)
	}

	var key [32]byte
	_, _ = rand.Read(key[:])
	encrypted, err := crypt.Encrypt(tarBuf.Bytes(), key[:], "")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	coursePath := filepath.Join(t.TempDir(), "nocourseid.course")
	bc, err := crypt.Create(coursePath, manifestJSON)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer bc.Close()
	if err := bc.WritePayload(encrypted); err != nil {
		t.Fatalf("WritePayload: %v", err)
	}
	bc.Close()

	_, err = LoadEncryptedCourse(coursePath, "en", key[:])
	if err == nil {
		t.Fatal("expected error for missing course_id, got nil")
	}

	_ = pub
}

func TestLoadEncryptedCourse_BadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.course")
	os.WriteFile(path, []byte("this is not a valid .course file"), 0644)

	_, err := LoadEncryptedCourse(path, "en", nil)
	if err == nil {
		t.Fatal("expected error for bad file, got nil")
	}
}

func TestScanEncryptedCourses(t *testing.T) {
	scanDir := t.TempDir()

	// Source directories with different IDs in course.yaml.
	dirA := t.TempDir()
	createTestCourseDir(t, dirA)

	dirB := t.TempDir()
	createTestCourseDir(t, dirB)
	os.WriteFile(filepath.Join(dirB, "course.yaml"), []byte(`id: other-course
title: Other Course
language: c
order: 1
sections:
  - id: intro
    title: Introduction
    type: exercises
    dir: lessons/
`), 0644)

	var key [32]byte
	_, err := rand.Read(key[:])
	if err != nil {
		t.Fatalf("rand: %v", err)
	}

	// Create three .course files: two with ID "test-course" (duplicate), one with "other-course".
	pathA, _ := packTestCourseKey(t, "test-course", dirA, key)
	os.Rename(pathA, filepath.Join(scanDir, "a.course"))

	pathB, _ := packTestCourseKey(t, "other-course", dirB, key)
	os.Rename(pathB, filepath.Join(scanDir, "b.course"))

	pathC, _ := packTestCourseKey(t, "test-course", dirA, key)
	os.Rename(pathC, filepath.Join(scanDir, "c.course"))

	courses, err := ScanEncryptedCourses(scanDir, "en", key[:], nil)
	if err != nil {
		t.Fatalf("ScanEncryptedCourses: %v", err)
	}
	if len(courses) != 2 {
		t.Fatalf("courses = %d, want 2 (third had duplicate ID)", len(courses))
	}

	ids := make(map[string]bool)
	for _, c := range courses {
		ids[c.ID] = true
	}
	if !ids["test-course"] {
		t.Error("test-course not found in results")
	}
	if !ids["other-course"] {
		t.Error("other-course not found in results")
	}
}

func TestScanEncryptedCourses_SkipsExistingID(t *testing.T) {
	scanDir := t.TempDir()

	dir := t.TempDir()
	createTestCourseDir(t, dir)
	path, key, _ := packTestCourse(t, "test-course", dir)
	os.Rename(path, filepath.Join(scanDir, "test.course"))

	existing := []Course{{ID: "test-course", Title: "Already Loaded"}}

	courses, err := ScanEncryptedCourses(scanDir, "en", key[:], existing)
	if err != nil {
		t.Fatalf("ScanEncryptedCourses: %v", err)
	}
	if len(courses) != 0 {
		t.Errorf("courses = %d, want 0 (existing ID should be skipped)", len(courses))
	}
}

func TestScanEncryptedCourses_EmptyDir(t *testing.T) {
	scanDir := t.TempDir()

	courses, err := ScanEncryptedCourses(scanDir, "en", nil, nil)
	if err != nil {
		t.Fatalf("ScanEncryptedCourses: %v", err)
	}
	if len(courses) != 0 {
		t.Errorf("courses = %d, want 0", len(courses))
	}
}

func TestLoadEncryptedCourse_WithCampaignBackerKey(t *testing.T) {
	// This test verifies the full end-to-end flow that a campaign backer
	// license user would experience: the license contains a course_key and
	// unlocked_courses list; when a .course file with a matching ID is
	// placed in the courses directory, it should load and decrypt correctly.
	//
	// The campaign backer key's course_key is:
	//   0487330b846bd44c20ce39d4ede89d503171c6bf3696bd83c67a6287ffa751d9

	courseKeyHex := "0487330b846bd44c20ce39d4ede89d503171c6bf3696bd83c67a6287ffa751d9"
	expectedCourseID := "c-module-02"

	courseDir := t.TempDir()
	createTestCourseDirWithID(t, courseDir, expectedCourseID)

	// Pack the course with the EXACT campaign backer key.
	key, err := decodeHexKey(courseKeyHex)
	if err != nil {
		t.Fatalf("decode course key: %v", err)
	}

	coursePath, _, err := packTestCourseWithKey(t, expectedCourseID, courseDir, key)
	if err != nil {
		t.Fatalf("pack test course: %v", err)
	}

	// Load the course using the campaign backer key (as the user's license would).
	// This simulates the exact call from app.go's loadCourses:
	//   encrypted, _ := lesson.ScanEncryptedCourses(a.coursesDir, lang, courseKey, courses)
	c, err := LoadEncryptedCourse(coursePath, "en", key[:])
	if err != nil {
		t.Fatalf("LoadEncryptedCourse with campaign backer key: %v", err)
	}
	if c == nil {
		t.Fatal("course is nil")
	}
	if !c.Encrypted {
		t.Error("Encrypted = false, want true")
	}
	if c.ID != expectedCourseID {
		t.Errorf("ID = %q, want %q", c.ID, expectedCourseID)
	}
	if len(c.Sections) == 0 {
		t.Fatal("no sections loaded")
	}
	if c.TotalLessons < 1 {
		t.Errorf("TotalLessons = %d, want >= 1", c.TotalLessons)
	}

	// Verify ScanEncryptedCourses also works with this key.
	scanDir := t.TempDir()
	os.Rename(coursePath, filepath.Join(scanDir, expectedCourseID+".course"))

	courses, err := ScanEncryptedCourses(scanDir, "en", key[:], nil)
	if err != nil {
		t.Fatalf("ScanEncryptedCourses: %v", err)
	}
	found := false
	for _, cc := range courses {
		if cc.ID == expectedCourseID {
			found = true
			if !cc.Encrypted {
				t.Error("course should be marked Encrypted")
			}
		}
	}
	if !found {
		t.Errorf("ScanEncryptedCourses did not return course %q", expectedCourseID)
	}
}

func TestScanEncryptedCourses_SkipsBroken(t *testing.T) {
	scanDir := t.TempDir()

	os.WriteFile(filepath.Join(scanDir, "broken.course"), []byte("not a valid course file"), 0644)

	courses, err := ScanEncryptedCourses(scanDir, "en", nil, nil)
	if err != nil {
		t.Fatalf("ScanEncryptedCourses: %v", err)
	}
	if len(courses) != 0 {
		t.Errorf("courses = %d, want 0 (broken course skipped)", len(courses))
	}
}
