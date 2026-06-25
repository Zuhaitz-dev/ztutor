package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ztutor/internal/config"
	"ztutor/internal/db"
	"ztutor/internal/lesson"
	"ztutor/internal/license"
	"ztutor/internal/logutil"
	"ztutor/internal/remote"
	"ztutor/internal/sandbox"
	"ztutor/internal/ssh"
	"ztutor/internal/tui"
	"ztutor/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

func defaultDataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "ztutor")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".local", "share", "ztutor")
}

func main() {
	verbose := flag.Bool("v", false, "enable verbose debug logging")
	flag.Parse()
	logutil.SetVerbose(*verbose)

	logutil.Info("%s", version.String())
	logutil.Debug("verbose logging enabled")

	dataDir := envOrDefault("ZTUTOR_DATA_DIR", defaultDataDir())
	configPath := envOrDefault("ZTUTOR_CONFIG", "./ztutor.json")
	logutil.Debug("data dir: %s", dataDir)
	logutil.Debug("config path: %s", configPath)

	if dataDir != "." {
		if err := os.MkdirAll(dataDir, 0700); err != nil {
			logutil.Warn("cannot create data dir %s: %v", dataDir, err)
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logutil.Warn("config: %v (using defaults)", err)
	}

	hostKey := cfg.SSH.HostKey
	if hostKey == "" || hostKey == "ztutor_host_key" {
		hostKey = filepath.Join(dataDir, "ztutor_host_key")
	}
	dbPath := cfg.DB.Path
	if dbPath == "" || dbPath == "ztutor.db" {
		dbPath = filepath.Join(dataDir, "ztutor.db")
	}
	coursesDir := cfg.CoursesDir
	if coursesDir == "" {
		coursesDir = "./courses"
	}
	lessonsDir := envOrDefault("ZTUTOR_LESSONS_DIR", "./lessons")
	pubKeyHex := os.Getenv("ZTUTOR_LICENSE_PUBKEY")
	if pubKeyHex != "" {
		if b, err := hex.DecodeString(pubKeyHex); err == nil {
			license.SetPublicKey(ed25519.PublicKey(b))
		}
	}

	licenseFile := envOrDefault("ZTUTOR_LICENSE_FILE", cfg.License.File)
	if licenseFile == "" {
		licenseFile = filepath.Join(dataDir, "license.key")
	}
	logutil.Debug("host key: %s", hostKey)
	logutil.Debug("db path: %s", dbPath)
	logutil.Debug("courses dir: %s", coursesDir)
	logutil.Debug("lessons dir: %s", lessonsDir)
	logutil.Debug("license file: %s", licenseFile)

	lic, _ := license.Check(licenseFile)

	achievementsFile := filepath.Join(filepath.Dir(lessonsDir), "custom_achievements.yaml")

	srv, err := ssh.New(ssh.Config{
		HostKey:          hostKey,
		CoursesDir:       coursesDir,
		LessonsDir:       lessonsDir,
		AchievementsFile: achievementsFile,
		DBPath:           dbPath,
		Addr:             cfg.SSH.Addr,
		Keymap:           cfg.Keymap,
		License:          lic,
		SetupToken:       db.GenerateSetupToken(),
		MaxConns:         cfg.SSH.MaxConns,
	}, &ssh.TUIProvider{
		NewStudentApp: func(username, coursesDir, lessonsDir string, db *db.DB, license *license.State, width, height int, keymap string, launchGDB func(*sandbox.DebugBuild, lesson.Lesson), startLesson *lesson.Lesson) tea.Model {
			return tui.NewApp(username, coursesDir, lessonsDir, db, license, width, height, keymap, launchGDB, startLesson)
		},
		NewAdminApp: func(username string, db *db.DB, license *license.State, lessonsDir, coursesDir, achievementsFile string, width, height int) tea.Model {
			return tui.NewAdminApp(username, db, license, lessonsDir, coursesDir, achievementsFile, width, height)
		},
		LoadAchievements: tui.LoadCustomAchievements,
	})
	if err != nil {
		logutil.Fatal("init: %v", err)
	}

	if warnings := sandbox.HealthCheck(); len(warnings) > 0 {
		logutil.Warn("sandbox toolchain issues detected:")
		for _, w := range warnings {
			logutil.Warn("  - %s", w)
		}
	}

	if cfg.Exec.Addr != "" {
		logutil.Debug("exec server enabled at %s (tls: %v, max_conns: %d)", cfg.Exec.Addr, cfg.Exec.TLS, cfg.Exec.MaxConns)
		go func() {
			if err := remote.ListenAndServeTLS(cfg.Exec.Addr, cfg.Exec.TLS, cfg.Exec.CertFile, cfg.Exec.KeyFile, cfg.Exec.MaxConns); err != nil {
				logutil.Error("exec server: %v", err)
			}
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logutil.Info("received %s, shutting down...", sig)
		cancel()
	}()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logutil.Info("stopping server...")
		srv.Shutdown(ctx)
	case err := <-errCh:
		if err != nil {
			logutil.Fatal("server: %v", err)
		}
	}

	if err := srv.Close(); err != nil {
		logutil.Error("close: %v", err)
	}
	logutil.Info("ztutor stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
