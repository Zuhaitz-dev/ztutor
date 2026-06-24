package crypt

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	magic     = "\x5a\x54\x43\x52" // "ZTCR"
	version1  = 1
	flagGzip  = 1 << 0
	headerLen = 4 + 2 + 2 + 4 // magic + version + flags + manifestLen = 12
)

// BinaryCourse wraps a .course file on disk for reading or writing.
type BinaryCourse struct {
	f       *os.File
	flags   uint16
	version uint16
	rawMan  []byte
}

// Open reads a .course file and parses its header.
func Open(path string) (*BinaryCourse, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	b := &BinaryCourse{f: f}
	if err := b.readHeader(); err != nil {
		f.Close()
		return nil, err
	}
	return b, nil
}

// Create writes a new .course file with the given manifest JSON (signed).
// The caller must call WritePayload and Close to finalize.
func Create(path string, manifestJSON []byte) (*BinaryCourse, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}

	b := &BinaryCourse{
		f:       f,
		version: version1,
		flags:   flagGzip,
		rawMan:  manifestJSON,
	}

	// Write header placeholder — we'll seek back after knowing payload.
	hdr := make([]byte, headerLen)
	copy(hdr[0:4], magic)
	binary.LittleEndian.PutUint16(hdr[4:6], b.version)
	binary.LittleEndian.PutUint16(hdr[6:8], b.flags)
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(len(b.rawMan)))
	if _, err := f.Write(hdr); err != nil {
		f.Close()
		return nil, fmt.Errorf("write header: %w", err)
	}
	if _, err := f.Write(b.rawMan); err != nil {
		f.Close()
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	return b, nil
}

// Manifest returns the raw manifest JSON bytes.
func (b *BinaryCourse) Manifest() []byte { return b.rawMan }

// Version returns the format version.
func (b *BinaryCourse) Version() uint16 { return b.version }

// IsCompressed reports whether the payload is gzip-compressed.
func (b *BinaryCourse) IsCompressed() bool { return b.flags&flagGzip != 0 }

// WritePayload encrypts the given tar.gz data and appends it to the file.
// The file position is advanced past the payload.
func (b *BinaryCourse) WritePayload(encrypted []byte) error {
	if _, err := b.f.Write(encrypted); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

// Close closes the underlying file.
func (b *BinaryCourse) Close() error { return b.f.Close() }

// ReadPayload reads and decrypts the payload using the given key.
func (b *BinaryCourse) ReadPayload(key [32]byte, courseID string) ([]byte, error) {
	payload, err := io.ReadAll(b.f)
	if err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}
	return Decrypt(payload, key[:], courseID)
}

// DecodeManifest parses the stored manifest and verifies its signature.
func (b *BinaryCourse) DecodeManifest(pub ed25519.PublicKey, sig string) (*CourseManifest, error) {
	return VerifyManifest(b.rawMan, sig, pub)
}

// ── helpers ──

func (b *BinaryCourse) readHeader() error {
	hdr := make([]byte, headerLen)
	if _, err := io.ReadFull(b.f, hdr); err != nil {
		return fmt.Errorf("read header: %w", err)
	}
	if string(hdr[0:4]) != magic {
		return fmt.Errorf("not a .course file: bad magic %x", hdr[0:4])
	}
	b.version = binary.LittleEndian.Uint16(hdr[4:6])
	b.flags = binary.LittleEndian.Uint16(hdr[6:8])
	if b.version != version1 {
		return fmt.Errorf("unsupported version %d", b.version)
	}
	manLen := binary.LittleEndian.Uint32(hdr[8:12])
	b.rawMan = make([]byte, manLen)
	if _, err := io.ReadFull(b.f, b.rawMan); err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	return nil
}

// ── tar helpers (for coursepack and loader) ──

// ExtractTarGz extracts a gzip-compressed tar archive to dir.
func ExtractTarGz(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		target := filepath.Join(dir, hdr.Name)
		// Path traversal check — reject names that escape dir.
		cleanDir := filepath.Clean(dir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(target), cleanDir) {
			return fmt.Errorf("path traversal rejected: %q", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			f, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write %s: %w", target, err)
			}
			f.Close()
		}
	}
	return nil
}

// BuildTarGz creates a gzip-compressed tar archive of dir contents,
// writing to w. Non-regular files and non-directories are skipped.
func BuildTarGz(w io.Writer, dir string) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
}

// ── internal signature helpers ──

func encodeSig(sig []byte) string {
	var b strings.Builder
	for _, x := range sig {
		b.WriteByte(hexTable[x>>4])
		b.WriteByte(hexTable[x&0x0f])
	}
	return b.String()
}

func decodeSig(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd hex length")
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi, ok := hexVal(s[i])
		if !ok {
			return nil, fmt.Errorf("invalid hex: %c", s[i])
		}
		lo, ok := hexVal(s[i+1])
		if !ok {
			return nil, fmt.Errorf("invalid hex: %c", s[i+1])
		}
		b[i/2] = hi<<4 | lo
	}
	return b, nil
}

const hexTable = "0123456789abcdef"

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

var _ = json.Marshal // suppress unused import; json is used in manifest.go
