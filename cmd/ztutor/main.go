package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"ztutor/internal/db"
	"ztutor/internal/lesson"
	"ztutor/internal/license"
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
	flag.Parse()
	logutil.SetVerbose(*verbose)

	logutil.Info("%s", version.String())
	logutil.Debug("verbose logging enabled")
	tui.SetNativeGamepadEnabled(os.Getenv("ZTUTOR_GAMEPAD") != "0")

	dataDir := envOrDefault("ZTUTOR_DATA_DIR", defaultDataDir())
	logutil.Debug("data dir: %s", dataDir)
	if dataDir != "." {
		if err := os.MkdirAll(dataDir, 0700); err != nil {
			logutil.Warn("cannot create data dir %s: %v", dataDir, err)
		}
	}

	dbPath := envOrDefault("ZTUTOR_DB", filepath.Join(dataDir, "ztutor.db"))
	logutil.Debug("db path: %s", dbPath)
	database, err := db.Open(dbPath)
	if err != nil {
		logutil.Fatal("db: %v", err)
	}
	defer database.Close()

	username := currentUser()

	configureLicensePublicKey()

	if _, err := database.GetUser(username); err != nil {
		if err := database.CreateUser(username, "", db.RoleStudent); err != nil {
			logutil.Fatal("create user: %v", err)
		}
	}

	lic := loadStartupLicense(username, database, dataDir)

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

	if warnings := sandbox.HealthCheck(); len(warnings) > 0 {
		logutil.Warn("sandbox toolchain issues detected:")
		for _, w := range warnings {
			logutil.Warn("  - %s", w)
		}
	}

	coursesDir := envOrDefault("ZTUTOR_COURSES_DIR", defaultCoursesDir(dataDir))
	lessonsDir := envOrDefault("ZTUTOR_LESSONS_DIR", "")
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

	var resumeLesson *lesson.Lesson
	for {
		var pendingGDB *sandbox.DebugBuild
		var pendingLesson lesson.Lesson

		app := tui.NewApp(
			username,
			coursesDir,
			lessonsDir,
			database,
			lic,
			width, height,
			keymap,
			func(build *sandbox.DebugBuild, l lesson.Lesson) {
				pendingGDB = build
				pendingLesson = l
			},
			resumeLesson,
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
			break
		}

		if pendingGDB == nil {
			break
		}

		runLocalGDB(pendingGDB)
		pendingGDB.Close()
		resumeLesson = &pendingLesson
	}
}

func configureLicensePublicKey() {
	pubKeyHex := os.Getenv("ZTUTOR_LICENSE_PUBKEY")
	if pubKeyHex == "" {
		return
	}
	b, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		logutil.Warn("invalid ZTUTOR_LICENSE_PUBKEY: %v", err)
		return
	}
	license.SetPublicKey(ed25519.PublicKey(b))
}

func loadStartupLicense(username string, database *db.DB, dataDir string) *license.State {
	licenseFile := discoverLicenseFile(dataDir)
	if licenseFile == "" {
		return nil
	}
	data, err := os.ReadFile(licenseFile)
	if err != nil {
		logutil.Warn("read license file %s: %v", licenseFile, err)
		return nil
	}
	state, info, err := license.CheckData(data)
	if err != nil {
		logutil.Warn("license file %s: %v", licenseFile, err)
		return nil
	}
	if info.IsPersonal() {
		if err := database.RedeemPersonalLicense(username, info, data); err != nil {
			logutil.Warn("redeem personal license from %s: %v", licenseFile, err)
			return nil
		}
	}
	logutil.Info("license loaded from %s", licenseFile)
	return state
}

func discoverLicenseFile(dataDir string) string {
	if explicit := os.Getenv("ZTUTOR_LICENSE_FILE"); explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
		return ""
	}
	candidates := []string{"license.key"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "license.key"))
	}
	candidates = append(candidates, filepath.Join(dataDir, "license.key"))
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func runLocalGDB(build *sandbox.DebugBuild) {
	lang := sandbox.GetLanguage("c")
	if lang == nil {
		fmt.Fprintln(os.Stderr, "gdb: C toolchain not found")
		return
	}
	args := []string{"-q", "-iex", "set debuginfod enabled off"}
	if len(build.RuntimeArgs) > 0 {
		args = append(args, "--args", build.BinaryPath)
		args = append(args, build.RuntimeArgs...)
	} else {
		args = append(args, build.BinaryPath)
	}
	cmd := exec.Command(lang.DebuggerPath(), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logutil.Error("gdb: %v", err)
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

func defaultCoursesDir(dataDir string) string {
	installed := filepath.Join(dataDir, "courses")
	if info, err := os.Stat(installed); err == nil && info.IsDir() {
		return installed
	}
	return "./courses"
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
