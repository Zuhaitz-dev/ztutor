package license

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	SetPublicKey(pub)

	info := Info{
		Licensee:        "Test School",
		MaxStudents:     50,
		Features:        []string{"multi_user", "admin_ui", "interviews"},
		UnlockedCourses: []string{"c-programming", "python-basics"},
		IssuedAt:        time.Now().Format(time.RFC3339),
	}

	signed, err := Sign(priv, info)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	var wrapper struct {
		Sig     string          `json:"sig"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(signed, &wrapper); err != nil {
		t.Fatalf("unmarshal signed: %v", err)
	}
	if wrapper.Sig == "" {
		t.Error("signature is empty")
	}

	sig, payload, err := parseLicenseFile(signed)
	if err != nil {
		t.Fatalf("parseLicenseFile: %v", err)
	}

	payloadBytes, _ := json.Marshal(payload)
	if !ed25519.Verify(pub, payloadBytes, sig) {
		t.Error("signature verification failed")
	}
}

func TestCheckValidLicense(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	SetPublicKey(pub)

	info := Info{
		Licensee:        "Acme Corp",
		MaxStudents:     100,
		Features:        []string{"multi_user", "admin_ui", "interviews"},
		UnlockedCourses: []string{"c-programming", "c-pointers", "python-basics"},
		IssuedAt:        time.Now().Format(time.RFC3339),
	}

	signed, _ := Sign(priv, info)

	dir := t.TempDir()
	path := filepath.Join(dir, "license.key")
	os.WriteFile(path, signed, 0644)

	state, err := Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if !state.Licensed {
		t.Error("should be licensed")
	}
	if state.Licensee != "Acme Corp" {
		t.Errorf("licensee = %q", state.Licensee)
	}
	if state.MaxStudents != 100 {
		t.Errorf("max students = %d", state.MaxStudents)
	}
	if len(state.UnlockedCourses) != 3 {
		t.Errorf("expected 3 unlocked courses, got %d: %v", len(state.UnlockedCourses), state.UnlockedCourses)
	}
	if !state.HasMultiUser {
		t.Error("should have multi user")
	}
	if !state.HasAdminUI {
		t.Error("should have admin UI")
	}
	if !state.HasInterviewQuestions {
		t.Error("should have interviews")
	}
}

func TestCheckNoLicense(t *testing.T) {
	pub, _, _ := GenerateKeyPair()
	SetPublicKey(pub)

	_, err := Check("/nonexistent/license.key")
	if err != ErrNotLicensed {
		t.Errorf("expected ErrNotLicensed, got %v", err)
	}
}

func TestStateProductTier(t *testing.T) {
	tests := []struct {
		name string
		in   *State
		want string
	}{
		{name: "free nil", in: nil, want: "free"},
		{name: "free unlicensed", in: &State{}, want: "free"},
		{name: "premium", in: &State{Licensed: true}, want: "premium"},
		{name: "business admin", in: &State{Licensed: true, HasAdminUI: true}, want: "business"},
		{name: "business multi", in: &State{Licensed: true, HasMultiUser: true}, want: "business"},
	}
	for _, tt := range tests {
		if got := tt.in.ProductTier(); got != tt.want {
			t.Errorf("%s: ProductTier() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestCheckExpiredLicense(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	SetPublicKey(pub)

	info := Info{
		Licensee:        "Expired Inc",
		MaxStudents:     10,
		Features:        []string{"multi_user"},
		UnlockedCourses: []string{"c-programming"},
		ExpiresAt:       time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		IssuedAt:        time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
	}

	signed, _ := Sign(priv, info)

	dir := t.TempDir()
	path := filepath.Join(dir, "license.key")
	os.WriteFile(path, signed, 0644)

	_, err = Check(path)
	if err != ErrExpired {
		t.Errorf("expected ErrExpired, got %v", err)
	}
}

func TestCheckTamperedLicense(t *testing.T) {
	pub, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	otherPub, otherPriv, _ := GenerateKeyPair()
	_ = otherPub
	SetPublicKey(pub)

	info := Info{
		Licensee:        "Tampered",
		MaxStudents:     999,
		Features:        []string{"multi_user"},
		UnlockedCourses: []string{"c-programming"},
		IssuedAt:        time.Now().Format(time.RFC3339),
	}

	signed, _ := Sign(otherPriv, info)

	dir := t.TempDir()
	path := filepath.Join(dir, "license.key")
	os.WriteFile(path, signed, 0644)

	_, err = Check(path)
	if err == nil {
		t.Error("tampered license should fail verification")
	}
}

func TestInfoExpired(t *testing.T) {
	past := Info{ExpiresAt: time.Now().Add(-time.Hour).Format(time.RFC3339)}
	future := Info{ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}
	never := Info{}

	if !past.IsExpired() {
		t.Error("past expiry should be expired")
	}
	if future.IsExpired() {
		t.Error("future expiry should not be expired")
	}
	if never.IsExpired() {
		t.Error("no expiry should not be expired")
	}
}

func TestInfoHasFeature(t *testing.T) {
	info := Info{Features: []string{"multi_user", "admin_ui"}}

	if !info.HasFeature("multi_user") {
		t.Error("should have multi_user")
	}
	if !info.HasFeature("admin_ui") {
		t.Error("should have admin_ui")
	}
	if info.HasFeature("interviews") {
		t.Error("should not have interviews")
	}
}

func TestCanAccessCourse(t *testing.T) {
	s := &State{
		UnlockedCourses: []string{"c-programming", "python-basics"},
	}

	if !s.CanAccessCourse("c-programming") {
		t.Error("should access c-programming")
	}
	if !s.CanAccessCourse("python-basics") {
		t.Error("should access python-basics")
	}
	if s.CanAccessCourse("c-interviews") {
		t.Error("should not access c-interviews")
	}

	var nilState *State
	if nilState.CanAccessCourse("c-programming") {
		t.Error("nil state should not access any course")
	}
}

func TestInterviewsBackwardCompat(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	SetPublicKey(pub)

	info := Info{
		Licensee:    "Interview School",
		MaxStudents: 10,
		Features:    []string{"interviews"},
		IssuedAt:    time.Now().Format(time.RFC3339),
	}

	signed, _ := Sign(priv, info)

	dir := t.TempDir()
	path := filepath.Join(dir, "license.key")
	os.WriteFile(path, signed, 0644)

	state, err := Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if !state.HasInterviewQuestions {
		t.Error("should have interview questions")
	}
	if contains(state.UnlockedCourses, "c-interviews") {
		t.Error("c-interviews should NOT be auto-added to unlocked courses (interviews are now sections within courses)")
	}
}
