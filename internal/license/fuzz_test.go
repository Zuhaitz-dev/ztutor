package license

import (
	"testing"
)

func FuzzCheckData(f *testing.F) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		f.Skip()
	}
	SetPublicKey(pub)

	info := Info{
		Licensee:    "Fuzz Test",
		MaxStudents: 50,
		Features:    []string{"multi_user"},
		IssuedAt:    "2025-01-01T00:00:00Z",
	}
	validData, err := Sign(priv, info)
	if err != nil {
		f.Skip()
	}

	f.Add(validData)
	f.Add([]byte{})
	f.Add([]byte("invalid"))
	f.Add([]byte("not valid json"))
	f.Add([]byte("{\"licensee\": \"bad\"}"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			return
		}
		_, _, err := CheckData(data)
		// CheckData should never panic, regardless of input.
		_ = err
	})
}

func FuzzSignVerifyRoundtrip(f *testing.F) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		f.Skip()
	}
	SetPublicKey(pub)

	f.Add("Test", "Fuzz User", "fuzz@example.com", "id-001", 10, int64(365*24*3600))

	f.Fuzz(func(t *testing.T, licensee, username, email, licenseID string, maxStudents int, expiresUnix int64) {
		if len(licensee) > 256 || len(username) > 256 || len(email) > 256 || len(licenseID) > 256 {
			return
		}
		if maxStudents < 0 || maxStudents > 100000 {
			return
		}

		info := Info{
			Licensee:    licensee,
			Username:    username,
			Email:       email,
			LicenseID:   licenseID,
			MaxStudents: maxStudents,
			Features:    []string{"multi_user"},
			IssuedAt:    "2025-01-01T00:00:00Z",
		}

		signed, err := Sign(priv, info)
		if err != nil {
			t.Skip()
		}

		state, parsedInfo, err := CheckData(signed)
		if err != nil {
			t.Fatalf("CheckData failed on valid signed blob: %v", err)
		}
		if state == nil {
			t.Fatal("state is nil")
		}
		_ = parsedInfo
	})
}
