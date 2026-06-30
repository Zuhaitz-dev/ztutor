package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

var (
	ErrNotLicensed = errors.New("no valid license found")
	ErrExpired     = errors.New("license expired")
)

type Info struct {
	Licensee        string   `json:"sub"`
	LicenseID       string   `json:"license_id,omitempty"`
	Username        string   `json:"username,omitempty"`
	Email           string   `json:"email,omitempty"`
	MaxStudents     int      `json:"max_students"`
	Features        []string `json:"features"`
	UnlockedCourses []string `json:"unlocked_courses,omitempty"`
	CourseKey       string   `json:"course_key,omitempty"`
	ExpiresAt       string   `json:"exp,omitempty"`
	IssuedAt        string   `json:"iat"`
}

type State struct {
	Licensed              bool
	Licensee              string
	LicenseID             string
	Username              string
	Email                 string
	MaxStudents           int
	ExpiresAt             time.Time
	UnlockedCourses       []string
	CourseKey             []byte
	HasMultiUser          bool
	HasAdminUI            bool
	HasInterviewQuestions bool
}

// ProductTier classifies the currently active license into the product-facing
// tiers used in the UI and docs.
func (s *State) ProductTier() string {
	if s == nil || !s.Licensed {
		return "free"
	}
	if s.HasAdminUI || s.HasMultiUser {
		return "business"
	}
	return "premium"
}

func (i Info) IsExpired() bool {
	if i.ExpiresAt == "" {
		return false
	}
	exp, err := time.Parse(time.RFC3339, i.ExpiresAt)
	if err != nil {
		return false
	}
	return time.Now().After(exp)
}

func (i Info) HasFeature(f string) bool {
	for _, feat := range i.Features {
		if feat == f {
			return true
		}
	}
	return false
}

func Check(licenseFile string) (*State, error) {
	data, err := os.ReadFile(licenseFile)
	if err != nil {
		return nil, ErrNotLicensed
	}

	state, _, err := CheckData(data)
	return state, err
}

func CheckData(data []byte) (*State, Info, error) {
	sig, info, err := parseLicenseFile(data)
	if err != nil {
		return nil, Info{}, fmt.Errorf("invalid license: %w", err)
	}

	if len(publicKey) == 0 {
		return nil, Info{}, fmt.Errorf("license verification key not configured")
	}

	if !verify(sig, info) {
		return nil, Info{}, fmt.Errorf("license signature verification failed")
	}

	if info.IsExpired() {
		return nil, Info{}, ErrExpired
	}

	interviewUnlocked := info.HasFeature("interviews")

	state := &State{
		Licensed:              true,
		Licensee:              info.Licensee,
		LicenseID:             info.LicenseID,
		Username:              info.Username,
		Email:                 info.Email,
		MaxStudents:           info.MaxStudents,
		UnlockedCourses:       info.UnlockedCourses,
		HasMultiUser:          info.HasFeature("multi_user"),
		HasAdminUI:            info.HasFeature("admin_ui"),
		HasInterviewQuestions: interviewUnlocked,
	}

	if info.ExpiresAt != "" {
		state.ExpiresAt, _ = time.Parse(time.RFC3339, info.ExpiresAt)
	}

	if info.CourseKey != "" {
		var err error
		state.CourseKey, err = hex.DecodeString(info.CourseKey)
		if err != nil {
			state.CourseKey = nil
		}
	}

	return state, info, nil
}

func (i Info) IsPersonal() bool {
	return i.Username != "" || i.Email != "" || i.LicenseID != ""
}

func (s *State) CanAccessCourse(courseID string) bool {
	if s == nil {
		return false
	}
	return contains(s.UnlockedCourses, courseID)
}

func (s *State) CanDecryptCourse(courseID string) bool {
	if s == nil || s.CourseKey == nil {
		return false
	}
	if contains(s.UnlockedCourses, "*") {
		return true
	}
	return contains(s.UnlockedCourses, courseID)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func parseLicenseFile(data []byte) (signature []byte, info Info, err error) {
	var wrapper struct {
		Sig     string          `json:"sig"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, Info{}, fmt.Errorf("parse license: %w", err)
	}

	sig, err := base64.RawURLEncoding.DecodeString(wrapper.Sig)
	if err != nil {
		return nil, Info{}, fmt.Errorf("decode sig: %w", err)
	}
	if err := json.Unmarshal(wrapper.Payload, &info); err != nil {
		return nil, Info{}, fmt.Errorf("decode payload: %w", err)
	}
	return sig, info, nil
}

func verify(sig []byte, info Info) bool {
	if len(sig) == 0 || len(publicKey) == 0 {
		return false
	}
	payload, err := json.Marshal(info)
	if err != nil {
		return false
	}
	return ed25519.Verify(publicKey, payload, sig)
}

func Sign(privateKey ed25519.PrivateKey, info Info) ([]byte, error) {
	payload, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(privateKey, payload)
	wrapper := struct {
		Sig     string          `json:"sig"`
		Payload json.RawMessage `json:"payload"`
	}{
		Sig:     base64.RawURLEncoding.EncodeToString(sig),
		Payload: payload,
	}
	return json.MarshalIndent(wrapper, "", "  ")
}

func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}
