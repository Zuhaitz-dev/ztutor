package lesson

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter_Full(t *testing.T) {
	raw := `---
difficulty: intermediate
tags: [memory, pointers]
companies:
  - Google
  - Amazon
references:
  - K&R 2nd ed., §5.1
  - man 3 malloc
tutorial:
  - "Step one: understand the heap."
  - "Step two: free what you malloc."
---
# Dynamic Memory

Content goes here.`

	fm, body := parseFrontmatter(raw)

	if fm.Difficulty != "intermediate" {
		t.Errorf("difficulty = %q, want %q", fm.Difficulty, "intermediate")
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "memory" || fm.Tags[1] != "pointers" {
		t.Errorf("tags = %v, want [memory pointers]", fm.Tags)
	}
	if len(fm.Companies) != 2 || fm.Companies[0] != "Google" || fm.Companies[1] != "Amazon" {
		t.Errorf("companies = %v, want [Google Amazon]", fm.Companies)
	}
	if len(fm.References) != 2 || fm.References[0] != "K&R 2nd ed., §5.1" {
		t.Errorf("references = %v", fm.References)
	}
	if len(fm.Tutorial) != 2 || fm.Tutorial[0] != "Step one: understand the heap." {
		t.Errorf("tutorial = %v", fm.Tutorial)
	}
	if body != "# Dynamic Memory\n\nContent goes here." {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontmatter_Minimal(t *testing.T) {
	raw := `---
difficulty: beginner
tags: [io]
---
# Hello World

Just print stuff.`

	fm, body := parseFrontmatter(raw)
	if fm.Difficulty != "beginner" {
		t.Errorf("difficulty = %q", fm.Difficulty)
	}
	if len(fm.Tags) != 1 || fm.Tags[0] != "io" {
		t.Errorf("tags = %v", fm.Tags)
	}
	if body != "# Hello World\n\nJust print stuff." {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontmatter_None(t *testing.T) {
	raw := "# No Frontmatter\n\nJust content."

	fm, body := parseFrontmatter(raw)
	if fm.Difficulty != "" {
		t.Errorf("difficulty = %q, want empty", fm.Difficulty)
	}
	if fm.Tags != nil {
		t.Errorf("tags = %v, want nil", fm.Tags)
	}
	if body != raw {
		t.Errorf("body = %q, want unchanged", body)
	}
}

func TestParseFrontmatter_Premium(t *testing.T) {
	raw := `---
difficulty: advanced
premium: true
tags: [memory]
---
# Pointers`

	fm, body := parseFrontmatter(raw)
	if !fm.Premium {
		t.Error("premium = false, want true")
	}
	if body != "# Pointers" {
		t.Errorf("body = %q", body)
	}

	// Premium omitted = false.
	raw2 := `---
difficulty: beginner
---
# Free`

	fm2, _ := parseFrontmatter(raw2)
	if fm2.Premium {
		t.Error("premium = true, want false when omitted")
	}
}

func TestParseFrontmatter_EmptyKeys(t *testing.T) {
	raw := `---
difficulty: advanced
---
# Advanced`

	fm, body := parseFrontmatter(raw)
	if fm.Difficulty != "advanced" {
		t.Errorf("difficulty = %q", fm.Difficulty)
	}
	if body != "# Advanced" {
		t.Errorf("body = %q", body)
	}
}

func TestLoadLang(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "lesson.md"), []byte(`---
difficulty: beginner
tags: [basics, io, compilation]
references:
  - K&R 2nd ed., §1.1
  - man 3 printf
---
# 01: Test Lesson

Some content.`), 0644)

	os.WriteFile(filepath.Join(dir, "exercise.c"), []byte(`#include <stdio.h>
int main(void) {
    printf("hello\n");
    return 0;
}`), 0644)

	os.WriteFile(filepath.Join(dir, "expected.txt"), []byte("hello\n"), 0644)
	os.WriteFile(filepath.Join(dir, "hints.txt"), []byte("Hint 1\n---\nHint 2"), 0644)
	os.WriteFile(filepath.Join(dir, "trivia.txt"), []byte("Trivium 1\n---\nTrivium 2"), 0644)

	lesson, err := LoadLang(dir, "en")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if lesson.ID != filepath.Base(dir) {
		t.Errorf("ID = %q", lesson.ID)
	}
	if lesson.Title != "01: Test Lesson" {
		t.Errorf("Title = %q", lesson.Title)
	}
	if lesson.Difficulty != "beginner" {
		t.Errorf("Difficulty = %q", lesson.Difficulty)
	}
	if len(lesson.Tags) != 3 {
		t.Errorf("Tags = %v", lesson.Tags)
	}
	if len(lesson.References) != 2 {
		t.Errorf("References = %v", lesson.References)
	}
	if string(lesson.Exercise) == "" {
		t.Error("Exercise is empty")
	}
	if len(lesson.Hints) != 2 {
		t.Errorf("Hints = %v (len=%d)", lesson.Hints, len(lesson.Hints))
	}
	if len(lesson.Trivia) != 2 {
		t.Errorf("Trivia = %v (len=%d)", lesson.Trivia, len(lesson.Trivia))
	}
	if len(lesson.Tests) != 1 {
		t.Errorf("Tests = %d, want 1", len(lesson.Tests))
	}
	if lesson.Tests[0].Expected != "hello\n" {
		t.Errorf("Expected = %q", lesson.Tests[0].Expected)
	}
}

func TestLoad_MultiTest(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "lesson.md"), []byte(`# Multi-test`), 0644)
	os.WriteFile(filepath.Join(dir, "exercise.c"), []byte(`int main(void) { return 0; }`), 0644)
	os.WriteFile(filepath.Join(dir, "expected.txt"), []byte("test1"), 0644)
	os.WriteFile(filepath.Join(dir, "stdin.txt"), []byte("input1"), 0644)
	os.WriteFile(filepath.Join(dir, "expected.2.txt"), []byte("test2"), 0644)
	os.WriteFile(filepath.Join(dir, "stdin.2.txt"), []byte("input2"), 0644)
	os.WriteFile(filepath.Join(dir, "expected.3.txt"), []byte("test3"), 0644)

	lesson, err := LoadLang(dir, "en")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(lesson.Tests) != 3 {
		t.Fatalf("Tests = %d, want 3", len(lesson.Tests))
	}
	if lesson.Tests[0].Expected != "test1" {
		t.Errorf("Test 1 expected = %q", lesson.Tests[0].Expected)
	}
	if lesson.Tests[0].Stdin != "input1" {
		t.Errorf("Test 1 stdin = %q", lesson.Tests[0].Stdin)
	}
	if lesson.Tests[1].Expected != "test2" {
		t.Errorf("Test 2 expected = %q", lesson.Tests[1].Expected)
	}
	if lesson.Tests[1].Stdin != "input2" {
		t.Errorf("Test 2 stdin = %q", lesson.Tests[1].Stdin)
	}
	if lesson.Tests[2].Expected != "test3" {
		t.Errorf("Test 3 expected = %q", lesson.Tests[2].Expected)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"# Hello World\n\nBody", "Hello World"},
		{"#   Padded   \n\nBody", "Padded"},
		{"No title here", "No title here"},
		{"Plain text\nmore text", "Plain text"},
	}

	for _, tt := range tests {
		got := extractTitle(tt.content)
		if got != tt.want {
			t.Errorf("extractTitle(%q) = %q, want %q", tt.content, got, tt.want)
		}
	}
}

