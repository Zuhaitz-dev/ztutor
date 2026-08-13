package tui

import (
	"fmt"
	"time"

	"ztutor/internal/db"
	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
	"ztutor/internal/logutil"
	"ztutor/internal/remote"
	"ztutor/internal/sandbox"
	"ztutor/internal/update"
	"ztutor/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

// sized holds terminal dimensions for screens that respond to resizes.
// Embed in model structs to get Width/Height fields and a HandleResize method.
type sized struct {
	Width  int
	Height int
}

func (s *sized) HandleResize(msg tea.WindowSizeMsg) {
	s.Width = msg.Width
	s.Height = msg.Height
}

// Named color constants used across the TUI.
const (
	ColorAccent  = "212" // pink — titles, borders, cursors
	ColorCyan    = "81"  // cyan — selected items, highlights
	ColorAmber   = "214" // amber — mascot sprite, warnings
	ColorDim     = "243" // mid-gray — descriptions, help bar
	ColorFaded   = "241" // dark gray — dim/disabled text
	ColorBody    = "252" // light gray — main content
	ColorGold    = "220" // gold — stars, rank
	ColorBorder  = "237" // very dark gray — borders, separators
	ColorBG      = "236" // dark background — selection highlight
	ColorSuccess = "42"  // green — success messages
	ColorError   = "196" // red — error messages
	ColorHex     = "117" // light blue — hex viewer header
	ColorSection = "214" // amber — section headers
)

// backCmd returns a tea.Cmd that fires the given message. Use this instead of
// inline func() tea.Msg closures for navigation back-to-parent patterns.
func backCmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

// rtlWrap applies rtlAlignBlock when rtl is true, returns content unchanged otherwise.
func rtlWrap(rtl bool, content string, width int) string {
	if !rtl {
		return content
	}
	return rtlAlignBlock(content, width)
}

// dirArrow returns "▸ " in LTR and " ◂" in RTL.
func dirArrow(rtl bool) string {
	if rtl {
		return " ◂"
	}
	return "▸ "
}

// launchGDBMsg is no longer used — GDB runs inside the TUI now.

// achievementEventMsg carries a list of event strings that may unlock achievements.
type achievementEventMsg struct{ events []string }

type mascotTickMsg struct{}

type mascotFrameSetter interface {
	SetMascotFrame(int)
}

// Localizable is implemented by screens that can update their displayed locale
// without navigating away. When the user presses ^L outside of intro/menu, the
// App calls SetLocale on the current screen if it implements this interface
// instead of redirecting to the menu.
type Localizable interface {
	SetLocale(loc *i18n.Locale)
}

func mascotTickCmd() tea.Cmd {
	return tea.Tick(450*time.Millisecond, func(time.Time) tea.Msg {
		return mascotTickMsg{}
	})
}

type launchAdminMsg struct{}

type changeLangMsg struct{ lang string }

// updateCheckMsg is sent when a background version check completes.
type updateCheckMsg struct {
	version string
	url     string
}

// checkUpdateCmd returns a tea.Cmd that checks GitHub for a newer release.
func (a *App) checkUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		info, err := update.CheckLatest(version.Version, a.db, a.username)
		if err != nil || info == nil {
			return nil
		}
		return updateCheckMsg{
			version: info.Version,
			url:     info.ReleaseURL,
		}
	}
}

type App struct {
	username   string
	db         *db.DB
	coursesDir string
	lessonsDir string
	courses    []lesson.Course
	progress   map[string]int
	keymap     string
	streak     int
	isAdmin    bool

	uiLang string
	loc    *i18n.Locale

	langCache map[string]sandbox.Language
	executor  sandbox.Executor

	pendingNotifications []string
	pendingCourseEntry   *lesson.Course // set when a course intro is playing
	activeCourseID       string         // current course context for returning from lesson/exercise screens
	activeCourseNodeID   string         // preferred lesson/node selection when reopening a course

	current tea.Model
	gamepad *NativeGamepad

	sized
	mascotFrame int

	LaunchAdmin bool
}

