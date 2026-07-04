package ssh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"ztutor/internal/db"
	"ztutor/internal/lesson"
	"ztutor/internal/license"
	"ztutor/internal/logutil"
	"ztutor/internal/sandbox"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/sys/unix"
)

type TUIProvider struct {
	NewStudentApp    func(username, coursesDir, lessonsDir string, db *db.DB, license *license.State, width, height int, keymap string, launchGDB func(*sandbox.DebugBuild, lesson.Lesson), startLesson *lesson.Lesson) tea.Model
	NewAdminApp      func(username string, db *db.DB, license *license.State, lessonsDir, coursesDir, achievementsFile string, width, height int) tea.Model
	LoadAchievements func(path string)
}

type SessionResult interface {
	tea.Model
	WantsRelaunch() bool
	RelaunchUser() string
}

type Config struct {
	HostKey          string
	CoursesDir       string
	LessonsDir       string
	AchievementsFile string
	DBPath           string
	Addr             string
	Keymap           string
	License          *license.State
	SetupToken       string
	MaxConns         int // 0 = unlimited
}

type Server struct {
	config   Config
	tui      *TUIProvider
	db       *db.DB
	listener net.Listener
	stopCh   chan struct{}
	stopOnce sync.Once
	readyCh  chan struct{} // closed once the server is accepting connections

	// setup-token rate limiting (only active when no users exist)
	tokenMu       sync.Mutex
	tokenExpiry   time.Time
	tokenAttempts int
}

func New(cfg Config, tui *TUIProvider) (*Server, error) {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if clang := sandbox.GetLanguage("c"); clang != nil {
		if !clang.HasDebugger() {
			logutil.Info("NOTE: gdb not found at %q — debugger unavailable", clang.DebuggerPath())
		}
	} else {
		logutil.Warn("C language toolchain not found — compilation will fail")
	}

	if cfg.AchievementsFile != "" {
		tui.LoadAchievements(cfg.AchievementsFile)
	}

	if cfg.License != nil && cfg.License.Licensed {
		logutil.Info("license: %s [tier=%s students=%d courses=%v multi=%v admin=%v interviews=%v]",
			cfg.License.Licensee, cfg.License.ProductTier(), cfg.License.MaxStudents,
			cfg.License.UnlockedCourses, cfg.License.HasMultiUser,
			cfg.License.HasAdminUI, cfg.License.HasInterviewQuestions)
	} else {
		logutil.Info("license: none (free tier)")
	}

	return &Server{
		config:  cfg,
		tui:     tui,
		db:      database,
		stopCh:  make(chan struct{}),
		readyCh: make(chan struct{}),
	}, nil
}

func (s *Server) ListenAndServe() error {
	signer, err := loadOrGenerateHostKey(s.config.HostKey)
	if err != nil {
		return fmt.Errorf("host key: %w", err)
	}

	hasUsers, err := s.db.HasUsers()
	if err != nil {
		logutil.Warn("cannot check users: %v (assuming users exist for safety)", err)
		hasUsers = true // safer default: require auth rather than allow open access
	}

	serverConfig := &gossh.ServerConfig{
		PublicKeyCallback: func(meta gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
			if !hasUsers {
				return &gossh.Permissions{Extensions: map[string]string{"username": meta.User()}}, nil
			}
			_, err := s.db.GetUser(meta.User())
			if err != nil {
				return nil, fmt.Errorf("unknown user")
			}
			return &gossh.Permissions{Extensions: map[string]string{"username": meta.User()}}, nil
		},
		PasswordCallback: func(meta gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
			if !hasUsers {
				if err := s.checkSetupToken(string(password)); err != nil {
					return nil, err
				}
				return &gossh.Permissions{Extensions: map[string]string{"username": meta.User()}}, nil
			}
			_, err := s.db.Authenticate(meta.User(), string(password))
			if err != nil {
				return nil, err
			}
			return &gossh.Permissions{Extensions: map[string]string{"username": meta.User()}}, nil
		},
		KeyboardInteractiveCallback: func(meta gossh.ConnMetadata, client gossh.KeyboardInteractiveChallenge) (*gossh.Permissions, error) {
			answers, err := client(meta.User(), "",
				[]string{meta.User() + "@ztutor's password: "},
				[]bool{false},
			)
			if err != nil {
				return nil, err
			}
			if len(answers) == 0 {
				return nil, fmt.Errorf("no password provided")
			}
			if !hasUsers {
				if err := s.checkSetupToken(answers[0]); err != nil {
					return nil, err
				}
				return &gossh.Permissions{Extensions: map[string]string{"username": meta.User()}}, nil
			}
			_, err = s.db.Authenticate(meta.User(), answers[0])
			if err != nil {
				return nil, err
			}
			return &gossh.Permissions{Extensions: map[string]string{"username": meta.User()}}, nil
		},
	}
	serverConfig.AddHostKey(signer)

	listener, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = listener
	if !hasUsers {
		s.tokenMu.Lock()
		s.tokenExpiry = time.Now().Add(24 * time.Hour)
		s.tokenMu.Unlock()
	}
	close(s.readyCh) // unblock any waiters
	defer listener.Close()

	if hasUsers {
		count, err := s.db.CountUsers()
		if err != nil {
			logutil.Warn("cannot count users: %v", err)
		} else {
			logutil.Info("serving %d student(s) on %s", count, listener.Addr())
		}
	} else {
		tokenPreview := s.config.SetupToken[:20]
		if s.config.License != nil && s.config.License.HasAdminUI {
			logutil.Info("no users yet - business setup token prefix: %s...", tokenPreview)
			logutil.Info("  connect to create the admin account: ssh <any-name>@localhost -p %d", listener.Addr().(*net.TCPAddr).Port)
		} else {
			logutil.Info("no users yet - learner setup token prefix: %s...", tokenPreview)
			logutil.Info("  connect to create the first learner account: ssh <any-name>@localhost -p %d", listener.Addr().(*net.TCPAddr).Port)
		}
	}

	var sem chan struct{}
	if s.config.MaxConns > 0 {
		sem = make(chan struct{}, s.config.MaxConns)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return nil
			default:
				logutil.Warn("accept error: %v", err)
				continue
			}
		}

		if sem != nil {
			select {
			case sem <- struct{}{}:
				go func() {
					defer func() { <-sem }()
					s.handleConnection(conn, serverConfig)
				}()
			default:
				conn.Close()
			}
		} else {
			go s.handleConnection(conn, serverConfig)
		}
	}
}

