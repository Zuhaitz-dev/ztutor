package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"ztutor/internal/crypt"
)

func TestCoursepack_EndToEnd(t *testing.T) {
	dir := t.TempDir()

	courseDir := filepath.Join(dir, "python-basics")
	os.MkdirAll(filepath.Join(courseDir, "lessons", "01-intro"), 0755)
	os.WriteFile(filepath.Join(courseDir, "course.yaml"), []byte("id: python-basics\ntitle: Python Basics\nlanguage: python\n"), 0644)
	os.WriteFile(filepath.Join(courseDir, "lessons", "01-intro", "lesson.md"), []byte("# Hello\n\nContent"), 0644)

	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	var aesKey [32]byte
	rand.Read(aesKey[:])

	outPath := filepath.Join(dir, "python-basics.course")

	cm := crypt.CourseManifest{CourseID: "python-basics", Version: "1.0.0", Title: "Python Basics", Language: "python"}
	manJSON, sig, err := crypt.SignManifest(cm, privKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	var tarBuf bytes.Buffer
	if err := crypt.BuildTarGz(&tarBuf, courseDir); err != nil {
		t.Fatalf("build tar: %v", err)
	}

	encrypted, err := crypt.Encrypt(tarBuf.Bytes(), aesKey[:], "python-basics")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	bc, err := crypt.Create(outPath, manJSON)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	bc.WritePayload(encrypted)
	bc.Close()

	bc2, err := crypt.Open(outPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer bc2.Close()

	decoded, err := bc2.DecodeManifest(pubKey, sig)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if decoded.CourseID != "python-basics" {
		t.Errorf("course ID = %s", decoded.CourseID)
	}

	payload, err := bc2.ReadPayload(aesKey, "python-basics")
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}

	extractDir := filepath.Join(dir, "extracted")
	os.MkdirAll(extractDir, 0755)
	if err := crypt.ExtractTarGz(bytes.NewReader(payload), extractDir); err != nil {
		t.Fatalf("extract: %v", err)
	}

	if _, err := os.Stat(filepath.Join(extractDir, "course.yaml")); err != nil {
		t.Error("course.yaml not found in extracted content")
	}
}
