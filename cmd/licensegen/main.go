package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"ztutor/internal/license"
	"ztutor/internal/logutil"
)

type keyFile struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

func main() {
	generateKeys := flag.Bool("gen-key", false, "generate a new keypair")
	keyPairPath := flag.String("key-file", "ztutor_license_keys.json", "path to keypair file")
	licensee := flag.String("licensee", "", "licensee name (company/school)")
	username := flag.String("username", "", "personal license username binding")
	email := flag.String("email", "", "personal license email binding")
	licenseID := flag.String("license-id", "", "unique personal license id")
	maxStudents := flag.Int("max-students", 50, "maximum student count")
	expiresAfter := flag.String("expires", "", "expiry duration (e.g. 365d, 0 for no expiry)")
	features := flag.String("features", "multi_user,admin_ui,interviews", "comma-separated feature list")
	courses := flag.String("courses", "", "comma-separated course IDs to unlock")
	courseKeyHex := flag.String("course-key", "", "hex-encoded 32-byte AES key for course encryption")
	generateCourseKey := flag.Bool("gen-course-key", false, "generate a random course encryption key")
	flag.Parse()

	if *generateKeys {
		pub, priv, err := license.GenerateKeyPair()
		if err != nil {
			logutil.Fatal("generate keys: %v", err)
		}
		kf := keyFile{
			PublicKey:  fmt.Sprintf("%x", pub),
			PrivateKey: fmt.Sprintf("%x", priv),
		}
		data, _ := json.MarshalIndent(kf, "", "  ")
		os.WriteFile(*keyPairPath, data, 0600)
		fmt.Printf("keys written to %s\n", *keyPairPath)
		return
	}

	if *generateCourseKey {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			logutil.Fatal("generate course key: %v", err)
		}
		fmt.Printf("course_key: %x\n", key)
		return
	}

	if *licensee == "" {
		logutil.Fatal("--licensee required")
	}

	data, err := os.ReadFile(*keyPairPath)
	if err != nil {
		logutil.Fatal("read key file: %v (use --gen-key first)", err)
	}
	var kf keyFile
	if err := json.Unmarshal(data, &kf); err != nil {
		logutil.Fatal("parse key file: %v", err)
	}

	privBytes, err := hexDecode(kf.PrivateKey)
	if err != nil {
		logutil.Fatal("decode private key: %v", err)
	}
	priv := ed25519.PrivateKey(privBytes)

	var expiresAt string
	if *expiresAfter != "" && *expiresAfter != "0" {
		dur := parseDuration(*expiresAfter)
		if dur > 0 {
			expiresAt = time.Now().Add(dur).Format(time.RFC3339)
		}
	}

	featList := parseFeatures(*features)
	courseList := parseFeatures(*courses)

	if (*username != "" || *email != "") && *licenseID == "" {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			logutil.Fatal("generate license id: %v", err)
		}
		*licenseID = fmt.Sprintf("%x", buf)
	}

	info := license.Info{
		Licensee:        *licensee,
		LicenseID:       *licenseID,
		Username:        *username,
		Email:           *email,
		MaxStudents:     *maxStudents,
		Features:        featList,
		UnlockedCourses: courseList,
		CourseKey:       *courseKeyHex,
		ExpiresAt:       expiresAt,
		IssuedAt:        time.Now().Format(time.RFC3339),
	}

	signed, err := license.Sign(priv, info)
	if err != nil {
		logutil.Fatal("sign: %v", err)
	}

	os.WriteFile("license_"+sanitize(*licensee)+".key", signed, 0644)
	fmt.Printf("license written to license_%s.key\n", sanitize(*licensee))
	fmt.Printf("licensee: %s\n", *licensee)
	if *username != "" {
		fmt.Printf("username: %s\n", *username)
	}
	if *email != "" {
		fmt.Printf("email: %s\n", *email)
	}
	if *licenseID != "" {
		fmt.Printf("license_id: %s\n", *licenseID)
	}
	fmt.Printf("students: %d\n", *maxStudents)
	fmt.Printf("features: %v\n", featList)
	if len(courseList) > 0 {
		fmt.Printf("courses: %v\n", courseList)
	}
	if expiresAt != "" {
		fmt.Printf("expires: %s\n", expiresAt)
	} else {
		fmt.Println("expires: never")
	}
}

func hexDecode(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var v byte
		for j := 0; j < 2; j++ {
			c := s[i+j]
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= c - '0'
			case c >= 'a' && c <= 'f':
				v |= c - 'a' + 10
			case c >= 'A' && c <= 'F':
				v |= c - 'A' + 10
			default:
				return nil, fmt.Errorf("invalid hex char: %c", c)
			}
		}
		b[i/2] = v
	}
	return b, nil
}

func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	var v int
	var unit string
	fmt.Sscanf(s, "%d%s", &v, &unit)
	switch unit {
	case "y", "yr", "year", "years":
		return time.Duration(v) * 365 * 24 * time.Hour
	case "mo", "month", "months":
		return time.Duration(v) * 30 * 24 * time.Hour
	case "d", "day", "days":
		return time.Duration(v) * 24 * time.Hour
	default:
		return 0
	}
}

func parseFeatures(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var features []string
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			features = append(features, f)
		}
	}
	return features
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, strings.ToLower(s))
}
