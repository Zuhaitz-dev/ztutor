package crypt

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
)

// CourseManifest holds the course metadata embedded in the .course file header.
// It is stored as plaintext JSON and signed with the publisher's Ed25519 key.
type CourseManifest struct {
	CourseID string `json:"course_id"`
	Version  string `json:"version"`
	Title    string `json:"title"`
	Language string `json:"language"`
}

// SignManifest marshals m to JSON, signs it with priv, and returns the JSON
// bytes plus the base64-encoded Ed25519 signature.
func SignManifest(m CourseManifest, priv ed25519.PrivateKey) (jsonBytes []byte, signature string, err error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, "", fmt.Errorf("marshal manifest: %w", err)
	}
	sig := ed25519.Sign(priv, raw)
	return raw, encodeSig(sig), nil
}

// VerifyManifest checks that the JSON manifest is properly signed by pub.
// Returns the decoded CourseManifest on success.
func VerifyManifest(jsonBytes []byte, sig string, pub ed25519.PublicKey) (*CourseManifest, error) {
	sigBytes, err := decodeSig(sig)
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(pub, jsonBytes, sigBytes) {
		return nil, fmt.Errorf("manifest signature verification failed")
	}
	var m CourseManifest
	if err := json.Unmarshal(jsonBytes, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}
	if m.CourseID == "" {
		return nil, fmt.Errorf("manifest missing course_id")
	}
	return &m, nil
}