// Ready returns a channel that is closed once the server is accepting connections.
func (s *Server) Ready() <-chan struct{} { return s.readyCh }

// ListenAddr returns the address the server is listening on, or "" if not yet started.
func (s *Server) ListenAddr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Shutdown(ctx context.Context) {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		if s.listener != nil {
			s.listener.Close()
		}
	})
}

func (s *Server) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// checkSetupToken validates the setup token with expiry and brute-force protection.
// Locked out after 5 failed attempts; expires 24 h after the server binds.
func (s *Server) checkSetupToken(candidate string) error {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if s.tokenAttempts >= 5 {
		return fmt.Errorf("setup token locked after too many failed attempts — restart the server to reset")
	}
	if time.Now().After(s.tokenExpiry) {
		return fmt.Errorf("setup token expired (24 h window) — restart the server to get a new one")
	}
	if !db.SecureCompare(candidate, s.config.SetupToken) {
		s.tokenAttempts++
		remaining := 5 - s.tokenAttempts
		return fmt.Errorf("invalid setup token (%d attempt(s) remaining)", remaining)
	}
	return nil
}

func (s *Server) handleConnection(conn net.Conn, config *gossh.ServerConfig) {
	defer conn.Close()

	sshConn, chans, reqs, err := gossh.NewServerConn(conn, config)
	if err != nil {
		logutil.Error("ssh handshake: %v", err)
		return
	}
	defer sshConn.Close()

	go gossh.DiscardRequests(reqs)

	for ch := range chans {
		if ch.ChannelType() != "session" {
			ch.Reject(gossh.UnknownChannelType, "only session channels allowed")
			continue
		}

		channel, requests, err := ch.Accept()
		if err != nil {
			logutil.Error("accept channel: %v", err)
			continue
		}

		go s.handleSession(channel, requests, sshConn.User())
	}
}

