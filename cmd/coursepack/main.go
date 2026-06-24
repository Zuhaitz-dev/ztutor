package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ztutor/internal/crypt"
	"ztutor/internal/logutil"

	"gopkg.in/yaml.v3"
)

func main() {
	dir := flag.String("dir", "", "course directory containing course.yaml")
	out := flag.String("out", "", "output .course file path")
	keyHex := flag.String("key", "", "AES-256 key as hex (64 chars)")
	pubKeyHex := flag.String("publisher-key", "", "Ed25519 public key hex")
	privKeyHex := flag.String("publisher-priv", "", "Ed25519 private key hex")
	flag.Parse()

	if *dir == "" || *out == "" || *keyHex == "" || *pubKeyHex == "" || *privKeyHex == "" {
		logutil.Fatal("usage: coursepack --dir <dir> --out <file> --key <hex> --publisher-key <hex> --publisher-priv <hex>")
	}

	keyBytes, err := hex.DecodeString(*keyHex)
	if err != nil || len(keyBytes) != 32 {
		logutil.Fatal("invalid --key: must be 64 hex chars (32 bytes)")
	}
	var key [32]byte
	copy(key[:], keyBytes)

	pubBytes, err := hex.DecodeString(*pubKeyHex)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		logutil.Fatal("invalid --publisher-key: must be %d hex chars", ed25519.PublicKeySize*2)
	}
	privBytes, err := hex.DecodeString(*privKeyHex)
	if err != nil || len(privBytes) != ed25519.PrivateKeySize {
		logutil.Fatal("invalid --publisher-priv: must be %d hex chars", ed25519.PrivateKeySize*2)
	}
	pub := ed25519.PublicKey(pubBytes)
	priv := ed25519.PrivateKey(privBytes)

	courseYAML := filepath.Join(*dir, "course.yaml")
	data, err := os.ReadFile(courseYAML)
	if err != nil {
		logutil.Fatal("read course.yaml: %v", err)
	}
	var manifest struct {
		ID       string `yaml:"id"`
		Title    string `yaml:"title"`
		Language string `yaml:"language"`
	}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		logutil.Fatal("parse course.yaml: %v", err)
	}
	if manifest.ID == "" {
		logutil.Fatal("course.yaml missing 'id' field")
	}

	cm := crypt.CourseManifest{
		CourseID: manifest.ID,
		Version:  "1.0.0",
		Title:    manifest.Title,
		Language: manifest.Language,
	}
	manifestJSON, sig, err := crypt.SignManifest(cm, priv)
	if err != nil {
		logutil.Fatal("sign manifest: %v", err)
	}

	var tarBuf bytes.Buffer
	if err := crypt.BuildTarGz(&tarBuf, *dir); err != nil {
		logutil.Fatal("build tar: %v", err)
	}

	encrypted, err := crypt.Encrypt(tarBuf.Bytes(), key[:], manifest.ID)
	if err != nil {
		logutil.Fatal("encrypt: %v", err)
	}

	bc, err := crypt.Create(*out, manifestJSON)
	if err != nil {
		logutil.Fatal("create .course: %v", err)
	}
	if err := bc.WritePayload(encrypted); err != nil {
		bc.Close()
		logutil.Fatal("write payload: %v", err)
	}
	if err := bc.Close(); err != nil {
		logutil.Fatal("close: %v", err)
	}

	bc2, err := crypt.Open(*out)
	if err != nil {
		logutil.Fatal("verify: %v", err)
	}
	defer bc2.Close()

	decoded, err := bc2.DecodeManifest(pub, sig)
	if err != nil {
		logutil.Fatal("verify manifest sig: %v", err)
	}

	fmt.Printf("coursepack: %s (%s v%s) written to %s\n", decoded.CourseID, decoded.Language, decoded.Version, *out)
	fmt.Printf("  manifest signature: %s...\n", sig[:16])
	fmt.Printf("  encrypted with AES-256-GCM (course_key AAD)\n")
}