func NewApp(username, coursesDir, lessonsDir string, database *db.DB, width, height int, serverKeymap string) *App {
	keymap, _ := database.GetUserSetting(username, "keymap")
	if keymap == "" {
		keymap = serverKeymap
	}
	uiLang, _ := database.GetUserSetting(username, "lang")
	if uiLang == "" {
		uiLang = "en"
	}

	app := &App{
		username:   username,
		db:         database,
		coursesDir: coursesDir,
		lessonsDir: lessonsDir,
		sized:      sized{Width: width, Height: height},
		keymap:     keymap,
		langCache:  make(map[string]sandbox.Language),
		executor:   sandbox.DefaultExecutor(),
		uiLang:     uiLang,
		loc:        i18n.New(uiLang),
	}
	if nativeGamepadEnabled {
		app.gamepad = NewNativeGamepad()
	}

	if u, err := database.GetUser(username); err == nil && u.Role == db.RoleAdmin {
		app.isAdmin = true
	}

	app.loadCourses()
	app.loadProgress()
	app.streak = database.UpdateStreak(username)

	introSeen, _ := database.GetUserSetting(username, "intro_seen")
	seenIntro := introSeen == "1"
	app.current = NewIntroScreen(width, height, seenIntro, app.loc)
	app.applyMascotFrame()

	return app
}

func (a *App) WantsRelaunch() bool  { return a.LaunchAdmin }
func (a *App) RelaunchUser() string { return a.username }

