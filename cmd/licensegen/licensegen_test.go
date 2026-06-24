package main

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ztutor/internal/license"
)

func TestGeneratedKeypairWorks(t *testing.T) {
	pub, priv, err := license.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	license.SetPublicKey(pub)

	info := license.Info{
		Licensee:    "Test School",
		MaxStudents: 50,
		Features:    []string{"premium_lessons", "multi_user"},
		IssuedAt:    "2025-01-01T00:00:00Z",
	}
	signed, err := license.Sign(priv, info)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	dir := t.TempDir()
	licFile := filepath.Join(dir, "license.key")
	os.WriteFile(licFile, signed, 0644)

	state, err := license.Check(licFile)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !state.Licensed {
		t.Error("expected licensed")
	}
	if state.Licensee != "Test School" {
		t.Errorf("licensee = %q", state.Licensee)
	}
	if !state.HasMultiUser {
		t.Error("expected HasMultiUser")
	}
}

func TestGeneratedKeypair_Expired(t *testing.T) {
	pub, priv, err := license.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	license.SetPublicKey(pub)

	info := license.Info{
		Licensee:    "Expired Org",
		MaxStudents: 10,
		Features:    []string{"multi_user"},
		ExpiresAt:   time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
		IssuedAt:    time.Now().Add(-96 * time.Hour).Format(time.RFC3339),
	}
	signed, err := license.Sign(priv, info)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	dir := t.TempDir()
	licFile := filepath.Join(dir, "license.key")
	os.WriteFile(licFile, signed, 0644)

	_, err = license.Check(licFile)
	if err != license.ErrExpired {
		t.Errorf("expected ErrExpired, got %v", err)
	}
}

func TestGeneratedKeypair_WrongKey(t *testing.T) {
	pub, _, err := license.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	_, wrongPriv, _ := license.GenerateKeyPair()
	license.SetPublicKey(pub)

	info := license.Info{
		Licensee:    "Wrong Key Org",
		MaxStudents: 5,
		Features:    []string{"multi_user"},
		IssuedAt:    time.Now().Format(time.RFC3339),
	}
	signed, err := license.Sign(wrongPriv, info)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	dir := t.TempDir()
	licFile := filepath.Join(dir, "license.key")
	os.WriteFile(licFile, signed, 0644)

	_, err = license.Check(licFile)
	if err == nil {
		t.Error("expected verification failure for license signed with wrong key")
	}
}

func TestSaveAndLoadKeyFile(t *testing.T) {
	// Simulate --gen-key: generate keypair, marshal as keyFile, write to disk.
	pub, priv, err := license.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	kf := keyFile{
		PublicKey:  hexEncode(pub),
		PrivateKey: hexEncode(priv),
	}
	pubKeyBytes := ed25519.PublicKey(pub)

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ztutor_license_keys.json")

	data, _ := json.MarshalIndent(kf, "", "  ")
	os.WriteFile(keyPath, data, 0600)

	// Read it back and sign.
	readData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key file: %v", err)
	}
	var loaded keyFile
	if err := json.Unmarshal(readData, &loaded); err != nil {
		t.Fatalf("parse key file: %v", err)
	}
	if loaded.PublicKey != kf.PublicKey {
		t.Error("loaded public key mismatch")
	}

	privBytes, err := hexDecode(loaded.PrivateKey)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	loadedPriv := ed25519.PrivateKey(privBytes)

	license.SetPublicKey(pubKeyBytes)
	info := license.Info{
		Licensee:    "Loaded Key School",
		MaxStudents: 25,
		Features:    []string{"multi_user"},
		IssuedAt:    time.Now().Format(time.RFC3339),
	}
	signed, err := license.Sign(loadedPriv, info)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	licFile := filepath.Join(dir, "license.key")
	os.WriteFile(licFile, signed, 0644)

	state, err := license.Check(licFile)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !state.Licensed {
		t.Error("license should be valid")
	}
	if state.Licensee != "Loaded Key School" {
		t.Errorf("licensee = %q", state.Licensee)
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Acme Corp", "acme_corp"},
		{"Test School", "test_school"},
		{"abc_123-XYZ", "abc_123-xyz"},
		{"hello!@#world", "hello___world"},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitize(tt.input)
		if got != tt.want {
			t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHexDecodeRoundtrip(t *testing.T) {
	// hexEncode is not exported — test the decode against known hex values.
	tests := []struct {
		hex string
	}{
		{"deadbeef"},
		{"0123456789abcdef"},
		{"ABCDEF"},
	}
	for _, tt := range tests {
		b, err := hexDecode(tt.hex)
		if err != nil {
			t.Errorf("hexDecode(%q): %v", tt.hex, err)
		}
		if len(b) != len(tt.hex)/2 {
			t.Errorf("hexDecode(%q) len = %d, want %d", tt.hex, len(b), len(tt.hex)/2)
		}
	}
}

func TestParseFeatures(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"  ", nil},
		{"multi_user", []string{"multi_user"}},
		{"multi_user,admin_ui", []string{"multi_user", "admin_ui"}},
		{"multi_user, admin_ui, interviews", []string{"multi_user", "admin_ui", "interviews"}},
		{"multi_user,,admin_ui", []string{"multi_user", "admin_ui"}},
	}
	for _, tt := range tests {
		got := parseFeatures(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseFeatures(%q) = %v (%d), want %v (%d)", tt.input, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseFeatures(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input  string
		wantOK bool
	}{
		{"365d", true},
		{"30d", true},
		{"1y", true},
		{"2years", true},
		{"6mo", true},
		{"12months", true},
		{"0d", false},
		{"", false},
		{"abc", false},
	}
	for _, tt := range tests {
		got := parseDuration(tt.input)
		if (got > 0) != tt.wantOK {
			t.Errorf("parseDuration(%q) = %v, wantOK=%v", tt.input, got, tt.wantOK)
		}
	}
}

func TestHexDecode_Invalid(t *testing.T) {
	_, err := hexDecode("ghij")
	if err == nil {
		t.Error("expected error for invalid hex")
	}
	if err == nil || !strings.Contains(err.Error(), "invalid hex") {
		t.Errorf("unexpected error: %v", err)
	}
}

// hexEncode mirrors the fmt.Sprintf("%x", ...) used in main.go.
func hexEncode(b []byte) string {
	hex := make([]byte, len(b)*2)
	for i, v := range b {
		hex[i*2] = "0123456789abcdef"[v>>4]
		hex[i*2+1] = "0123456789abcdef"[v&0x0f]
	}
	return string(hex)
}
