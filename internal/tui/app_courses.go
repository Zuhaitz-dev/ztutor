package tui

import (
	"fmt"
	"os"

	"ztutor/internal/lesson"
	"ztutor/internal/logutil"
	"ztutor/internal/sandbox"
)

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
	for _, c := range a.courses {
		n += c.TotalLessons + c.TotalQuizzes
	}
	return n
}

// allLessons returns a flat slice of all lessons across all courses.
func (a *App) allLessons() []lesson.Lesson {
	var out []lesson.Lesson
	for _, c := range a.courses {
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
		for _, c := range a.courses {
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
	for _, c := range a.courses {
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