func (s *Server) handleSession(channel gossh.Channel, requests <-chan *gossh.Request, username string) {
	defer channel.Close()

	if username == "" {
		username = "user"
	}

	if !validSessionUsername(username) {
		logutil.Debug("ssh: rejected invalid username %q", username)
		return
	}

	var isAdmin bool
	hasUsers, err := s.db.HasUsers()
	if err != nil {
		logutil.Warn("cannot check users in session: %v", err)
		hasUsers = true // conservative: require auth checks
	}

	if !hasUsers {
		isAdmin = true
	} else {
		user, err := s.db.GetUser(username)
		if err == nil && user.Role == db.RoleAdmin {
			isAdmin = true
		}
	}

	var ptyRequested bool
	var initialCols, initialRows int
	var termName string

	for req := range requests {
		switch req.Type {
		case "pty-req":
			ptyRequested = true
			if len(req.Payload) >= 4 {
				termLen := int(binary.BigEndian.Uint32(req.Payload[0:4]))
				if 4+termLen <= len(req.Payload) {
					termName = string(req.Payload[4 : 4+termLen])
				}
				offset := 4 + termLen
				if offset+8 <= len(req.Payload) {
					initialCols = int(binary.BigEndian.Uint32(req.Payload[offset:]))
					initialRows = int(binary.BigEndian.Uint32(req.Payload[offset+4:]))
				}
			}
			req.Reply(true, nil)

		case "shell":
			if !ptyRequested {
				req.Reply(false, nil)
				return
			}
			req.Reply(true, nil)
			if isAdmin {
				s.runAdminTUI(channel, requests, username, initialCols, initialRows, termName)
			} else {
				s.runTUI(channel, requests, username, initialCols, initialRows, termName)
			}
			return

		default:
			req.Reply(false, nil)
		}
	}
}

func setPTYSize(master *os.File, rows, cols int) {
	unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{ //nolint:errcheck
		Row: uint16(rows),
		Col: uint16(cols),
	})
}

// runGDBSession allocates a PTY, connects gdb to the slave side, and proxies
// the SSH channel through the master. gdb sees a real terminal (isatty=true)
// and behaves interactively. Blocks until gdb exits.
func (s *Server) runGDBSession(channel gossh.Channel, requests <-chan *gossh.Request, build *sandbox.DebugBuild, cols, rows int) {
	master, slaveName, err := openPTY()
	if err != nil {
		fmt.Fprintf(channel, "\r\ngdb: failed to allocate PTY: %v\r\n", err)
		return
	}
	defer master.Close()

	if cols > 0 && rows > 0 {
		setPTYSize(master, rows, cols)
	}

	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		fmt.Fprintf(channel, "\r\ngdb: failed to open PTY slave: %v\r\n", err)
		return
	}

	gdbArgs := []string{"-q", "-iex", "set debuginfod enabled off"}
	if len(build.RuntimeArgs) > 0 {
		gdbArgs = append(gdbArgs, "--args", build.BinaryPath)
		gdbArgs = append(gdbArgs, build.RuntimeArgs...)
	} else {
		gdbArgs = append(gdbArgs, build.BinaryPath)
	}
	gdb := exec.Command(sandbox.GetLanguage("c").DebuggerPath(), gdbArgs...)
	gdb.Stdin = slave
	gdb.Stdout = slave
	gdb.Stderr = slave
	gdb.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true, // new session — no inherited controlling terminal
		Setctty: true, // make slave the controlling terminal
		Ctty:    0,    // fd 0 = slave in the child
	}

	if err := gdb.Start(); err != nil {
		slave.Close()
		fmt.Fprintf(channel, "\r\ngdb: %v\r\n", err)
		fmt.Fprintf(channel, "Is gdb installed? (sudo dnf install gdb)\r\n")
		return
	}
	slave.Close() // parent doesn't need the slave fd once the child has it

	// Proxy: SSH channel <-> PTY master.
	go io.Copy(master, channel)
	go io.Copy(channel, master)

	// Forward terminal resize events from the SSH client to the PTY.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case req, ok := <-requests:
				if !ok {
					return
				}
				if req.Type == "window-change" && len(req.Payload) >= 8 {
					w := int(binary.BigEndian.Uint32(req.Payload[0:4]))
					h := int(binary.BigEndian.Uint32(req.Payload[4:8]))
					setPTYSize(master, h, w)
				}
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
		}
	}()

	gdb.Wait() //nolint:errcheck
}

func colorProfileForTerm(termName string) termenv.Profile {
	ck := os.Getenv("COLORTERM")
	if ck == "truecolor" || ck == "24bit" {
		return termenv.TrueColor
	}
	if strings.Contains(termName, "256color") || strings.Contains(termName, "truecolor") {
		return termenv.ANSI256
	}
	if strings.HasPrefix(termName, "xterm") || strings.HasPrefix(termName, "vte") || strings.HasPrefix(termName, "screen") {
		return termenv.ANSI
	}
	return termenv.Ascii
}

