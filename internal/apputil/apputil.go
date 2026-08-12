// Package apputil holds helpers shared by the ztutor and ztutord entrypoints.
package apputil

import (
	"fmt"
	"os"
	"path/filepath"

	"ztutor/internal/update"
	"ztutor/internal/version"
)

// DefaultDataDir returns the base directory for the database and host key.
func DefaultDataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "ztutor")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".local", "share", "ztutor")
}

// EnvOrDefault returns the value of the environment variable key, or fallback
// when the variable is unset or empty.
func EnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// CheckAndPrintUpdate checks GitHub for a newer release and prints the result.
// Exits non-zero when the check itself fails.
func CheckAndPrintUpdate() {
	info, err := update.CheckLatest(version.Version, nil, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
		os.Exit(1)
	}
	if info == nil {
		fmt.Printf("ztutor %s is up to date.\n", version.Version)
		return
	}
	fmt.Printf("New version %s available\n", info.Version)
	fmt.Printf("  Released: %s\n", info.PublishedAt)
	fmt.Printf("  Download: %s\n", info.ReleaseURL)
	fmt.Printf("\nRun update-ztutor.sh to install automatically.\n")
}
