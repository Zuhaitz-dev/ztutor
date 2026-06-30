package tui

import (
	"reflect"
	"strings"
	"testing"

	"ztutor/internal/lesson"
	"ztutor/internal/license"
)

func TestSummarizeLicensedCourses(t *testing.T) {
	courses := []lesson.Course{
		{ID: "c-programming", Title: "C Programming"},
		{ID: "redis-capstone", Title: "redis-capstone"},
		{ID: "empty-title"},
	}

	tests := []struct {
		name          string
		lic           *license.State
		courses       []lesson.Course
		wantAll       bool
		wantInstalled []string
		wantMissing   []string
	}{
		{
			name:          "no license",
			lic:           nil,
			courses:       courses,
			wantInstalled: nil,
			wantMissing:   nil,
		},
		{
			name: "all installed",
			lic: &license.State{
				Licensed:        true,
				UnlockedCourses: []string{"c-programming", "redis-capstone"},
			},
			courses:       courses,
			wantInstalled: []string{"C Programming (c-programming)", "redis-capstone"},
			wantMissing:   nil,
		},
		{
			name: "some missing",
			lic: &license.State{
				Licensed:        true,
				UnlockedCourses: []string{"c-programming", "missing-course"},
			},
			courses:       courses,
			wantInstalled: []string{"C Programming (c-programming)"},
			wantMissing:   []string{"missing-course"},
		},
		{
			name: "none installed",
			lic: &license.State{
				Licensed:        true,
				UnlockedCourses: []string{"missing-a", "missing-b"},
			},
			courses:       courses,
			wantInstalled: nil,
			wantMissing:   []string{"missing-a", "missing-b"},
		},
		{
			name: "wildcard license",
			lic: &license.State{
				Licensed:        true,
				UnlockedCourses: []string{"*"},
			},
			courses:       courses,
			wantAll:       true,
			wantInstalled: nil,
			wantMissing:   nil,
		},
		{
			name: "title fallback to id",
			lic: &license.State{
				Licensed:        true,
				UnlockedCourses: []string{"empty-title"},
			},
			courses:       courses,
			wantInstalled: []string{"empty-title"},
			wantMissing:   nil,
		},
		{
			name: "duplicate course ids preserved",
			lic: &license.State{
				Licensed:        true,
				UnlockedCourses: []string{"c-programming", "c-programming"},
			},
			courses:       courses,
			wantInstalled: []string{"C Programming (c-programming)", "C Programming (c-programming)"},
			wantMissing:   nil,
		},
		{
			name: "licensed with empty entitlements",
			lic: &license.State{
				Licensed:        true,
				UnlockedCourses: nil,
			},
			courses:       courses,
			wantInstalled: nil,
			wantMissing:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeLicensedCourses(tt.lic, tt.courses)
			if got.allLicensed != tt.wantAll {
				t.Fatalf("allLicensed = %v, want %v", got.allLicensed, tt.wantAll)
			}
			if !reflect.DeepEqual(got.installed, tt.wantInstalled) {
				t.Fatalf("installed = %#v, want %#v", got.installed, tt.wantInstalled)
			}
			if !reflect.DeepEqual(got.missing, tt.wantMissing) {
				t.Fatalf("missing = %#v, want %#v", got.missing, tt.wantMissing)
			}
		})
	}
}

func TestLicenseSummaryWildcardRender(t *testing.T) {
	lic := &license.State{
		Licensed:        true,
		Licensee:        "Acme",
		UnlockedCourses: []string{"*"},
	}
	courses := []lesson.Course{
		{ID: "c-programming", Title: "C Programming"},
		{ID: "redis-capstone", Title: "Redis Capstone"},
	}
	screen := NewLicenseSummaryScreen(testLocale(), lic, courses, 80, 24)
	view := stripANSI(screen.View())
	for _, want := range []string{
		"Courses: all courses",
		"Installed now: 2 installed course(s)",
		"Not installed: licensed content may still need to be",
		"installed",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("license summary missing %q, got:\n%s", want, view)
		}
	}
}