func (s *Server) runAdminTUI(channel gossh.Channel, requests <-chan *gossh.Request, username string, cols, rows int, termName string) {
	defer func() {
		if r := recover(); r != nil {
			logutil.Error("Admin TUI panic for %s: %v", username, r)
		}
	}()

	lipgloss.SetColorProfile(colorProfileForTerm(termName))
	curCols, curRows := cols, rows

	app := s.tui.NewAdminApp(username, s.db, s.config.License, s.config.LessonsDir, s.config.CoursesDir, s.config.AchievementsFile, curCols, curRows)

	p := tea.NewProgram(
		app,
		tea.WithInput(channel),
		tea.WithOutput(channel),
		tea.WithAltScreen(),
		tea.WithoutCatchPanics(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	goroutineDone := make(chan struct{})
	go func() {
		defer close(goroutineDone)
		if curCols > 0 && curRows > 0 {
			p.Send(tea.WindowSizeMsg{Width: curCols, Height: curRows})
		}
		for {
			select {
			case <-ctx.Done():
				return
			case req, ok := <-requests:
				if !ok {
					cancel()
					return
				}
				if req.Type == "window-change" && len(req.Payload) >= 8 {
					w := int(binary.BigEndian.Uint32(req.Payload[0:4]))
					h := int(binary.BigEndian.Uint32(req.Payload[4:8]))
					curCols, curRows = w, h
					p.Send(tea.WindowSizeMsg{Width: w, Height: h})
				}
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
		}
	}()

	if _, err := p.Run(); err != nil {
		logutil.Error("Admin TUI run error: %v", err)
	}

	cancel()
	<-goroutineDone

	if sr, ok := app.(SessionResult); ok && sr.WantsRelaunch() {
		if sr, ok := app.(SessionResult); ok {
			s.runTUI(channel, requests, sr.RelaunchUser(), curCols, curRows, termName)
		}
	}
}

func (s *Server) runTUI(channel gossh.Channel, requests <-chan *gossh.Request, username string, cols, rows int, termName string) {
	defer func() {
		if r := recover(); r != nil {
			logutil.Error("TUI panic for %s: %v", username, r)
		}
	}()

	lipgloss.SetColorProfile(colorProfileForTerm(termName))

	// Loop: TUI → gdb session → TUI → ... until the user quits normally.
	curCols, curRows := cols, rows
	var resumeLesson *lesson.Lesson // non-nil after a gdb session: reopen that exercise
	for {
		var pendingGDB *sandbox.DebugBuild
		var pendingLesson lesson.Lesson

		app := s.tui.NewStudentApp(username, s.config.CoursesDir, s.config.LessonsDir, s.db,
			s.config.License,
			curCols, curRows,
			s.config.Keymap,
			func(build *sandbox.DebugBuild, l lesson.Lesson) {
				pendingGDB = build
				pendingLesson = l
			},
			resumeLesson,
		)

		p := tea.NewProgram(
			app,
			tea.WithInput(channel),
			tea.WithOutput(channel),
			tea.WithAltScreen(),
			tea.WithoutCatchPanics(),
		)

		// Window-change goroutine: forward SSH resize events to BubbleTea.
		// Tracks the latest size so the restarted TUI gets correct dimensions.
		ctx, cancel := context.WithCancel(context.Background())
		goroutineDone := make(chan struct{})
		go func() {
			defer close(goroutineDone)
			if curCols > 0 && curRows > 0 {
				p.Send(tea.WindowSizeMsg{Width: curCols, Height: curRows})
			}
			for {
				select {
				case <-ctx.Done():
					return
				case req, ok := <-requests:
					if !ok {
						cancel()
						return
					}
					if req.Type == "window-change" && len(req.Payload) >= 8 {
						w := int(binary.BigEndian.Uint32(req.Payload[0:4]))
						h := int(binary.BigEndian.Uint32(req.Payload[4:8]))
						curCols, curRows = w, h
						p.Send(tea.WindowSizeMsg{Width: w, Height: h})
					}
					if req.WantReply {
						req.Reply(false, nil)
					}
				}
			}
		}()

		if _, err := p.Run(); err != nil {
			logutil.Error("TUI run error: %v", err)
		}

		// Stop the window-change goroutine before handing requests to gdb.
		cancel()
		<-goroutineDone

		if sr, ok := app.(SessionResult); ok && sr.WantsRelaunch() {
			s.runAdminTUI(channel, requests, username, curCols, curRows, termName)
			break
		}

		if pendingGDB == nil {
			break // normal quit (q / ctrl+c)
		}

		s.runGDBSession(channel, requests, pendingGDB, curCols, curRows)
		pendingGDB.Close()
		resumeLesson = &pendingLesson // restart directly at the exercise screen
	}
}

func validSessionUsername(name string) bool {
	if len(name) < 1 || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func loadOrGenerateHostKey(path string) (gossh.Signer, error) {
	if data, err := os.ReadFile(path); err == nil {
		return gossh.ParsePrivateKey(data)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}

	signer, err := gossh.NewSignerFromKey(key)
	if err != nil {
		return nil, fmt.Errorf("create signer: %w", err)
	}

	return signer, nil
}
