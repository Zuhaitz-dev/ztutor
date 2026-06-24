package tui

import (
	"testing"

	"ztutor/internal/lesson"
	"ztutor/internal/license"
)

func TestSectionCounts(t *testing.T) {
	c := &lesson.Course{
		Sections: []lesson.Section{
			{Type: "exercises", Lessons: []lesson.Lesson{{ID: "l1"}, {ID: "l2"}}},
			{Type: "interviews", Lessons: []lesson.Lesson{{ID: "i1"}}},
			{Type: "challenges", Challenges: []lesson.Challenge{{ID: "c1"}, {ID: "c2"}}},
		},
	}
	lessons, interviews, challenges := sectionCounts(c)
	if lessons != 2 {
		t.Errorf("lessons = %d, want 2", lessons)
	}
	if interviews != 1 {
		t.Errorf("interviews = %d, want 1", interviews)
	}
	if challenges != 2 {
		t.Errorf("challenges = %d, want 2", challenges)
	}
}

func TestSectionCounts_Empty(t *testing.T) {
	c := &lesson.Course{}
	lessons, interviews, challenges := sectionCounts(c)
	if lessons != 0 || interviews != 0 || challenges != 0 {
		t.Errorf("empty course should have 0 counts: l=%d i=%d c=%d", lessons, interviews, challenges)
	}
}

func TestCourseProgressCounts(t *testing.T) {
	c := &lesson.Course{
		Sections: []lesson.Section{
			{
				Type:    "exercises",
				Lessons: []lesson.Lesson{{ID: "l1"}, {ID: "l2"}, {ID: "l3"}},
			},
			{
				Type:       "challenges",
				Challenges: []lesson.Challenge{{ID: "c1"}},
			},
		},
	}
	progress := map[string]int{
		"l1": 3,
		"l2": 1,
		"c1": 2,
	}
	done, total := courseProgressCounts(c, progress)
	if done != 3 {
		t.Errorf("done = %d, want 3", done)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
}

func TestCourseProgressCounts_Empty(t *testing.T) {
	c := &lesson.Course{}
	done, total := courseProgressCounts(c, nil)
	if done != 0 || total != 0 {
		t.Errorf("empty should be 0: done=%d total=%d", done, total)
	}
}

func TestFilterCourses_EnrollmentRequired(t *testing.T) {
	courses := []lesson.Course{
		{ID: "free", EnrollmentRequired: false, Sections: []lesson.Section{{Type: "exercises", Lessons: []lesson.Lesson{{ID: "a"}}}}},
		{ID: "premium", EnrollmentRequired: true, Sections: []lesson.Section{{Type: "exercises", Lessons: []lesson.Lesson{{ID: "b"}}}}},
	}
	enrolled := map[string]bool{"premium": true}
	result := filterCourses(courses, nil, enrolled)
	if len(result) != 2 {
		t.Fatalf("expected 2 courses, got %d", len(result))
	}
}

func TestFilterCourses_EnrolledOnly(t *testing.T) {
	courses := []lesson.Course{
		{ID: "free", EnrollmentRequired: false, Sections: []lesson.Section{{Type: "exercises", Lessons: []lesson.Lesson{{ID: "a"}}}}},
		{ID: "premium", EnrollmentRequired: true, Sections: []lesson.Section{{Type: "exercises", Lessons: []lesson.Lesson{{ID: "b"}}}}},
	}
	enrolled := map[string]bool{}
	result := filterCourses(courses, nil, enrolled)
	if len(result) != 1 {
		t.Fatalf("expected 1 course (free only), got %d", len(result))
	}
	if result[0].ID != "free" {
		t.Errorf("free course not in result: %v", result)
	}
}

func TestFilterCourses_LicenseGated(t *testing.T) {
	courses := []lesson.Course{
		{ID: "c1", EnrollmentRequired: true, Sections: []lesson.Section{{Type: "exercises", Lessons: []lesson.Lesson{{ID: "a"}}}}},
		{ID: "c2", EnrollmentRequired: false, Sections: []lesson.Section{{Type: "exercises", Lessons: []lesson.Lesson{{ID: "b"}}}}},
	}
	lic := &license.State{
		Licensed:        true,
		HasMultiUser:    true,
		UnlockedCourses: []string{"c1"},
	}
	result := filterCourses(courses, lic, map[string]bool{"c1": true})
	if len(result) != 2 {
		t.Fatalf("expected 2 courses, got %d", len(result))
	}
}

func TestFilterCourses_LicenseGatedWithoutMultiUser(t *testing.T) {
	courses := []lesson.Course{
		{ID: "c1", EnrollmentRequired: true, Sections: []lesson.Section{{Type: "exercises", Lessons: []lesson.Lesson{{ID: "a"}}}}},
		{ID: "c2", EnrollmentRequired: false, Sections: []lesson.Section{{Type: "exercises", Lessons: []lesson.Lesson{{ID: "b"}}}}},
	}
	lic := &license.State{
		Licensed:        true,
		HasMultiUser:    false,
		UnlockedCourses: []string{}, // course c1 is not unlocked
	}
	result := filterCourses(courses, lic, map[string]bool{"c1": true})
	if len(result) != 2 {
		t.Fatalf("expected 2 courses (c1 via enrollment, c2 free), got %d", len(result))
	}
}
