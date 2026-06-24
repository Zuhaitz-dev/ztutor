// Package crypt implements the encrypted .course binary format for ztutor.
//
// Format (all integers little-endian):
//
//	Offset  Size   Field
//	0       4      Magic: 0x5A 0x54 0x43 0x52 ("ZTCR")
//	4       2      Version uint16 (1)
//	6       2      Flags uint16 (bit 0 = gzip compressed payload)
//	8       4      ManifestLen uint32 — length of the JSON manifest in bytes
//	12      N      Manifest — UTF-8 JSON (plaintext, Ed25519-signed)
//	12+N    M      Payload — AES-256-GCM encrypted tar.gz of the course directory
//
// The payload is encrypted with a 256-bit key from the license payload.
// The course ID is passed as Additional Authenticated Data (AAD), so
// decryption fails if the .course file is renamed or repurposed.
// The GCM nonce (12 random bytes) is prepended to the ciphertext.
package crypt
