package crypt

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
)

func TestManifest_SignAndVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	m := CourseManifest{
		CourseID: "course-foo",
		Version:  "1.0.0",
		Title:    "Introduction to Go",
		Language: "en",
	}

	jsonBytes, sig, err := SignManifest(m, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	decoded, err := VerifyManifest(jsonBytes, sig, pub)
	if err != nil {
		t.Fatalf("VerifyManifest: %v", err)
	}

	if decoded.CourseID != m.CourseID {
		t.Fatalf("CourseID mismatch: got %q, want %q", decoded.CourseID, m.CourseID)
	}
	if decoded.Version != m.Version {
		t.Fatalf("Version mismatch: got %q, want %q", decoded.Version, m.Version)
	}
	if decoded.Title != m.Title {
		t.Fatalf("Title mismatch: got %q, want %q", decoded.Title, m.Title)
	}
	if decoded.Language != m.Language {
		t.Fatalf("Language mismatch: got %q, want %q", decoded.Language, m.Language)
	}
}

func TestManifest_TamperedJSON(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	m := CourseManifest{
		CourseID: "course-foo",
		Version:  "1.0.0",
		Title:    "Introduction to Go",
		Language: "en",
	}

	jsonBytes, sig, err := SignManifest(m, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	// Tamper with JSON by changing a field.
	tampered := make([]byte, len(jsonBytes))
	copy(tampered, jsonBytes)
	tampered = []byte(strings.Replace(string(tampered), "Introduction to Go", "Malicious Course", 1))

	_, err = VerifyManifest(tampered, sig, pub)
	if err == nil {
		t.Fatal("expected verification to fail on tampered JSON")
	}
}

func TestManifest_WrongPublicKey(t *testing.T) {
	_, priv1, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key1: %v", err)
	}
	pub2, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key2: %v", err)
	}

	m := CourseManifest{
		CourseID: "course-foo",
		Version:  "1.0.0",
		Title:    "Introduction to Go",
		Language: "en",
	}

	jsonBytes, sig, err := SignManifest(m, priv1)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	_, err = VerifyManifest(jsonBytes, sig, pub2)
	if err == nil {
		t.Fatal("expected verification to fail with wrong public key")
	}
}

func TestManifest_MissingCourseID(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	m := CourseManifest{
		CourseID: "", // missing
		Version:  "1.0.0",
		Title:    "Introduction to Go",
		Language: "en",
	}

	jsonBytes, sig, err := SignManifest(m, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	_, err = VerifyManifest(jsonBytes, sig, pub)
	if err == nil {
		t.Fatal("expected verification to fail on missing course_id")
	}
	if !strings.Contains(err.Error(), "course_id") {
		t.Fatalf("expected error mentioning course_id, got: %v", err)
	}
}

func TestManifest_TamperedSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	m := CourseManifest{
		CourseID: "course-foo",
		Version:  "1.0.0",
		Title:    "Introduction to Go",
		Language: "en",
	}

	jsonBytes, sig, err := SignManifest(m, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	// Flip a character in the hex signature.
	sigBytes := []byte(sig)
	if sigBytes[0] == 'a' {
		sigBytes[0] = 'b'
	} else {
		sigBytes[0] = 'a'
	}

	_, err = VerifyManifest(jsonBytes, string(sigBytes), pub)
	if err == nil {
		t.Fatal("expected verification to fail on tampered signature")
	}
}

func TestManifest_InvalidSignatureHex(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	validJSON := []byte(`{"course_id":"c","version":"1","title":"T","language":"en"}`)

	// Test odd-length hex.
	_, err = VerifyManifest(validJSON, "abc", pub)
	if err == nil {
		t.Fatal("expected error for odd-length hex signature")
	}

	// Test invalid hex character.
	_, err = VerifyManifest(validJSON, "xxyy", pub)
	if err == nil {
		t.Fatal("expected error for invalid hex in signature")
	}
}

func TestManifest_UnicodeFields(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	m := CourseManifest{
		CourseID: "coursé-日本語",
		Version:  "1.0.0",
		Title:    "Introducción a Go — 日本語",
		Language: "es",
	}

	jsonBytes, sig, err := SignManifest(m, priv)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	decoded, err := VerifyManifest(jsonBytes, sig, pub)
	if err != nil {
		t.Fatalf("VerifyManifest: %v", err)
	}

	if decoded.CourseID != m.CourseID {
		t.Fatalf("unicode CourseID mismatch: got %q, want %q", decoded.CourseID, m.CourseID)
	}
	if decoded.Title != m.Title {
		t.Fatalf("unicode Title mismatch: got %q, want %q", decoded.Title, m.Title)
	}
}
