package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"os/user"
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
	"ztutor/internal/update"
	"ztutor/internal/version"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
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
	localMode := flag.Bool("local", false, "open local setup or the admin dashboard in this terminal while the SSH server runs")
	showVersion := flag.Bool("version", false, "print version and exit")
	checkUpdate := flag.Bool("check-update", false, "check for a newer release and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	if *checkUpdate {
		checkAndPrintUpdate()
		return
	}

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
	cfg.ApplySandboxLimits()

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

	setupToken := db.GenerateSetupToken()
	srv, err := ssh.New(ssh.Config{
		HostKey:          hostKey,
		CoursesDir:       coursesDir,
		LessonsDir:       lessonsDir,
		AchievementsFile: achievementsFile,
		DBPath:           dbPath,
		Addr:             cfg.SSH.Addr,
		Keymap:           cfg.Keymap,
		License:          lic,
		SetupToken:       setupToken,
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logutil.Info("received %s, shutting down...", sig)
		cancel()
	}()

	if cfg.Exec.Addr != "" {
		logutil.Debug("exec server enabled at %s (tls: %v, max_conns: %d)", cfg.Exec.Addr, cfg.Exec.TLS, cfg.Exec.MaxConns)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logutil.Error("exec server panic: %v", r)
				}
			}()
			if err := remote.ListenAndServeTLSContext(ctx, cfg.Exec.Addr, cfg.Exec.TLS, cfg.Exec.CertFile, cfg.Exec.KeyFile, cfg.Exec.MaxConns); err != nil {
				if ctx.Err() == nil {
					logutil.Error("exec server: %v", err)
				}
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	if *localMode {
		select {
		case <-srv.Ready():
		case err := <-errCh:
			logutil.Fatal("server: %v", err)
		}
		runLocalControl(srv, dbPath, lic, lessonsDir, coursesDir, achievementsFile, cfg.Keymap)
		srv.Shutdown(ctx)
	} else {
		select {
		case <-ctx.Done():
			logutil.Info("stopping server...")
			srv.Shutdown(ctx)
		case err := <-errCh:
			if err != nil {
				logutil.Fatal("server: %v", err)
			}
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

func runLocalControl(srv *ssh.Server, dbPath string, lic *license.State, lessonsDir, coursesDir, achievementsFile, keymap string) {
	localDB, err := db.Open(dbPath)
	if err != nil {
		logutil.Error("local admin: open db: %v", err)
		return
	}
	defer localDB.Close()

	width, height := 120, 40
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 && h > 0 {
		width, height = w, h
	}

	if lic != nil && lic.HasAdminUI {
		logutil.Info("SSH server running at %s - business control opening", srv.ListenAddr())
	} else {
		logutil.Info("SSH server running at %s - learner setup opening", srv.ListenAddr())
	}

	username := adminUsername()
	showAdmin := true
	for {
		if showAdmin {
			app := tui.NewAdminApp(username, localDB, lic, lessonsDir, coursesDir, achievementsFile, width, height)
			if _, err := tea.NewProgram(app, tea.WithAltScreen(), tea.WithoutCatchPanics()).Run(); err != nil {
				logutil.Error("local admin TUI: %v", err)
				return
			}
			if !app.WantsRelaunch() {
				return
			}
			showAdmin = false
		} else {
			app := tui.NewApp(username, coursesDir, lessonsDir, localDB, lic, width, height, keymap, nil, nil)
			if _, err := tea.NewProgram(app, tea.WithAltScreen(), tea.WithoutCatchPanics()).Run(); err != nil {
				logutil.Error("local student TUI: %v", err)
				return
			}
			if !app.WantsRelaunch() {
				return
			}
			showAdmin = true
		}
	}
}

func adminUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "admin"
}

func checkAndPrintUpdate() {
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
