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

type dummyModel struct{}

func (m dummyModel) Init() tea.Cmd { return tea.Quit }
func (m dummyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, tea.Quit
}
func (m dummyModel) View() string { return "" }

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

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	hostKeyPath := filepath.Join(dir, "host_key")

	// Create a test user in the DB before starting the server.
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	pw := "testpass"
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
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	var addr net.Addr
	for i := 0; i < 200; i++ {
		if srv.listener != nil {
			addr = srv.listener.Addr()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == nil {
		srv.Shutdown(context.TODO())
		srv.Close()
		t.Fatal("server did not start in time")
	}

	clientConfig := &gossh.ClientConfig{
		User: "testuser",
		Auth: []gossh.AuthMethod{
			gossh.Password(pw),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := gossh.Dial("tcp", addr.String(), clientConfig)
	if err != nil {
		srv.Shutdown(context.TODO())
		srv.Close()
		t.Fatalf("ssh dial: %v", err)
	}
	client.Close()

	// Let the session goroutine finish.
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

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	hostKeyPath := filepath.Join(dir, "host_key")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := database.CreateUser("testuser", "correctpass", db.RoleStudent); err != nil {
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
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	var addr net.Addr
	for i := 0; i < 200; i++ {
		if srv.listener != nil {
			addr = srv.listener.Addr()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == nil {
		srv.Shutdown(context.TODO())
		srv.Close()
		t.Fatal("server did not start in time")
	}

	clientConfig := &gossh.ClientConfig{
		User: "testuser",
		Auth: []gossh.AuthMethod{
			gossh.Password("wrongpass"),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	_, err = gossh.Dial("tcp", addr.String(), clientConfig)
	if err == nil {
		srv.Shutdown(context.TODO())
		srv.Close()
		t.Fatal("expected auth failure with wrong password")
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

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	hostKeyPath := filepath.Join(dir, "host_key")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := database.CreateUser("realuser", "testpass", db.RoleStudent); err != nil {
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
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	var addr net.Addr
	for i := 0; i < 200; i++ {
		if srv.listener != nil {
			addr = srv.listener.Addr()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == nil {
		srv.Shutdown(context.TODO())
		srv.Close()
		t.Fatal("server did not start in time")
	}

	clientConfig := &gossh.ClientConfig{
		User: "nonexistent",
		Auth: []gossh.AuthMethod{
			gossh.Password("testpass"),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	_, err = gossh.Dial("tcp", addr.String(), clientConfig)
	if err == nil {
		srv.Shutdown(context.TODO())
		srv.Close()
		t.Fatal("expected auth failure with unknown user")
	}

	srv.Shutdown(context.TODO())
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Error("ListenAndServe did not return in time")
	}
	srv.Close()
}