func TestSplitBlocks(t *testing.T) {
	tests := []struct {
		raw  string
		want []string
	}{
		{"Block 1\n---\nBlock 2", []string{"Block 1", "Block 2"}},
		{"Single block", []string{"Single block"}},
		{"Block 1\n---\nBlock 2\n---\nBlock 3", []string{"Block 1", "Block 2", "Block 3"}},
	}

	for _, tt := range tests {
		got := splitBlocks(tt.raw)
		if len(got) != len(tt.want) {
			t.Errorf("splitBlocks(%q) = %v (len=%d), want %v (len=%d)", tt.raw, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitBlocks(%q)[%d] = %q, want %q", tt.raw, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseTestFilename(t *testing.T) {
	tests := []struct {
		name    string
		wantNum int
		wantFld string
		wantOK  bool
	}{
		{"expected.2.txt", 2, "expected", true},
		{"stdin.5.txt", 5, "stdin", true},
		{"args.3.txt", 3, "args", true},
		{"expected.txt", 0, "", false},
		{"expected.1.txt", 0, "", false},
		{"other.txt", 0, "", false},
		{"expected.abc.txt", 0, "", false},
	}

	for _, tt := range tests {
		num, field, ok := parseTestFilename(tt.name)
		if ok != tt.wantOK || num != tt.wantNum || field != tt.wantFld {
			t.Errorf("parseTestFilename(%q) = (%d, %q, %v), want (%d, %q, %v)",
				tt.name, num, field, ok, tt.wantNum, tt.wantFld, tt.wantOK)
		}
	}
}