func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{mascotTickCmd(), a.checkUpdateCmd()}
	if a.gamepad != nil {
		cmds = append(cmds, a.gamepad.Next())
	}
	if a.current != nil {
		if cmd := a.current.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if gm, ok := msg.(gamepadInputMsg); ok {
		m, cmd := a.Update(normalizeInputMsg(gm))
		if app, ok := m.(*App); ok && app.gamepad != nil {
			return app, tea.Batch(cmd, app.gamepad.Next())
		}
		return m, cmd
	}
	msg = normalizeInputMsg(msg)
	// ^L: save language to DB synchronously before any async cmd, so the
	// preference is persisted even if the user navigates away immediately.
	// IntroScreen and MenuScreen rebuild their own content and also emit
	// changeLangMsg, so we let them handle the key normally — but we still
	// save to DB here first to close the race.
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == KeyLanguage {
		next := a.loc.Next()
		logutil.Debug("lang: switching from %s to %s (user=%s)", a.loc.Lang(), next.Lang(), a.username)
		if err := a.db.SetUserSetting(a.username, "lang", next.Lang()); err != nil {
			logutil.Warn("failed to save lang for %s: %v", a.username, err)
		}
		a.uiLang = next.Lang()
		a.loc = next
		_, isIntro := a.current.(*IntroScreen)
		_, isMenu := a.current.(*MenuScreen)
		_, isConnect := a.current.(*connectChoiceScreen)
		if !isIntro && !isMenu && !isConnect {
			// Reload course/lesson data for the new locale so that when the
			// user returns to the menu, lesson titles and content are in the
			// correct language. (The changeLangMsg path for menu/intro already
			// calls loadCourses; this ensures parity for in-place locale switches.)
			a.loadCourses()
			// Push updated lesson content/hints/tutorial into the live screen
			// before swapping the locale so everything re-renders together.
			a.refreshCurrentScreenData()
			// If the current screen knows how to update its own locale, do
			// that in place so the user stays where they are. Otherwise fall
			// back to navigating to the menu so they see the new language.
			if l, ok := a.current.(Localizable); ok {
				l.SetLocale(a.loc)
			} else {
				a.switchToMenu()
			}
			return a, nil
		}
		// For intro/menu/connect: pass the key through so they rebuild their
		// own view. changeLangMsg will arrive later but DB is already saved.
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// BubbleTea queries terminal size via ioctl on startup which fails
		// on an SSH channel and returns 0,0. Ignore those bogus messages.
		if msg.Width <= 0 || msg.Height <= 0 {
			return a, nil
		}
		a.HandleResize(msg)
		return a, a.resizeCurrent()

	case mascotTickMsg:
		a.mascotFrame++
		a.applyMascotFrame()
		return a, mascotTickCmd()

	case updateCheckMsg:
		if msg.version != "" {
			notif := fmt.Sprintf("New version %s available — download at %s", msg.version, msg.url)
			logutil.Info("%s", notif)
			a.pendingNotifications = append(a.pendingNotifications, notif)
		}
		return a, nil

	case introCompleteMsg:
		if msg.courseID != "" {
			// Course-specific intro finished.
			key := "course_intro_seen_" + msg.courseID
			if err := a.db.SetUserSetting(a.username, key, "1"); err != nil {
				logutil.Warn("failed to save %s for %s: %v", key, a.username, err)
			}
			if a.pendingCourseEntry != nil {
				course := *a.pendingCourseEntry
				a.pendingCourseEntry = nil
				// Re-derive course from the loaded courses so sections reflect
				// the current course data.
				for _, fc := range a.courses {
					if fc.ID == course.ID {
						course = fc
						break
					}
				}
				if len(course.Sections) == 0 {
					a.switchToMenu()
				} else {
					a.openCourse(course)
				}
			} else {
				a.switchToMenu()
			}
		} else {
			// Main app intro finished.
			if err := a.db.SetUserSetting(a.username, "intro_seen", "1"); err != nil {
				logutil.Warn("failed to save intro_seen for %s: %v", a.username, err)
			}
			execAddr, _ := a.db.GetUserSetting(a.username, "exec_addr")
			a.current = NewConnectChoiceScreen(a.loc, a.Width, a.Height, execAddr)
			a.applyMascotFrame()
			return a, a.current.Init()
		}
		return a, nil

	case enterCoursePathMsg:
		a.openCourse(msg.course)
		return a, a.current.Init()

	case showCourseIntroMsg:
		a.pendingCourseEntry = &msg.course
		a.current = NewCourseIntroScreen(msg.course.ID, msg.course.Language, msg.course.Title, msg.course.CourseIntro, a.loc, a.Width, a.Height)
		a.applyMascotFrame()
		return a, a.current.Init()

	case changeLangMsg:
		if err := a.db.SetUserSetting(a.username, "lang", msg.lang); err != nil {
			logutil.Warn("failed to save lang for %s: %v", a.username, err)
		}
		a.uiLang = msg.lang
		a.loc = i18n.New(msg.lang)
		a.loadCourses()
		// Intro rebuilds its own beats; stay there instead of jumping to menu.
		if intro, ok := a.current.(*IntroScreen); ok {
			// For course intros, customBeats come from course_intro_i18n.
			// The intro already rebuilt with the stale beats when ^L fired;
			// now that loadCourses ran we can push the correct language beats.
			if intro.courseID != "" {
				if c, found := a.findCourse(intro.courseID); found {
					intro.customBeats = c.CourseIntro
					intro.Beats, intro.beatMeta = courseIntroBeats(intro.courseLang, intro.courseTitle, intro.customBeats, a.loc)
					if intro.BeatIdx >= len(intro.Beats) {
						intro.BeatIdx = len(intro.Beats) - 1
					}
					intro.CharIdx = 0
				}
			}
			return a, nil
		}
		// Preserve lesson-list position across language reload for MenuScreen.
		var savedCourseID string
		if m, ok := a.current.(*MenuScreen); ok && m.viewLevel == "lessons" && m.selectedCourse != nil {
			savedCourseID = m.selectedCourse.ID
		}
		a.switchToMenu()
		if savedCourseID != "" {
			if m, ok := a.current.(*MenuScreen); ok {
				m.restoreCourse(savedCourseID)
			}
		}
		return a, nil

	case launchAdminMsg:
		a.LaunchAdmin = true
		return a, tea.Quit

	case NavigateToMenu:
		a.switchToMenu()
		return a, nil

	case NavigateBackToCourse:
		a.returnToActiveCourse()
		return a, nil

	case NavigateToConnectChoice:
		execAddr, _ := a.db.GetUserSetting(a.username, "exec_addr")
		a.current = NewConnectChoiceScreen(a.loc, a.Width, a.Height, execAddr)
		a.applyMascotFrame()
		return a, a.current.Init()

	case NavigateToRemoteConfig:
		a.current = NewRemoteConfigScreen(a.loc, a.Width, a.Height, a.db, a.username)
		return a, a.current.Init()

	case remoteConfigSavedMsg:
		if msg.addr != "" {
			a.executor = &sandbox.HybridExecutor{
				Local:  sandbox.LocalExecutor{},
				Remote: remote.NewClientWithToken(msg.addr, msg.token, msg.tls),
			}
			logutil.Info("executor updated: remote at %s (tls: %v)", msg.addr, msg.tls)
		} else {
			a.executor = sandbox.LocalExecutor{}
			logutil.Info("executor updated: local only")
		}
		a.switchToMenu()
		return a, nil

	case NavigateToLessonMsg:
		a.activeCourseNodeID = msg.Lesson.ID
		a.current = NewLessonScreen(msg.Lesson, a.progress[msg.Lesson.ID], a.Width, a.Height, a.loc)
		a.applyMascotFrame()
		return a, a.current.Init()

	case NavigateToExerciseMsg:
		a.activeCourseNodeID = msg.Lesson.ID
		if len(msg.Lesson.Tutorial) > 0 {
			key := "tutorial_" + msg.Lesson.ID
			val, _ := a.db.GetUserSetting(a.username, key)
			if val != "1" {
				a.current = NewPreExerciseScreen(msg.Lesson, a.Width, a.Height, a.loc)
				a.applyMascotFrame()
				return a, a.current.Init()
			}
		}
		showMascot, showTimer := a.exercisePrefs()
		es := NewExerciseScreen(msg.Lesson, a.resolveLanguage(&msg.Lesson), a.executor, a.Width, a.Height, a.keymap, a.progress[msg.Lesson.ID], a.streak, a.loc, showMascot, showTimer)
		es.SetHasGamepad(a.gamepad != nil)
		a.current = es
		a.applyMascotFrame()
		return a, a.current.Init()

	case startExerciseMsg:
		a.activeCourseNodeID = msg.lesson.ID
		if err := a.db.SetUserSetting(a.username, "tutorial_"+msg.lesson.ID, "1"); err != nil {
			logutil.Warn("failed to save tutorial_%s for %s: %v", msg.lesson.ID, a.username, err)
		}
		showMascot, showTimer := a.exercisePrefs()
		es2 := NewExerciseScreen(msg.lesson, a.resolveLanguage(&msg.lesson), a.executor, a.Width, a.Height, a.keymap, a.progress[msg.lesson.ID], a.streak, a.loc, showMascot, showTimer)
		es2.SetHasGamepad(a.gamepad != nil)
		a.current = es2
		a.applyMascotFrame()
		return a, a.current.Init()

	case NavigateToAchievements:
		earned, _ := a.db.GetAchievements(a.username)
		a.current = NewAchievementScreen(earned, a.Width, a.Height, a.loc)
		a.applyMascotFrame()
		return a, a.current.Init()

	case NavigateToLeaderboard:
		entries, _ := a.db.Leaderboard()
		a.current = NewLeaderboardScreen(entries, a.username, a.totalLessons(), a.Width, a.Height, a.loc)
		a.applyMascotFrame()
		return a, a.current.Init()

	case NavigateToChallengeMsg:
		a.current = NewChallengeScreen(msg.Challenge, msg.CourseID, msg.Lang, a.executor, a.Width, a.Height, a.loc)
		a.applyMascotFrame()
		return a, a.current.Init()

	case NavigateToQuizMsg:
		a.current = NewQuizScreen(msg.Quiz, a.progress[msg.Quiz.ID], a.Width, a.Height, a.loc)
		a.applyMascotFrame()
		return a, a.current.Init()

	case NavigateToSettings:
		showMascot, showTimer := a.exercisePrefs()
		a.current = NewSettingsScreen(a.username, a.db, a.keymap, showMascot, showTimer, a.Width, a.Height, a.loc)
		a.applyMascotFrame()
		return a, a.current.Init()

	case NavigateToCredits:
		a.current = NewCreditsScreen(a.Width, a.Height, a.loc)
		a.applyMascotFrame()
		return a, a.current.Init()

	case settingsSavedMsg:
		if err := a.db.SetUserSetting(a.username, msg.key, msg.value); err != nil {
			logutil.Warn("failed to save setting %s for %s: %v", msg.key, a.username, err)
		}
		if msg.key == "keymap" {
			a.keymap = msg.value
		}
		return a, nil

	case persistSettingMsg:
		if err := a.db.SetUserSetting(a.username, msg.key, msg.value); err != nil {
			logutil.Warn("failed to persist setting %s for %s: %v", msg.key, a.username, err)
		}
		return a, nil

	case lessonCompletedMsg:
		if err := a.db.MarkCompleted(a.username, msg.lessonID, msg.stars); err != nil {
			logutil.Warn("failed to mark %s completed for %s: %v", msg.lessonID, a.username, err)
		}
		if msg.stars > a.progress[msg.lessonID] {
			a.progress[msg.lessonID] = msg.stars
		}
		a.activeCourseNodeID = a.preferredCourseNodeAfterCompletion(msg.lessonID)
		// Check for graduate achievement: all lessons in the same course done.
		a.checkGraduate(msg.lessonID)
		if msg.goNext {
			if next, ok := a.nextLesson(msg.lessonID); ok {
				a.activeCourseNodeID = next.ID
				a.current = NewLessonScreen(next, a.progress[next.ID], a.Width, a.Height, a.loc)
				a.applyMascotFrame()
				return a, a.current.Init()
			}
		}
		a.returnToActiveCourse()
		return a, nil

	case achievementEventMsg:
		a.grantAchievements(msg.events)
		return a, nil
	}

	if a.current != nil {
		m, cmd := a.current.Update(msg)
		a.current = m
		a.applyMascotFrame()
		return a, cmd
	}

	return a, nil
}

func (a *App) View() string {
	if a.current != nil {
		return a.current.View()
	}
	return "loading..."
}
