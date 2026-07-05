package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"ztutor/internal/db"
	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
	"ztutor/internal/license"
	"ztutor/internal/logutil"
	"ztutor/internal/remote"
	"ztutor/internal/sandbox"
	"ztutor/internal/update"
	"ztutor/internal/version"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// launchGDBMsg is sent by ExerciseScreen when the user wants to start a gdb
// session. The App forwards it to the server layer, then quits so the SSH
// handler can run gdb directly with a real PTY.
type launchGDBMsg struct {
	build  *sandbox.DebugBuild
	lesson lesson.Lesson
}

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
	username    string
	db          *db.DB
	coursesDir  string
	lessonsDir  string
	courses     []lesson.Course
	progress    map[string]int
	enrolledSet map[string]bool
	keymap      string
	streak      int
	lic         *license.State
	isAdmin     bool

	uiLang string
	loc    *i18n.Locale

	langCache map[string]sandbox.Language
	executor  sandbox.Executor

	pendingNotifications []string
	pendingCourseEntry   *lesson.Course // set when a course intro is playing
	activeCourseID       string         // current course context for returning from lesson/exercise screens
	activeCourseNodeID   string         // preferred lesson/node selection when reopening a course

	current   tea.Model
	launchGDB func(*sandbox.DebugBuild, lesson.Lesson)
	gamepad   *NativeGamepad

	sized
	mascotFrame int

	LaunchAdmin bool
}

