package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
)

type Config struct {
	Keymap string `json:"keymap"`

	SSH struct {
		Addr    string `json:"addr"`
		HostKey string `json:"host_key"`

		// MaxConns caps concurrent SSH connections. 0 = unlimited.
		MaxConns int `json:"max_conns"`
	} `json:"ssh"`

	// Exec configures the remote-execution TCP endpoint. When Addr is non-empty,
	// ztutord listens on that address for sandbox execution requests from
	// standalone ztutor clients.
	Exec struct {
		Addr string `json:"addr"`

		// TLS enables Transport Layer Security. When set, both CertFile and
		// KeyFile must be provided. The client must connect with TLS and verify
		// the server certificate against the system CA pool or a custom CA file.
		TLS      bool   `json:"tls"`
		CertFile string `json:"cert_file"`
		KeyFile  string `json:"key_file"`
		CAFile   string `json:"ca_file,omitempty"`

		// MaxConns caps concurrent exec connections. 0 = unlimited.
		MaxConns int `json:"max_conns"`
	} `json:"exec"`

	DB struct {
		Path string `json:"path"`
	} `json:"db"`

	CoursesDir string `json:"courses_dir"`

	License struct {
		File string `json:"file"`
	} `json:"license"`
}

func Defaults() Config {
	cfg := Config{}
	cfg.Keymap = "default"
	cfg.SSH.Addr = ":2222"
	cfg.SSH.HostKey = "ztutor_host_key"
	cfg.SSH.MaxConns = 200
	cfg.DB.Path = "ztutor.db"
	cfg.CoursesDir = "./courses"
	return cfg
}

func Load(path string) (Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}

	// Detect unknown keys before unmarshaling.
	if unknowns := detectUnknownKeys(data, cfg); len(unknowns) > 0 {
		return cfg, fmt.Errorf("unknown config keys: %s", strings.Join(unknowns, ", "))
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return cfg, fmt.Errorf("config: %w", err)
		}
		return cfg, err
	}
	return cfg, nil
}

// detectUnknownKeys compares JSON keys against known struct field tags.
func detectUnknownKeys(data []byte, cfg Config) []string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil // malformed JSON, let the real decoder report it
	}

	known := knownJSONKeys(reflect.TypeOf(cfg))
	var unknown []string
	for k := range raw {
		if !known[k] {
			unknown = append(unknown, fmt.Sprintf("%q", k))
		}
	}
	return unknown
}

// knownJSONKeys returns the set of top-level JSON field names for the given type.
func knownJSONKeys(t reflect.Type) map[string]bool {
	keys := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		keys[name] = true
	}
	return keys
}
