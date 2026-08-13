package main

import (
	"fmt"
	"os"
	"path/filepath"

	"ztutor/internal/db"
	"ztutor/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if err := run(); err != nil {
		fmt.Println("tuitest error:", err)
		os.Exit(1)
	}
}

func run() error {
	// Use the project's courses/ directory when running from the repo root.
	// If it doesn't exist, fall back to a minimal temp course so the tool
	// still works after a fresh checkout without course content.
	coursesDir := "./courses"
	var cleanup func()
	if !hasCourseManifests(coursesDir) {
		var err error
		coursesDir, cleanup, err = setupTempCourses()
		if err != nil {
			return err
		}
	}
	if cleanup != nil {
		defer cleanup()
	}

	tmpDir, err := os.MkdirTemp("", "ztutor-tuitest-db-")
	if err != nil {
		return fmt.Errorf("tempdir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dbase, err := db.Open(tmpDir + "/test.db")
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer dbase.Close()

	app := tui.NewApp("tester", coursesDir, "", dbase, 80, 24, "default")
	p := tea.NewProgram(app, tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

// setupTempCourses creates a minimal course tree so tuitest works without the
// real course content. Returns the courses dir path and a cleanup function.
func setupTempCourses() (string, func(), error) {
	dir, err := os.MkdirTemp("", "ztutor-tuitest-courses-")
	if err != nil {
		return "", nil, fmt.Errorf("tempdir: %w", err)
	}

	lessonDir := dir + "/01-intro/lessons/01-hello"
	if err := os.MkdirAll(lessonDir, 0755); err != nil {
		return "", nil, fmt.Errorf("mkdir: %w", err)
	}

	courseYAML := `id: 01-intro
title: Intro to C
description: A minimal smoke-test course.
language: c
order: 1
sections:
  - id: lessons
    title: Lessons
    type: exercises
    dir: lessons/
toolchain:
  source_extension: .c
  syntax_highlighting: c
`
	lessonMD := "# Hello World\n\nWrite a program that prints \"Hello, World!\".\n\n## Exercise\n\n```c\n#include <stdio.h>\n\nint main(void) {\n    printf(\"Hello, World!\\n\");\n    return 0;\n}\n```\n"

	if err := os.WriteFile(dir+"/01-intro/course.yaml", []byte(courseYAML), 0600); err != nil {
		return "", nil, fmt.Errorf("write course.yaml: %w", err)
	}
	if err := os.WriteFile(lessonDir+"/lesson.md", []byte(lessonMD), 0600); err != nil {
		return "", nil, fmt.Errorf("write lesson.md: %w", err)
	}

	return dir, func() { os.RemoveAll(dir) }, nil
}

func hasCourseManifests(dir string) bool {
	matches, err := filepath.Glob(filepath.Join(dir, "*", "course.yaml"))
	return err == nil && len(matches) > 0
}
