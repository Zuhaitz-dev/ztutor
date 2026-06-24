package tui

import (
	"testing"

	"ztutor/internal/lesson"
	"ztutor/internal/license"
)

func makeSection(id, typ string) lesson.Section {
	return lesson.Section{
		ID:      id,
		Title:   id,
		Type:    typ,
		Lessons: []lesson.Lesson{{ID: "l1", Title: "Lesson 1"}},
	}
}

func makeCourse(id string, order int, enrollmentRequired bool, sections ...lesson.Section) lesson.Course {
	return lesson.Course{
		ID:                 id,
		Title:              id,
		Order:              order,
		EnrollmentRequired: enrollmentRequired,
		Sections:           sections,
	}
}

func licensedFor(courseIDs ...string) *license.State {
	return &license.State{
		Licensed:        true,
		UnlockedCourses: courseIDs,
	}
}

func licenseWithInterviews(courseIDs ...string) *license.State {
	return &license.State{
		Licensed:              true,
		UnlockedCourses:       courseIDs,
		HasInterviewQuestions: true,
	}
}

// TestFilterCourses_NilLicense: free courses visible, enrollment-required ones hidden.
func TestFilterCourses_NilLicense(t *testing.T) {
	courses := []lesson.Course{
		makeCourse("01-c-intro", 1, false, makeSection("s1", "exercises")),
		makeCourse("c-advanced", 2, true, makeSection("s1", "exercises")),
	}

	got := filterCourses(courses, nil, nil)
	if len(got) != 1 {
		t.Fatalf("courses = %d, want 1 (only free course)", len(got))
	}
	if got[0].ID != "01-c-intro" {
		t.Errorf("course ID = %q, want 01-c-intro", got[0].ID)
	}
}

// TestFilterCourses_NilLicense_NoPanic: calling with nil lic must not panic.
func TestFilterCourses_NilLicense_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("filterCourses panicked with nil license: %v", r)
		}
	}()

	courses := []lesson.Course{
		makeCourse("premium-course", 2, true, makeSection("s1", "exercises")),
	}
	filterCourses(courses, nil, nil)
}

// TestFilterCourses_WithLicense: licensed course appears.
func TestFilterCourses_WithLicense(t *testing.T) {
	courses := []lesson.Course{
		makeCourse("01-c-intro", 1, false, makeSection("s1", "exercises")),
		makeCourse("c-advanced", 2, true, makeSection("s1", "exercises")),
	}

	got := filterCourses(courses, licensedFor("c-advanced"), nil)
	if len(got) != 2 {
		t.Fatalf("courses = %d, want 2", len(got))
	}
}

// TestFilterCourses_InterviewSectionsHiddenWithoutLicense
func TestFilterCourses_InterviewSectionsHiddenWithoutLicense(t *testing.T) {
	courses := []lesson.Course{
		makeCourse("01-c-intro", 1, false,
			makeSection("exercises", "exercises"),
			makeSection("interviews", "interviews"),
		),
	}

	got := filterCourses(courses, nil, nil)
	if len(got) != 1 {
		t.Fatalf("courses = %d, want 1", len(got))
	}
	if len(got[0].Sections) != 1 {
		t.Errorf("sections = %d, want 1 (interview section should be hidden)", len(got[0].Sections))
	}
	if got[0].Sections[0].ID != "exercises" {
		t.Errorf("remaining section = %q, want exercises", got[0].Sections[0].ID)
	}
}

// TestFilterCourses_InterviewSectionsVisibleWithLicense
func TestFilterCourses_InterviewSectionsVisibleWithLicense(t *testing.T) {
	courses := []lesson.Course{
		makeCourse("01-c-intro", 1, false,
			makeSection("exercises", "exercises"),
			makeSection("interviews", "interviews"),
		),
	}

	got := filterCourses(courses, licenseWithInterviews(), nil)
	if len(got) != 1 {
		t.Fatalf("courses = %d, want 1", len(got))
	}
	if len(got[0].Sections) != 2 {
		t.Errorf("sections = %d, want 2 (both visible with interview license)", len(got[0].Sections))
	}
}

// TestFilterCourses_CourseDroppedWhenAllSectionsFiltered: course with only
// interview sections disappears entirely for unlicensed users.
func TestFilterCourses_CourseDroppedWhenAllSectionsFiltered(t *testing.T) {
	courses := []lesson.Course{
		makeCourse("01-c-intro", 1, false,
			makeSection("interviews", "interviews"),
		),
	}

	got := filterCourses(courses, nil, nil)
	if len(got) != 0 {
		t.Errorf("courses = %d, want 0 (all sections filtered → course dropped)", len(got))
	}
}

// TestFilterCourses_OriginalSliceNotMutated: the input slice must not be modified.
func TestFilterCourses_OriginalSliceNotMutated(t *testing.T) {
	courses := []lesson.Course{
		makeCourse("01-c-intro", 1, false,
			makeSection("exercises", "exercises"),
			makeSection("interviews", "interviews"),
		),
	}
	originalSectionCount := len(courses[0].Sections)

	filterCourses(courses, nil, nil)

	if len(courses[0].Sections) != originalSectionCount {
		t.Errorf("original course mutated: sections = %d, want %d", len(courses[0].Sections), originalSectionCount)
	}
}

// TestFilterCourses_Empty: empty input returns empty output.
func TestFilterCourses_Empty(t *testing.T) {
	got := filterCourses(nil, nil, nil)
	if len(got) != 0 {
		t.Errorf("got = %v, want nil/empty", got)
	}
}

// TestFilterCourses_EnrolledUnlocksRequiredCourse: enrollment grants access to
// an enrollment-required course even without a license.
func TestFilterCourses_EnrolledUnlocksRequiredCourse(t *testing.T) {
	courses := []lesson.Course{
		makeCourse("c-advanced", 2, true, makeSection("s1", "exercises")),
	}
	enrolled := map[string]bool{"c-advanced": true}

	got := filterCourses(courses, nil, enrolled)
	if len(got) != 1 {
		t.Fatalf("courses = %d, want 1 (enrolled user should see required course)", len(got))
	}
	if got[0].ID != "c-advanced" {
		t.Errorf("course ID = %q, want c-advanced", got[0].ID)
	}
}

// TestFilterCourses_EnrolledOtherCourseDoesNotUnlock: enrollment in course A
// does not grant access to unrelated enrollment-required course B.
func TestFilterCourses_EnrolledOtherCourseDoesNotUnlock(t *testing.T) {
	courses := []lesson.Course{
		makeCourse("c-advanced", 2, true, makeSection("s1", "exercises")),
	}
	enrolled := map[string]bool{"c-other": true}

	got := filterCourses(courses, nil, enrolled)
	if len(got) != 0 {
		t.Errorf("courses = %d, want 0 (enrolled in different course should not unlock c-advanced)", len(got))
	}
}
