package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ztutor/internal/db"
	"ztutor/internal/license"
)

func TestDiscoverLicenseFile_ExplicitEnvWins(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "custom.key")
	if err := os.WriteFile(explicit, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("ZTUTOR_LICENSE_FILE", explicit)

	got := discoverLicenseFile(filepath.Join(dir, "data"))
	if got != explicit {
		t.Fatalf("discoverLicenseFile = %q, want %q", got, explicit)
	}
}

func TestDiscoverLicenseFile_FallsBackToDataDir(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dataDir, "license.key")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("ZTUTOR_LICENSE_FILE", "")

	got := discoverLicenseFile(dataDir)
	if got != path {
		t.Fatalf("discoverLicenseFile = %q, want %q", got, path)
	}
}

func TestDiscoverLicenseFile_ExplicitMissingReturnsEmpty(t *testing.T) {
	t.Setenv("ZTUTOR_LICENSE_FILE", "/definitely/missing/license.key")
	if got := discoverLicenseFile(t.TempDir()); got != "" {
		t.Fatalf("discoverLicenseFile = %q, want empty", got)
	}
}

func TestConfigureLicensePublicKey_InvalidHexDoesNotPanic(t *testing.T) {
	license.SetPublicKey(nil)
	t.Cleanup(func() { license.SetPublicKey(nil) })
	t.Setenv("ZTUTOR_LICENSE_PUBKEY", "zz-not-hex")
	configureLicensePublicKey()

	info := license.Info{Licensee: "Acme", IssuedAt: time.Now().Format(time.RFC3339)}
	_, _, err := license.CheckData([]byte(`{"sig":"abc","payload":{"sub":"Acme","iat":"` + info.IssuedAt + `"}}`))
	if err == nil || !strings.Contains(err.Error(), "license verification key not configured") {
		t.Fatalf("CheckData error = %v, want missing verification key", err)
	}
}

func TestConfigureLicensePublicKey_ValidKeyEnablesVerification(t *testing.T) {
	pub, priv, err := license.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	license.SetPublicKey(nil)
	t.Cleanup(func() { license.SetPublicKey(nil) })
	t.Setenv("ZTUTOR_LICENSE_PUBKEY", hex.EncodeToString(pub))
	configureLicensePublicKey()

	info := license.Info{Licensee: "Acme", IssuedAt: time.Now().Format(time.RFC3339)}
	signed, err := license.Sign(priv, info)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	state, _, err := license.CheckData(signed)
	if err != nil {
		t.Fatalf("CheckData: %v", err)
	}
	if state == nil || !state.Licensed {
		t.Fatalf("state = %+v, want licensed state", state)
	}
}

func TestLoadStartupLicense_RedeemsPersonalLicense(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "ztutor.db"))
	if err != nil {
		t.Fatalf("Open DB: %v", err)
	}
	defer database.Close()
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	pub, priv, err := license.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	license.SetPublicKey(pub)
	t.Cleanup(func() { license.SetPublicKey(nil) })

	info := license.Info{
		Licensee:        "Campaign",
		LicenseID:       "lic-startup",
		Username:        "alice",
		UnlockedCourses: []string{"c-module-02"},
		CourseKey:       hex.EncodeToString(make([]byte, 32)),
		IssuedAt:        time.Now().Format(time.RFC3339),
	}
	signed, err := license.Sign(priv, info)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "license.key"), signed, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state := loadStartupLicense("alice", database, dataDir)
	if state == nil || !state.CanAccessCourse("c-module-02") {
		t.Fatalf("loadStartupLicense state = %+v, want unlocked c-module-02", state)
	}
	enrolled, err := database.ListEnrollments("alice")
	if err != nil {
		t.Fatalf("ListEnrollments: %v", err)
	}
	if len(enrolled) != 1 || enrolled[0] != "c-module-02" {
		t.Fatalf("enrollments = %v, want [c-module-02]", enrolled)
	}
}

func TestLoadStartupLicense_InvalidFileReturnsNil(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "ztutor.db"))
	if err != nil {
		t.Fatalf("Open DB: %v", err)
	}
	defer database.Close()
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "license.key"), []byte("not-json"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if state := loadStartupLicense("alice", database, dataDir); state != nil {
		t.Fatalf("loadStartupLicense state = %+v, want nil", state)
	}
}

func TestLoadStartupLicense_RedeemedByAnotherUserReturnsNil(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "ztutor.db"))
	if err != nil {
		t.Fatalf("Open DB: %v", err)
	}
	defer database.Close()
	for _, user := range []string{"alice", "bob"} {
		if err := database.CreateUser(user, "", db.RoleStudent); err != nil {
			t.Fatalf("CreateUser(%s): %v", user, err)
		}
	}

	pub, priv, err := license.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	license.SetPublicKey(pub)
	t.Cleanup(func() { license.SetPublicKey(nil) })

	info := license.Info{
		Licensee:        "Campaign",
		LicenseID:       "lic-redeemed",
		Username:        "alice",
		UnlockedCourses: []string{"c-module-02"},
		IssuedAt:        time.Now().Format(time.RFC3339),
	}
	signed, err := license.Sign(priv, info)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := database.RedeemPersonalLicense("bob", info, signed); err == nil {
		t.Fatal("expected bob redemption to fail for alice-bound license")
	}
	info.Username = ""
	info.Email = ""
	signed, err = license.Sign(priv, info)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := database.RedeemPersonalLicense("bob", info, signed); err != nil {
		t.Fatalf("RedeemPersonalLicense bob: %v", err)
	}

	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "license.key"), signed, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if state := loadStartupLicense("alice", database, dataDir); state != nil {
		t.Fatalf("loadStartupLicense state = %+v, want nil when redemption fails", state)
	}
}