func NewApp(username, coursesDir, lessonsDir string, database *db.DB, lic *license.State, width, height int, serverKeymap string, launchGDB func(*sandbox.DebugBuild, lesson.Lesson), startLesson *lesson.Lesson) *App {
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
		lic:        lic,
		sized:      sized{Width: width, Height: height},
		keymap:     keymap,
		launchGDB:  launchGDB,
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

	app.loadRedeemedLicenses()
	app.loadCourses()
	app.loadProgress()
	app.streak = database.UpdateStreak(username)

	if startLesson != nil {
		es := NewExerciseScreen(*startLesson, app.resolveLanguage(startLesson), app.executor, width, height, keymap, app.progress[startLesson.ID], app.streak, app.loc, true, true)
		es.SetHasGamepad(app.gamepad != nil)
		app.current = es
		app.applyMascotFrame()
	} else {
		introSeen, _ := database.GetUserSetting(username, "intro_seen")
		seenIntro := introSeen == "1"
		app.current = NewIntroScreen(width, height, seenIntro, app.loc)
		app.applyMascotFrame()
	}

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
			logutil.Info(notif)
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
				// Re-derive course from filtered list so sections reflect current license.
				for _, fc := range a.filteredCourses() {
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
			a.current = NewConnectChoiceScreen(a.loc, a.Width, a.Height, execAddr, a.lic != nil && a.lic.Licensed)
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
		a.current = NewConnectChoiceScreen(a.loc, a.Width, a.Height, execAddr, a.lic != nil && a.lic.Licensed)
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

	case NavigateToLicenseEntry:
		a.current = NewLicenseEntryScreen(a.loc, a.Width, a.Height, a.submitLicenseEntry)
		return a, a.current.Init()

	case NavigateToLicenseSummary:
		if a.lic != nil && a.lic.Licensed {
			a.current = NewLicenseSummaryScreen(a.loc, a.lic, a.courses, a.Width, a.Height)
		} else {
			a.current = NewLicenseEntryScreen(a.loc, a.Width, a.Height, a.submitLicenseEntry)
		}
		return a, a.current.Init()

	case licenseEntryDoneMsg:
		if msg.err == nil && msg.licState != nil {
			a.lic = mergeLicenseStates(a.lic, msg.licState)
			a.loadCourses()
			a.current = NewLicenseSummaryScreen(a.loc, a.lic, a.courses, a.Width, a.Height)
		} else if s, ok := a.current.(*licenseEntryScreen); ok {
			s.form.SetMessage(fmt.Sprintf(s.loc.T("license_entry.invalid"), msg.err), true)
		}
		return a, nil

	case NavigateToLessonMsg:
		a.activeCourseNodeID = msg.Lesson.ID
		a.current = NewLessonScreen(msg.Lesson, a.progress[msg.Lesson.ID], a.Width, a.Height, a.loc)
		a.applyMascotFrame()
		return a, a.current.Init()

	case NavigateToExerciseMsg:
		a.activeCourseNodeID = msg.Lesson.ID
		if msg.Lesson.IsPremium && !a.canAccessLesson(msg.Lesson) {
			a.switchToMenu()
			return a, nil
		}
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
		if msg.lesson.IsPremium && !a.canAccessLesson(msg.lesson) {
			a.switchToMenu()
			return a, nil
		}
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

	case launchGDBMsg:
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

func (a *App) courseIntroChecker() func(string) bool {
	return func(courseID string) bool {
		key := "course_intro_seen_" + courseID
		val, _ := a.db.GetUserSetting(a.username, key)
		return val != "1"
	}
}

func (a *App) submitLicenseEntry(val string) tea.Cmd {
	return func() tea.Msg {
		data, err := readLicenseValue(val)
		if err != nil {
			return licenseEntryDoneMsg{err: err}
		}
		state, info, err := license.CheckData(data)
		if err != nil {
			return licenseEntryDoneMsg{err: err}
		}
		if info.IsPersonal() {
			if err := a.db.RedeemPersonalLicense(a.username, info, data); err != nil {
				return licenseEntryDoneMsg{err: err}
			}
		}
		return licenseEntryDoneMsg{licState: state}
	}
}

func (a *App) loadRedeemedLicenses() {
	blobs, err := a.db.ListRedeemedLicenseBlobs(a.username)
	if err != nil {
		logutil.Warn("load redeemed licenses for %s: %v", a.username, err)
		return
	}
	for _, blob := range blobs {
		state, _, err := license.CheckData(blob)
		if err != nil {
			logutil.Warn("skip invalid redeemed license for %s: %v", a.username, err)
			continue
		}
		a.lic = mergeLicenseStates(a.lic, state)
	}
}

func mergeLicenseStates(base, extra *license.State) *license.State {
	if base == nil {
		if extra == nil {
			return nil
		}
		copy := *extra
		copy.UnlockedCourses = append([]string(nil), extra.UnlockedCourses...)
		if extra.CourseKey != nil {
			copy.CourseKey = append([]byte(nil), extra.CourseKey...)
		}
		return &copy
	}
	if extra == nil {
		return base
	}

	merged := *base
	merged.Licensed = base.Licensed || extra.Licensed
	if merged.Licensee == "" {
		merged.Licensee = extra.Licensee
	}
	if merged.LicenseID == "" {
		merged.LicenseID = extra.LicenseID
	}
	if merged.Username == "" {
		merged.Username = extra.Username
	}
	if merged.Email == "" {
		merged.Email = extra.Email
	}
	if merged.MaxStudents == 0 {
		merged.MaxStudents = extra.MaxStudents
	}
	if merged.ExpiresAt.IsZero() || (!extra.ExpiresAt.IsZero() && extra.ExpiresAt.After(merged.ExpiresAt)) {
		merged.ExpiresAt = extra.ExpiresAt
	}
	merged.HasMultiUser = base.HasMultiUser || extra.HasMultiUser
	merged.HasAdminUI = base.HasAdminUI || extra.HasAdminUI
	merged.HasInterviewQuestions = base.HasInterviewQuestions || extra.HasInterviewQuestions
	if len(merged.CourseKey) == 0 && len(extra.CourseKey) > 0 {
		merged.CourseKey = append([]byte(nil), extra.CourseKey...)
	}

	seen := make(map[string]bool, len(base.UnlockedCourses)+len(extra.UnlockedCourses))
	merged.UnlockedCourses = make([]string, 0, len(base.UnlockedCourses)+len(extra.UnlockedCourses))
	for _, id := range append(append([]string(nil), base.UnlockedCourses...), extra.UnlockedCourses...) {
		if seen[id] {
			continue
		}
		seen[id] = true
		merged.UnlockedCourses = append(merged.UnlockedCourses, id)
	}
	return &merged
}

func (a *App) buildMenuScreen(notifications []string) *MenuScreen {
	enrolled, _ := a.db.ListEnrollments(a.username)
	a.enrolledSet = make(map[string]bool, len(enrolled))
	for _, id := range enrolled {
		a.enrolledSet[id] = true
	}
	filtered := filterCourses(a.courses, a.lic, a.enrolledSet)
	canAccess := func(courseID string) bool {
		if a.isAdmin {
			return true
		}
		if a.lic != nil && a.lic.CanAccessCourse(courseID) {
			return true
		}
		if a.enrolledSet[courseID] {
			return true
		}
		return strings.Contains(courseID, "01-") || a.lic == nil
	}
	return NewMenuScreen(filtered, enrolled, canAccess, a.progress, notifications, a.username, a.streak, a.isAdmin, a.courseIntroChecker(), a.loc, a.Width, a.Height)
}

func (a *App) switchToMenu() {
	m := a.buildMenuScreen(a.pendingNotifications)
	a.pendingNotifications = nil
	a.activeCourseID = ""
	a.activeCourseNodeID = ""
	a.current = m
	a.applyMascotFrame()
}

// newMenuScreenWithCourse creates a MenuScreen already navigated into a course's
// lesson list. Used after a course intro completes.
func (a *App) newMenuScreenWithCourse(c lesson.Course) *MenuScreen {
	m := a.buildMenuScreen(nil)
	m.enterCourseDirectly(c)
	return m
}

// openCourse navigates to the appropriate view for the given course.
// Courses with layout: path open a PathScreen; all others open the MenuScreen
// lesson list. This is the single routing point for course entry.
func (a *App) openCourse(c lesson.Course) {
	a.activeCourseID = c.ID
	if c.Layout == lesson.CourseLayoutPath {
		a.current = NewPathScreen(c, a.progress, a.activeCourseNodeID, a.loc, a.Width, a.Height)
		a.applyMascotFrame()
		return
	}
	m := a.newMenuScreenWithCourse(c)
	a.current = m
	a.applyMascotFrame()
}

func (a *App) returnToActiveCourse() {
	if a.activeCourseID == "" {
		a.switchToMenu()
		return
	}
	for _, c := range a.filteredCourses() {
		if c.ID == a.activeCourseID {
			if len(c.Sections) == 0 {
				break
			}
			a.openCourse(c)
			return
		}
	}
	a.switchToMenu()
}

func (a *App) canAccessLesson(l lesson.Lesson) bool {
	if !l.IsPremium {
		return true
	}
	for _, c := range a.courses {
		for _, sec := range c.Sections {
			for _, cl := range sec.Lessons {
				if cl.ID == l.ID {
					courseFree := c.Order == 1 || strings.Contains(c.ID, "01-")
					return courseFree || (a.lic != nil && a.lic.CanAccessCourse(c.ID)) || a.enrolledSet[c.ID]
				}
			}
		}
	}
	return true // lesson not found in any course — allow by default
}

func (a *App) filteredCourses() []lesson.Course {
	return filterCourses(a.courses, a.lic, a.enrolledSet)
}

// filterCourses is the pure access-control filter extracted for testability.
func filterCourses(courses []lesson.Course, lic *license.State, enrolled map[string]bool) []lesson.Course {
	hasInterviews := lic != nil && lic.HasInterviewQuestions

	var filtered []lesson.Course
	for _, c := range courses {
		courseFree := c.Order == 1 || strings.Contains(c.ID, "01-")
		hasCourseAccess := courseFree || (lic != nil && lic.CanAccessCourse(c.ID)) || enrolled[c.ID]

		if c.EnrollmentRequired && !hasCourseAccess {
			continue
		}

		// Filter out interview sections if the license doesn't include them.
		if !hasInterviews {
			visible := make([]lesson.Section, 0, len(c.Sections))
			for _, s := range c.Sections {
				if s.Type == "interviews" {
					continue
				}
				visible = append(visible, s)
			}
			c.Sections = visible
		}

		if len(c.Sections) > 0 {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func (a *App) applyMascotFrame() {
	if setter, ok := a.current.(mascotFrameSetter); ok {
		setter.SetMascotFrame(a.mascotFrame)
	}
}

func (a *App) resizeCurrent() tea.Cmd {
	if a.current == nil {
		return nil
	}
	m, cmd := a.current.Update(tea.WindowSizeMsg{Width: a.Width, Height: a.Height})
	a.current = m
	a.applyMascotFrame()
	return cmd
}

func (a *App) loadCourses() {
	lang := a.uiLang
	var courses []lesson.Course
	if a.coursesDir != "" {
		if info, err := os.Stat(a.coursesDir); err == nil && info.IsDir() {
			var err error
			courses, err = lesson.LoadCoursesLang(a.coursesDir, lang)
			if err != nil {
				logutil.Warn("load courses from %s: %v", a.coursesDir, err)
			}

			// Scan for encrypted .course files.
			var courseKey []byte
			if a.lic != nil {
				courseKey = a.lic.CourseKey
			}
			encrypted, _ := lesson.ScanEncryptedCourses(a.coursesDir, lang, courseKey, courses)
			courses = append(courses, encrypted...)
		}
	}
	if len(courses) == 0 {
		c, err := lesson.LoadAsSingleCourseLang(a.lessonsDir, lang)
		if err == nil {
			courses = append(courses, c...)
		}
	}
	a.courses = courses
}

// findCourse searches the loaded courses for a course with the given ID.
func (a *App) findCourse(id string) (lesson.Course, bool) {
	for _, c := range a.courses {
		if c.ID == id {
			return c, true
		}
	}
	return lesson.Course{}, false
}

// findLesson searches the loaded courses for a lesson with the given ID.
func (a *App) findLesson(id string) (lesson.Lesson, bool) {
	for _, c := range a.courses {
		for _, sec := range c.Sections {
			for _, l := range sec.Lessons {
				if l.ID == id {
					return l, true
				}
			}
		}
	}
	return lesson.Lesson{}, false
}

func (a *App) findQuiz(id string) (lesson.Quiz, bool) {
	for _, c := range a.courses {
		for _, sec := range c.Sections {
			for _, q := range sec.Quizzes {
				if q.ID == id {
					return q, true
				}
			}
		}
	}
	return lesson.Quiz{}, false
}

// refreshCurrentScreenData pushes reloaded course/lesson data into the live
// screen after loadCourses has run with a new locale.
// Call this before SetLocale so the screen re-renders with fresh content.
func (a *App) refreshCurrentScreenData() {
	switch s := a.current.(type) {
	case *PathScreen:
		if c, ok := a.findCourse(a.activeCourseID); ok {
			s.SetCourse(c, a.progress)
		}
	case *LessonScreen:
		if l, ok := a.findLesson(s.lesson.ID); ok {
			s.lesson = l
		}
	case *ExerciseScreen:
		if l, ok := a.findLesson(s.lesson.ID); ok {
			s.SetLesson(l)
		}
	case *PreExerciseScreen:
		if l, ok := a.findLesson(s.lesson.ID); ok {
			newBeats := tutorialBeats(l.Tutorial)
			s.lesson = l
			// Update text of each beat in-place to preserve the current position.
			for i := range newBeats {
				if i < len(s.Beats) {
					s.Beats[i].Text = newBeats[i].Text
				}
			}
			if len(newBeats) != len(s.Beats) {
				s.Beats = newBeats
				s.BeatIdx = 0
				s.CharIdx = 0
			} else {
				s.CharIdx = 0 // re-type the current beat in the new language
			}
		}
	case *QuizScreen:
		if q, ok := a.findQuiz(s.quiz.ID); ok {
			s.SetQuiz(q)
		}
	}
}

// exercisePrefs returns the persisted mascot/timer visibility settings.
func (a *App) exercisePrefs() (showMascot, showTimer bool) {
	v, _ := a.db.GetUserSetting(a.username, "mascot_visible")
	showMascot = v != "0" // default visible ("" or "1")
	t, _ := a.db.GetUserSetting(a.username, "timer_visible")
	showTimer = t != "0"
	return
}

func (a *App) loadProgress() {
	progress, err := a.db.Progress(a.username)
	if err != nil {
		a.progress = make(map[string]int)
		return
	}
	a.progress = progress
}

func (a *App) resolveLanguage(l *lesson.Lesson) sandbox.Language {
	langName := l.Language
	if langName == "" {
		langName = "c"
	}
	if cached, ok := a.langCache[langName]; ok {
		return cached
	}
	lang := sandbox.GetLanguage(langName)
	if lang == nil {
		lang = sandbox.GetLanguage("c")
	}
	a.langCache[langName] = lang
	return lang
}

// totalLessons returns the total number of lessons across all courses.
func (a *App) totalLessons() int {
	n := 0
	for _, c := range a.filteredCourses() {
		n += c.TotalLessons + c.TotalQuizzes
	}
	return n
}

// allLessons returns a flat slice of all lessons across all courses.
func (a *App) allLessons() []lesson.Lesson {
	var out []lesson.Lesson
	for _, c := range a.filteredCourses() {
		for _, sec := range c.Sections {
			out = append(out, sec.Lessons...)
		}
	}
	return out
}

// nextLesson returns the next lesson after lessonID, preferring the active
// course context when available so progression does not jump across courses.
func (a *App) nextLesson(lessonID string) (lesson.Lesson, bool) {
	if a.activeCourseID != "" {
		for _, c := range a.filteredCourses() {
			if c.ID != a.activeCourseID {
				continue
			}
			for _, sec := range c.Sections {
				for i, l := range sec.Lessons {
					if l.ID == lessonID && i+1 < len(sec.Lessons) {
						return sec.Lessons[i+1], true
					}
				}
			}
			return lesson.Lesson{}, false
		}
	}
	lessons := a.allLessons()
	for i, l := range lessons {
		if l.ID == lessonID && i+1 < len(lessons) {
			return lessons[i+1], true
		}
	}
	return lesson.Lesson{}, false
}

func (a *App) preferredCourseNodeAfterCompletion(lessonID string) string {
	if next, ok := a.nextLesson(lessonID); ok {
		return next.ID
	}
	return lessonID
}

// checkGraduate grants the "graduate" achievement if all lessons in the course
// that contains lessonID have been completed (stars > 0).
func (a *App) checkGraduate(lessonID string) {
	for _, c := range a.filteredCourses() {
		found := false
		for _, sec := range c.Sections {
			for _, l := range sec.Lessons {
				if l.ID == lessonID {
					found = true
					break
				}
			}
			for _, q := range sec.Quizzes {
				if q.ID == lessonID {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			continue
		}
		// Found the course — check all lessons across all its sections.
		allDone := true
		for _, sec := range c.Sections {
			for _, cl := range sec.Lessons {
				if a.progress[cl.ID] == 0 {
					allDone = false
					break
				}
			}
			for _, q := range sec.Quizzes {
				if a.progress[q.ID] == 0 {
					allDone = false
					break
				}
			}
			if !allDone {
				break
			}
		}
		if allDone {
			if err := a.db.GrantAchievement(a.username, "graduate"); err == nil {
				if ach := achievementByID("graduate"); ach != nil {
					notif := fmt.Sprintf("%s %s — %s", ach.Icon, ach.Name, ach.Desc)
					a.pendingNotifications = append(a.pendingNotifications, notif)
				}
			}
		}
		return
	}
}

// grantAchievements evaluates a list of event strings and grants the
// corresponding achievements, appending notification strings for newly unlocked
// ones to a.pendingNotifications.
func (a *App) grantAchievements(events []string) {
	// Load already-earned achievements once to avoid duplicating notifications.
	earned, _ := a.db.GetAchievements(a.username)
	alreadyEarned := make(map[string]bool, len(earned))
	for _, id := range earned {
		alreadyEarned[id] = true
	}

	tryGrant := func(id string) {
		if alreadyEarned[id] {
			return
		}
		if err := a.db.GrantAchievement(a.username, id); err != nil {
			return
		}
		alreadyEarned[id] = true // prevent double-notification in one batch
		if ach := achievementByID(id); ach != nil {
			notif := fmt.Sprintf("%s %s — %s", ach.Icon, ach.Name, ach.Desc)
			a.pendingNotifications = append(a.pendingNotifications, notif)
		}
	}

	for _, event := range events {
		switch event {
		case "compile":
			tryGrant("first_compile")

		case "pass":
			tryGrant("first_pass")

		case "pass_1attempt":
			tryGrant("one_shot")

		case "pass_5attempts":
			tryGrant("comeback")

		case "pass_3star":
			tryGrant("perfect")
			// Check five_perfect threshold.
			count, err := a.db.CountLessonsWithMinStars(a.username, 3)
			if err == nil && count >= 5 {
				tryGrant("five_perfect")
			}

		case "pass_nowarnings":
			tryGrant("clean_coder")

		case "gdb":
			tryGrant("debugger")

		case "asm":
			tryGrant("disassembler")

		case "interactive":
			tryGrant("interactive")

		case "asan":
			tryGrant("sanitized")

		case "segfault_king":
			tryGrant("segfault_king")

		case "into_the_loop":
			tryGrant("into_the_loop")

		case "beer":
			tryGrant("beer")

		case "konami":
			tryGrant("konami")
		}
	}
}

func starsStyle(stars, maxStars int) string {
	if maxStars == 1 {
		if stars >= 1 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("★  ")
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○  ")
	}
	switch stars {
	case 3:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("★★★")
	case 2:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Render("★★☆")
	case 1:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("★☆☆")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○  ")
	}
}

// renderKey is the single source of truth for how a keyboard shortcut looks
// anywhere in the TUI. Both help bars and the keybindings overlay call this.
func renderKey(k string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBody)).
		Background(lipgloss.Color(ColorBorder)).
		Bold(true).
		Padding(0, 1).
		Render(k)
}

func renderHelpItem(key, label string) string {
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
	if strings.TrimSpace(key) == "" {
		return descStyle.Render(label)
	}
	if strings.TrimSpace(label) == "" {
		return renderKey(key)
	}
	return renderKey(key) + " " + descStyle.Render(label)
}

func actionHelpBar(loc *i18n.Locale, actions ...HelpAction) string {
	parts := make([]string, 0, len(actions))
	for _, item := range actions {
		action, ok := LookupKeyAction(item.ID)
		if !ok {
			continue
		}
		parts = append(parts, renderHelpItem(keyDisplay(action.Keys), actionLabel(loc, action, item.Args...)))
	}
	return strings.Join(parts, "  ")
}

func helpBar(items ...string) string {
	var parts []string
	for _, item := range items {
		idx := strings.Index(item, " ")
		if idx > 0 {
			parts = append(parts, renderHelpItem(item[:idx], item[idx+1:]))
		} else {
			parts = append(parts, renderHelpItem("", item))
		}
	}
	return strings.Join(parts, "  ")
}

func bold(s string) string {
	return lipgloss.NewStyle().Bold(true).Render(s)
}

func joinInlineParts(rtl bool, parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			filtered = append(filtered, part)
		}
	}
	if rtl {
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
	}
	return strings.Join(filtered, "  ")
}

// rtlAlignBlock right-aligns every line of s to width when the caller is in
// an RTL locale. Each line is padded independently so ANSI styles are preserved.
func rtlAlignBlock(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(line)
	}
	return strings.Join(lines, "\n")
}

func dim(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded)).Render(s)
}

func titleStyle(s string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorAccent)).
		Render(s)
}

func codeStyle(s string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Render(s)
}
