package ssh

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"ztutor/internal/db"
	"ztutor/internal/lesson"
	"ztutor/internal/license"
	"ztutor/internal/sandbox"

	tea "github.com/charmbracelet/bubbletea"
	gossh "golang.org/x/crypto/ssh"
)

// dummyModel quits immediately — used in most tests where TUI content is irrelevant.
type dummyModel struct{}

func (m dummyModel) Init() tea.Cmd                           { return tea.Quit }
func (m dummyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, tea.Quit }
func (m dummyModel) View() string                            { return "" }

// holdModel stays alive until the SSH channel closes — used in MaxConns tests.
type holdModel struct{}

func (m holdModel) Init() tea.Cmd                           { return nil }
func (m holdModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m holdModel) View() string                            { return "" }

func newTestTUI() *TUIProvider {
	return &TUIProvider{
		NewStudentApp: func(username, coursesDir, lessonsDir string, db *db.DB, license *license.State, width, height int, keymap string, launchGDB func(*sandbox.DebugBuild, lesson.Lesson), startLesson *lesson.Lesson) tea.Model {
			return dummyModel{}
		},
		NewAdminApp: func(username string, db *db.DB, license *license.State, lessonsDir, coursesDir, achievementsFile string, width, height int) tea.Model {
			return dummyModel{}
		},
		LoadAchievements: func(path string) {},
	}
}

func requireTCPListener(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("tcp listening not permitted in this environment: %v", err)
	}
	ln.Close()
}

// waitForListener polls until the server has bound a port, then returns the address string.
func waitForListener(t *testing.T, srv *Server) string {
	t.Helper()
	for i := 0; i < 200; i++ {
		if srv.listener != nil {
			return srv.listener.Addr().String()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("server did not start listening in time")
	return ""
}

// dial connects via SSH using password auth.
func dial(addr, user, pass string) (*gossh.Client, error) {
	cfg := &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.Password(pass)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	return gossh.Dial("tcp", addr, cfg)
}

func TestServerCreation(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	hostKeyPath := filepath.Join(dir, "host_key")

	cfg := Config{
		HostKey: hostKeyPath,
		DBPath:  dbPath,
		Addr:    ":0",
		Keymap:  "default",
	}

	srv, err := New(cfg, newTestTUI())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()

	if srv.db == nil {
		t.Error("db should not be nil after New")
	}
	if srv.stopCh == nil {
		t.Error("stopCh should not be nil after New")
	}
	if srv.listener != nil {
		t.Error("listener should be nil before ListenAndServe")
	}
}

func TestServerSSHConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireTCPListener(t)

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	hostKeyPath := filepath.Join(dir, "host_key")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	const pw = "testpass"
	if err := database.CreateUser("testuser", pw, db.RoleStudent); err != nil {
		database.Close()
		t.Fatalf("CreateUser: %v", err)
	}
	database.Close()

	cfg := Config{
		HostKey:    hostKeyPath,
		DBPath:     dbPath,
		Addr:       ":0",
		Keymap:     "default",
		SetupToken: db.GenerateSetupToken(),
	}
	srv, err := New(cfg, newTestTUI())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	addr := waitForListener(t, srv)

	client, err := dial(addr, "testuser", pw)
	if err != nil {
		srv.Shutdown(context.TODO())
		srv.Close()
		t.Fatalf("ssh dial: %v", err)
	}
	client.Close()
	time.Sleep(50 * time.Millisecond)

	srv.Shutdown(context.TODO())
	select {
	case err := <-errCh:
		if err != nil {
			t.Logf("ListenAndServe returned: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("ListenAndServe did not return in time")
	}
	srv.Close()
}

func TestServerSSHBadPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireTCPListener(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, _ := db.Open(dbPath)
	database.CreateUser("testuser", "correctpass", db.RoleStudent) //nolint:errcheck
	database.Close()

	srv, _ := New(Config{HostKey: filepath.Join(dir, "host_key"), DBPath: dbPath, Addr: ":0", Keymap: "default"}, newTestTUI())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	addr := waitForListener(t, srv)

	if _, err := dial(addr, "testuser", "wrongpass"); err == nil {
		t.Error("expected auth failure with wrong password")
	}
	srv.Shutdown(context.TODO())
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Error("ListenAndServe did not return in time")
	}
	srv.Close()
}

func TestServerSSHBadUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireTCPListener(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, _ := db.Open(dbPath)
	database.CreateUser("realuser", "testpass", db.RoleStudent) //nolint:errcheck
	database.Close()

	srv, _ := New(Config{HostKey: filepath.Join(dir, "host_key"), DBPath: dbPath, Addr: ":0", Keymap: "default"}, newTestTUI())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	addr := waitForListener(t, srv)

	if _, err := dial(addr, "nonexistent", "testpass"); err == nil {
		t.Error("expected auth failure with unknown user")
	}
	srv.Shutdown(context.TODO())
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Error("ListenAndServe did not return in time")
	}
	srv.Close()
}

// ── New tests ─────────────────────────────────────────────────────────────────

func TestServer_SetupToken_FirstUser(t *testing.T) {
	// When the DB has no users, the setup token acts as the password for any username.
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireTCPListener(t)
	dir := t.TempDir()
	token := db.GenerateSetupToken()
	srv, err := New(Config{
		HostKey: filepath.Join(dir, "host_key"), DBPath: filepath.Join(dir, "test.db"),
		Addr: ":0", Keymap: "default", SetupToken: token,
	}, newTestTUI())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go srv.ListenAndServe() //nolint:errcheck
	addr := waitForListener(t, srv)
	defer func() { srv.Shutdown(context.TODO()); srv.Close() }()

	// Correct token — any username should be accepted.
	client, err := dial(addr, "firstadmin", token)
	if err != nil {
		t.Fatalf("setup token auth should succeed: %v", err)
	}
	client.Close()

	// Wrong token — must be rejected.
	if _, err = dial(addr, "firstadmin", "badtoken"); err == nil {
		t.Fatal("wrong setup token should be rejected")
	}
}

// startSession opens an SSH session channel, requests a PTY and shell, then
// waits briefly for the TUI constructor to fire before returning the client.
func startSession(t *testing.T, addr, user, pass string) *gossh.Client {
	t.Helper()
	client, err := dial(addr, user, pass)
	if err != nil {
		t.Fatalf("dial %s: %v", user, err)
	}
	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		t.Fatalf("new session: %v", err)
	}
	if err := sess.RequestPty("xterm", 24, 80, gossh.TerminalModes{}); err != nil {
		sess.Close()
		client.Close()
		t.Fatalf("RequestPty: %v", err)
	}
	// Shell request triggers the TUI constructor; ignore error — dummyModel
	// immediately quits so the server may close the channel first.
	_ = sess.Shell()
	time.Sleep(150 * time.Millisecond) // let TUI constructor run
	sess.Close()
	return client
}

