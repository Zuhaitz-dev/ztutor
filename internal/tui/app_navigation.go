package tui

import (
	"ztutor/internal/lesson"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) courseIntroChecker() func(string) bool {
	return func(courseID string) bool {
		key := "course_intro_seen_" + courseID
		val, _ := a.db.GetUserSetting(a.username, key)
		return val != "1"
	}
}

func (a *App) buildMenuScreen(notifications []string) *MenuScreen {
	return NewMenuScreen(a.courses, a.progress, notifications, a.username, a.streak, a.isAdmin, a.courseIntroChecker(), a.loc, a.Width, a.Height)
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
	for _, c := range a.courses {
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
