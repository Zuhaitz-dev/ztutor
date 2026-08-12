package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"syscall"

	"ztutor/internal/apputil"
	"ztutor/internal/config"
	"ztutor/internal/db"
	"ztutor/internal/logutil"
	"ztutor/internal/remote"
	"ztutor/internal/sandbox"
	"ztutor/internal/ssh"
	"ztutor/internal/tui"
	"ztutor/internal/version"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

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
		apputil.CheckAndPrintUpdate()
		return
	}

	logutil.SetVerbose(*verbose)

	logutil.Info("%s", version.String())
	logutil.Debug("verbose logging enabled")

	dataDir := apputil.EnvOrDefault("ZTUTOR_DATA_DIR", apputil.DefaultDataDir())
	configPath := apputil.EnvOrDefault("ZTUTOR_CONFIG", "./ztutor.json")
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
	lessonsDir := apputil.EnvOrDefault("ZTUTOR_LESSONS_DIR", "./lessons")

	logutil.Debug("host key: %s", hostKey)
	logutil.Debug("db path: %s", dbPath)
	logutil.Debug("courses dir: %s", coursesDir)
	logutil.Debug("lessons dir: %s", lessonsDir)

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
		SetupToken:       setupToken,
		MaxConns:         cfg.SSH.MaxConns,
	}, &ssh.TUIProvider{
		NewStudentApp: func(username, coursesDir, lessonsDir string, db *db.DB, width, height int, keymap string) tea.Model {
			return tui.NewApp(username, coursesDir, lessonsDir, db, width, height, keymap)
		},
		NewAdminApp: func(username string, db *db.DB, lessonsDir, coursesDir, achievementsFile string, width, height int) tea.Model {
			return tui.NewAdminApp(username, db, lessonsDir, coursesDir, achievementsFile, width, height)
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
		runLocalControl(srv, dbPath, lessonsDir, coursesDir, achievementsFile, cfg.Keymap)
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

func runLocalControl(srv *ssh.Server, dbPath, lessonsDir, coursesDir, achievementsFile, keymap string) {
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

	logutil.Info("SSH server running at %s - admin control opening", srv.ListenAddr())

	username := adminUsername()
	for {
		adminApp := tui.NewAdminApp(username, localDB, lessonsDir, coursesDir, achievementsFile, width, height)
		if _, err := tea.NewProgram(adminApp, tea.WithAltScreen(), tea.WithoutCatchPanics()).Run(); err != nil {
			logutil.Error("local admin TUI: %v", err)
			return
		}
		if !adminApp.WantsRelaunch() {
			return
		}
		studentApp := tui.NewApp(username, coursesDir, lessonsDir, localDB, width, height, keymap)
		if _, err := tea.NewProgram(studentApp, tea.WithAltScreen(), tea.WithoutCatchPanics()).Run(); err != nil {
			logutil.Error("local student TUI: %v", err)
			return
		}
		if !studentApp.WantsRelaunch() {
			return
		}
	}
}

func adminUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "admin"
}