func TestServer_AdminUser_RoutesToAdminTUI(t *testing.T) {
	// Admin users must be handed to NewAdminApp, not NewStudentApp.
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireTCPListener(t)
	var adminCalled, studentCalled bool
	tui := &TUIProvider{
		NewStudentApp: func(username, coursesDir, lessonsDir string, d *db.DB, lic *license.State, width, height int, keymap string, launchGDB func(*sandbox.DebugBuild, lesson.Lesson), startLesson *lesson.Lesson) tea.Model {
			studentCalled = true
			return dummyModel{}
		},
		NewAdminApp: func(username string, d *db.DB, lic *license.State, lessonsDir, coursesDir, achievementsFile string, width, height int) tea.Model {
			adminCalled = true
			return dummyModel{}
		},
		LoadAchievements: func(path string) {},
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := database.CreateUser("admin1", "adminpass", db.RoleAdmin); err != nil {
		database.Close()
		t.Fatalf("CreateUser admin: %v", err)
	}
	if err := database.CreateUser("student1", "stupass", db.RoleStudent); err != nil {
		database.Close()
		t.Fatalf("CreateUser student: %v", err)
	}
	database.Close()

	srv, err := New(Config{HostKey: filepath.Join(dir, "host_key"), DBPath: dbPath, Addr: ":0", Keymap: "default"}, tui)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go srv.ListenAndServe() //nolint:errcheck
	addr := waitForListener(t, srv)
	defer func() { srv.Shutdown(context.TODO()); srv.Close() }()

	// Admin user must call NewAdminApp.
	c := startSession(t, addr, "admin1", "adminpass")
	c.Close()
	if !adminCalled {
		t.Error("NewAdminApp was not called for admin user")
	}
	if studentCalled {
		t.Error("NewStudentApp must not be called for admin user")
	}

	// Student user must call NewStudentApp.
	adminCalled, studentCalled = false, false
	c = startSession(t, addr, "student1", "stupass")
	c.Close()
	if !studentCalled {
		t.Error("NewStudentApp was not called for student user")
	}
	if adminCalled {
		t.Error("NewAdminApp must not be called for student user")
	}
}

func TestServer_ShellWithoutPTY_Rejected(t *testing.T) {
	// A "shell" request without a prior "pty-req" must be rejected.
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireTCPListener(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, _ := db.Open(dbPath)
	database.CreateUser("testuser", "pw", db.RoleStudent) //nolint:errcheck
	database.Close()

	srv, _ := New(Config{HostKey: filepath.Join(dir, "host_key"), DBPath: dbPath, Addr: ":0", Keymap: "default"}, newTestTUI())
	go srv.ListenAndServe() //nolint:errcheck
	addr := waitForListener(t, srv)
	defer func() { srv.Shutdown(context.TODO()); srv.Close() }()

	client, err := dial(addr, "testuser", "pw")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	ch, inReqs, err := client.OpenChannel("session", nil)
	if err != nil {
		t.Fatalf("open channel: %v", err)
	}
	go gossh.DiscardRequests(inReqs)
	defer ch.Close()

	// Send "shell" without first sending "pty-req" — server must reply false.
	ok, err := ch.SendRequest("shell", true, nil)
	if err == nil && ok {
		t.Error("shell request without pty-req should have been rejected")
	}
}

func TestServer_KeyboardInteractiveAuth(t *testing.T) {
	// Keyboard-interactive is a supported SSH auth method.
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireTCPListener(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, _ := db.Open(dbPath)
	database.CreateUser("kiuser", "kipass", db.RoleStudent) //nolint:errcheck
	database.Close()

	srv, _ := New(Config{HostKey: filepath.Join(dir, "host_key"), DBPath: dbPath, Addr: ":0", Keymap: "default"}, newTestTUI())
	go srv.ListenAndServe() //nolint:errcheck
	addr := waitForListener(t, srv)
	defer func() { srv.Shutdown(context.TODO()); srv.Close() }()

	cfg := &gossh.ClientConfig{
		User: "kiuser",
		Auth: []gossh.AuthMethod{
			gossh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = "kipass"
				}
				return answers, nil
			}),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	client, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatalf("keyboard-interactive auth failed: %v", err)
	}
	client.Close()

	// Wrong password via keyboard-interactive must fail.
	cfgBad := &gossh.ClientConfig{
		User: "kiuser",
		Auth: []gossh.AuthMethod{
			gossh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				return []string{"wrongpass"}, nil
			}),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	if _, err = gossh.Dial("tcp", addr, cfgBad); err == nil {
		t.Fatal("wrong password via keyboard-interactive should be rejected")
	}
}

func TestServer_MaxConns(t *testing.T) {
	// With MaxConns=1 the second concurrent connection must be dropped.
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireTCPListener(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, _ := db.Open(dbPath)
	database.CreateUser("u", "pw", db.RoleStudent) //nolint:errcheck
	database.Close()

	// Use holdModel so the first session keeps the semaphore slot occupied.
	holdTUI := &TUIProvider{
		NewStudentApp: func(username, coursesDir, lessonsDir string, d *db.DB, lic *license.State, width, height int, keymap string, launchGDB func(*sandbox.DebugBuild, lesson.Lesson), startLesson *lesson.Lesson) tea.Model {
			return holdModel{}
		},
		NewAdminApp: func(username string, d *db.DB, lic *license.State, lessonsDir, coursesDir, achievementsFile string, width, height int) tea.Model {
			return dummyModel{}
		},
		LoadAchievements: func(path string) {},
	}

	srv, err := New(Config{
		HostKey: filepath.Join(dir, "host_key"), DBPath: dbPath,
		Addr: ":0", Keymap: "default", MaxConns: 1,
	}, holdTUI)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go srv.ListenAndServe() //nolint:errcheck
	addr := waitForListener(t, srv)
	defer func() { srv.Shutdown(context.TODO()); srv.Close() }()

	// First connection: completes handshake, session stays open (holdModel).
	c1, err := dial(addr, "u", "pw")
	if err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}
	defer c1.Close()

	// Second connection while c1 is held: must be dropped.
	if _, err = dial(addr, "u", "pw"); err == nil {
		t.Fatal("second connection should be rejected when MaxConns=1")
	}

	// After releasing c1, a new connection must be accepted.
	c1.Close()
	time.Sleep(100 * time.Millisecond) // let the server release the semaphore slot
	c3, err := dial(addr, "u", "pw")
	if err != nil {
		t.Fatalf("connection after slot freed should succeed: %v", err)
	}
	c3.Close()
}

func TestServer_MultipleSessions_Sequential(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireTCPListener(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := database.CreateUser("sequser", "seqpass", db.RoleStudent); err != nil {
		database.Close()
		t.Fatalf("CreateUser: %v", err)
	}
	database.Close()

	srv, err := New(Config{
		HostKey: filepath.Join(dir, "host_key"), DBPath: dbPath,
		Addr: ":0", Keymap: "default", MaxConns: 5,
	}, newTestTUI())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go srv.ListenAndServe() //nolint:errcheck
	addr := waitForListener(t, srv)
	defer func() { srv.Shutdown(context.TODO()); srv.Close() }()

	// Open sequential sessions on the same connection.
	client, err := dial(addr, "sequser", "seqpass")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	for i := 0; i < 3; i++ {
		sess, err := client.NewSession()
		if err != nil {
			t.Fatalf("session %d: %v", i, err)
		}
		if err := sess.RequestPty("xterm", 24, 80, gossh.TerminalModes{}); err != nil {
			sess.Close()
			t.Fatalf("session %d pty: %v", i, err)
		}
		if err := sess.Shell(); err != nil {
			sess.Close()
			t.Fatalf("session %d shell: %v", i, err)
		}
		sess.Close()
	}
}
