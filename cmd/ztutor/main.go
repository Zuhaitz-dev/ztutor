package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ztutor/internal/apputil"
	"ztutor/internal/db"
	"ztutor/internal/logutil"
	"ztutor/internal/remote"
	"ztutor/internal/sandbox"
	"ztutor/internal/tui"
	"ztutor/internal/version"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

func main() {
	verbose := flag.Bool("v", false, "enable verbose debug logging")
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
	tui.SetNativeGamepadEnabled(os.Getenv("ZTUTOR_GAMEPAD") != "0")

	dataDir := apputil.EnvOrDefault("ZTUTOR_DATA_DIR", apputil.DefaultDataDir())
	logutil.Debug("data dir: %s", dataDir)
	if dataDir != "." {
		if err := os.MkdirAll(dataDir, 0700); err != nil {
			logutil.Warn("cannot create data dir %s: %v", dataDir, err)
		}
	}

	dbPath := apputil.EnvOrDefault("ZTUTOR_DB", filepath.Join(dataDir, "ztutor.db"))
	logutil.Debug("db path: %s", dbPath)
	database, err := db.Open(dbPath)
	if err != nil {
		logutil.Fatal("db: %v", err)
	}
	defer database.Close()

	username := currentUser()

	if _, err := database.GetUser(username); err != nil {
		if err := database.CreateUser(username, "", db.RoleStudent); err != nil {
			logutil.Fatal("create user: %v", err)
		}
	}

	// Configure remote execution server: env vars take priority over DB user settings.
	execAddr := os.Getenv("ZTUTOR_EXEC_ADDR")
	execToken := os.Getenv("ZTUTOR_EXEC_TOKEN")
	useTLS := os.Getenv("ZTUTOR_EXEC_TLS") == "1"
	if execAddr == "" {
		execAddr, _ = database.GetUserSetting(username, "exec_addr")
		execToken, _ = database.GetUserSetting(username, "exec_token")
		tlsStr, _ := database.GetUserSetting(username, "exec_tls")
		useTLS = tlsStr == "1"
	}
	if execAddr != "" {
		sandbox.SetDefaultExecutor(&sandbox.HybridExecutor{
			Local:  sandbox.DefaultExecutor(),
			Remote: remote.NewClientWithToken(execAddr, execToken, useTLS),
		})
		logutil.Info("hybrid executor — remote at %s (tls: %v)", execAddr, useTLS)
	}

	sandbox.ApplyLimitsFromEnv()

	if warnings := sandbox.HealthCheck(); len(warnings) > 0 {
		logutil.Warn("sandbox toolchain issues detected:")
		for _, w := range warnings {
			logutil.Warn("  - %s", w)
		}
	}

	coursesDir := apputil.EnvOrDefault("ZTUTOR_COURSES_DIR", defaultCoursesDir(dataDir))
	lessonsDir := apputil.EnvOrDefault("ZTUTOR_LESSONS_DIR", "")
	if lessonsDir == "" {
		lessonsDir = "./courses" // modern default: courses dir contains the lessons
	}
	logutil.Debug("courses dir: %s", coursesDir)
	logutil.Debug("lessons dir: %s", lessonsDir)

	width, height := 80, 24
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 && h > 0 {
		width, height = w, h
	}

	keymap, _ := database.GetUserSetting(username, "keymap")
	if keymap == "" {
		keymap = "default"
	}

	app := tui.NewApp(
		username,
		coursesDir,
		lessonsDir,
		database,
		width, height,
		keymap,
	)

	if _, err := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithoutCatchPanics(),
	).Run(); err != nil {
		logutil.Error("tui: %v", err)
	}

	if app.LaunchAdmin {
		fmt.Fprintln(os.Stderr, "admin dashboard is only available via ztutord")
	}
}

func currentUser() string {
	if u := os.Getenv("ZTUTOR_USER"); u != "" {
		return u
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("LOGNAME"); u != "" {
		return u
	}
	return "user"
}

func defaultCoursesDir(dataDir string) string {
	installed := filepath.Join(dataDir, "courses")
	if info, err := os.Stat(installed); err == nil && info.IsDir() {
		return installed
	}
	return "./courses"
}
