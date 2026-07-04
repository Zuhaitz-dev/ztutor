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
	// Use the project's courses/ directory when running from the repo root.
	// If it doesn't exist, fall back to a minimal temp course so the tool
	// still works after a fresh checkout without course content.
	coursesDir := "./courses"
	var cleanup func()
	if !hasCourseManifests(coursesDir) {
		coursesDir, cleanup = setupTempCourses()
	}
	if cleanup != nil {
		defer cleanup()
	}

	tmpDir, err := os.MkdirTemp("", "ztutor-tuitest-db-")
	if err != nil {
		fmt.Println("tempdir error:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	dbase, err := db.Open(tmpDir + "/test.db")
	if err != nil {
		fmt.Println("db error:", err)
		os.Exit(1)
	}
	defer dbase.Close()

	app := tui.NewApp("tester", coursesDir, "", dbase, nil, 80, 24, "default", nil, nil)
	p := tea.NewProgram(app, tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout))
	if _, err := p.Run(); err != nil {
		fmt.Println("tui error:", err)
	}
}

// setupTempCourses creates a minimal course tree so tuitest works without the
// real course content. Returns the courses dir path and a cleanup function.
func setupTempCourses() (string, func()) {
	dir, err := os.MkdirTemp("", "ztutor-tuitest-courses-")
	if err != nil {
		fmt.Println("tempdir error:", err)
		os.Exit(1)
	}

	lessonDir := dir + "/01-intro/lessons/01-hello"
	if err := os.MkdirAll(lessonDir, 0755); err != nil {
		fmt.Println("mkdir error:", err)
		os.Exit(1)
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

	if err := os.WriteFile(dir+"/01-intro/course.yaml", []byte(courseYAML), 0644); err != nil {
		fmt.Println("write course.yaml error:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(lessonDir+"/lesson.md", []byte(lessonMD), 0644); err != nil {
		fmt.Println("write lesson.md error:", err)
		os.Exit(1)
	}

	return dir, func() { os.RemoveAll(dir) }
}

func hasCourseManifests(dir string) bool {
	matches, err := filepath.Glob(filepath.Join(dir, "*", "course.yaml"))
	return err == nil && len(matches) > 0
}
