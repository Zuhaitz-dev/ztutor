package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestImportStudentsCSV(t *testing.T) {
	dir := t.TempDir()
	db2, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db2.Close()

	csv := []byte("alice,password1\nbob,password2\ncharlie,password3\n")
	result, err := db2.ImportStudentsCSV(csv, 0)
	if err != nil {
		t.Fatalf("ImportStudentsCSV: %v", err)
	}
	if len(result.Created) != 3 {
		t.Errorf("Created = %d, want 3", len(result.Created))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("Skipped = %d, want 0", len(result.Skipped))
	}

	hasUsers, err := db2.HasUsers()
	if err != nil {
		t.Fatal(err)
	}
	if !hasUsers {
		t.Error("expected users after import")
	}
	users, err := db2.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 3 {
		t.Errorf("ListUsers = %d, want 3", len(users))
	}
}

func TestImportStudentsCSV_SeatLimit(t *testing.T) {
	dir := t.TempDir()
	db2, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db2.Close()

	// Import 2 students with a limit of 2
	csv := []byte("alice,pw1\nbob,pw2\n")
	result, err := db2.ImportStudentsCSV(csv, 2)
	if err != nil {
		t.Fatalf("ImportStudentsCSV: %v", err)
	}
	if len(result.Created) != 2 {
		t.Errorf("Created = %d, want 2", len(result.Created))
	}

	// Add a third student — should hit limit
	csv2 := []byte("charlie,pw3\n")
	result2, err := db2.ImportStudentsCSV(csv2, 2)
	if err != nil {
		t.Fatalf("ImportStudentsCSV second pass: %v", err)
	}
	if len(result2.Created) != 0 {
		t.Errorf("Created = %d, want 0 (seat limit)", len(result2.Created))
	}
	if len(result2.Errors) == 0 {
		t.Error("should have seat limit error")
	}
}

func TestImportStudentsCSV_SkipDuplicates(t *testing.T) {
	dir := t.TempDir()
	db2, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db2.Close()

	csv := []byte("alice,pw1\nbob,pw2\n")
	_, err = db2.ImportStudentsCSV(csv, 0)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}

	// Import again — should skip existing users
	result, err := db2.ImportStudentsCSV(csv, 0)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if len(result.Created) != 0 {
		t.Errorf("Created = %d, want 0 (all duplicates)", len(result.Created))
	}
	if len(result.Skipped) != 2 {
		t.Errorf("Skipped = %d, want 2", len(result.Skipped))
	}
}

func TestImportStudentsCSV_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	db2, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db2.Close()

	result, err := db2.ImportStudentsCSV([]byte{}, 0)
	if err != nil {
		t.Fatalf("ImportStudentsCSV empty: %v", err)
	}
	if len(result.Created) != 0 {
		t.Errorf("Created = %d, want 0", len(result.Created))
	}
}

func TestExportProgressCSV(t *testing.T) {
	dir := t.TempDir()
	db2, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db2.Close()

	// Create users and progress
	if err := db2.CreateUser("alice", "pw", RoleStudent); err != nil {
		t.Fatal(err)
	}
	if err := db2.CreateUser("bob", "pw", RoleStudent); err != nil {
		t.Fatal(err)
	}
	db2.MarkStarted("alice", "lesson-1")
	db2.MarkCompleted("alice", "lesson-1", 3)
	db2.MarkStarted("bob", "lesson-1")
	db2.MarkCompleted("bob", "lesson-1", 1)
	db2.MarkStarted("bob", "lesson-2")
	db2.MarkCompleted("bob", "lesson-2", 2)

	data, err := db2.ExportProgressCSV()
	if err != nil {
		t.Fatalf("ExportProgressCSV: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "alice") {
		t.Error("export should contain alice")
	}
	if !strings.Contains(content, "bob") {
		t.Error("export should contain bob")
	}
	if !strings.Contains(content, "lesson-1") {
		t.Error("export should contain lesson-1")
	}
	if !strings.Contains(content, "lesson-2") {
		t.Error("export should contain lesson-2")
	}
}

func TestExportRosterCSV(t *testing.T) {
	dir := t.TempDir()
	db2, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db2.Close()

	if err := db2.CreateUser("alice", "pw", RoleStudent); err != nil {
		t.Fatal(err)
	}
	db2.MarkStarted("alice", "lesson-1")
	db2.MarkCompleted("alice", "lesson-1", 3)
	db2.SetUserSetting("alice", "keymap", "vim")

	data, err := db2.ExportRosterCSV()
	if err != nil {
		t.Fatalf("ExportRosterCSV: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "alice") {
		t.Error("roster should contain alice")
	}
}
