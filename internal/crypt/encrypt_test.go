package crypt

import (
	"crypto/rand"
	"testing"
)

func generateKey() [32]byte {
	var key [32]byte
	rand.Read(key[:])
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := generateKey()
	plaintext := []byte("hello, world — this is a test message for round-trip encryption")
	courseID := "course-abc123"

	ciphertext, err := Encrypt(plaintext, key[:], courseID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, key[:], courseID)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("round-trip mismatch:\n  got:  %q\n  want: %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_WrongKey(t *testing.T) {
	key1 := generateKey()
	key2 := generateKey()
	plaintext := []byte("sensitive data")
	courseID := "course-abc123"

	ciphertext, err := Encrypt(plaintext, key1[:], courseID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = Decrypt(ciphertext, key2[:], courseID)
	if err == nil {
		t.Fatal("expected decryption to fail with wrong key")
	}
}

func TestEncryptDecrypt_WrongCourseID(t *testing.T) {
	key := generateKey()
	plaintext := []byte("course-bound data")
	courseID := "course-abc123"

	ciphertext, err := Encrypt(plaintext, key[:], courseID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = Decrypt(ciphertext, key[:], "course-different")
	if err == nil {
		t.Fatal("expected decryption to fail with wrong course ID (AAD mismatch)")
	}
}

func TestEncryptDecrypt_TamperedCiphertext(t *testing.T) {
	key := generateKey()
	plaintext := []byte("tamper-proof data")
	courseID := "course-abc123"

	ciphertext, err := Encrypt(plaintext, key[:], courseID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Flip a bit in the ciphertext portion (after the nonce).
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0x01

	_, err = Decrypt(tampered, key[:], courseID)
	if err == nil {
		t.Fatal("expected decryption to fail on tampered ciphertext")
	}
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	key := generateKey()
	courseID := "course-empty"

	ciphertext, err := Encrypt([]byte{}, key[:], courseID)
	if err != nil {
		t.Fatalf("Encrypt (empty): %v", err)
	}

	decrypted, err := Decrypt(ciphertext, key[:], courseID)
	if err != nil {
		t.Fatalf("Decrypt (empty): %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("expected empty decrypted, got %d bytes", len(decrypted))
	}
}

func TestEncrypt_KeyTooShort(t *testing.T) {
	shortKey := make([]byte, 16)
	rand.Read(shortKey)

	_, err := Encrypt([]byte("data"), shortKey, "course-1")
	if err == nil {
		t.Fatal("expected error for key < 32 bytes")
	}
}

func TestEncrypt_KeyTooLong(t *testing.T) {
	longKey := make([]byte, 64)
	rand.Read(longKey)

	_, err := Encrypt([]byte("data"), longKey, "course-1")
	if err == nil {
		t.Fatal("expected error for key > 32 bytes")
	}
}

func TestDecrypt_DataTooShort(t *testing.T) {
	key := generateKey()
	_, err := Decrypt([]byte{0x01, 0x02, 0x03}, key[:], "course-1")
	if err == nil {
		t.Fatal("expected error for data shorter than nonce size")
	}
}

func TestDecrypt_KeyTooShort(t *testing.T) {
	shortKey := make([]byte, 16)
	rand.Read(shortKey)

	_, err := Decrypt(make([]byte, 32), shortKey, "course-1")
	if err == nil {
		t.Fatal("expected error for key < 32 bytes")
	}
}

func TestEncryptDecrypt_NilKey(t *testing.T) {
	_, err := Encrypt([]byte("data"), nil, "course-1")
	if err == nil {
		t.Fatal("expected error for nil key")
	}
}

func TestDecrypt_NilData(t *testing.T) {
	key := generateKey()
	_, err := Decrypt(nil, key[:], "course-1")
	if err == nil {
		t.Fatal("expected error for nil data")
	}
}

func TestEncryptDecrypt_UnicodeCourseID(t *testing.T) {
	key := generateKey()
	plaintext := []byte("unicode course id test")
	courseID := "coursé-日本語-🇪🇸"

	ciphertext, err := Encrypt(plaintext, key[:], courseID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, key[:], courseID)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("unicode course ID round-trip mismatch")
	}
}

func TestEncryptDecrypt_LargePlaintext(t *testing.T) {
	key := generateKey()
	plaintext := make([]byte, 1024*1024) // 1 MB
	rand.Read(plaintext)
	courseID := "course-large"

	ciphertext, err := Encrypt(plaintext, key[:], courseID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, key[:], courseID)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if len(decrypted) != len(plaintext) {
		t.Fatalf("large plaintext length mismatch: got %d, want %d", len(decrypted), len(plaintext))
	}
	for i := range plaintext {
		if decrypted[i] != plaintext[i] {
			t.Fatalf("large plaintext mismatch at byte %d", i)
		}
	}
}
