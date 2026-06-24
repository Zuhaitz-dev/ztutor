package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrGenerateHostKey_GeneratesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host_key")

	signer, err := loadOrGenerateHostKey(path)
	if err != nil {
		t.Fatalf("loadOrGenerateHostKey: %v", err)
	}
	if signer == nil {
		t.Fatal("signer is nil")
	}

	// File should exist after generation.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("host key file should exist: %v", err)
	}
}

func TestLoadOrGenerateHostKey_LoadsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host_key")

	// Generate once.
	signer1, err := loadOrGenerateHostKey(path)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}

	// Load existing — should return same key.
	signer2, err := loadOrGenerateHostKey(path)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}

	pub1 := signer1.PublicKey()
	pub2 := signer2.PublicKey()
	if string(pub1.Marshal()) != string(pub2.Marshal()) {
		t.Error("public key changed between loads")
	}
}
