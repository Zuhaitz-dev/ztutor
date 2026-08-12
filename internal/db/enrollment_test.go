package db

import (
	"path/filepath"
	"testing"
)

func TestEnrollAndListEnrolledUsers(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.CreateUser("alice", "p", RoleStudent)
	db.CreateUser("bob", "p", RoleStudent)

	if err := db.Enroll("alice", "c-programming"); err != nil {
		t.Fatalf("Enroll alice: %v", err)
	}
	if err := db.Enroll("bob", "c-programming"); err != nil {
		t.Fatalf("Enroll bob: %v", err)
	}

	users, err := db.ListEnrolledUsers("c-programming")
	if err != nil {
		t.Fatalf("ListEnrolledUsers: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("enrolled users = %d, want 2", len(users))
	}

	found := map[string]bool{}
	for _, u := range users {
		found[u] = true
	}
	if !found["alice"] || !found["bob"] {
		t.Errorf("enrolled users = %v, want alice and bob", users)
	}
}

func TestEnroll_Idempotent(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.CreateUser("alice", "p", RoleStudent)

	if err := db.Enroll("alice", "c-programming"); err != nil {
		t.Fatalf("first Enroll: %v", err)
	}
	if err := db.Enroll("alice", "c-programming"); err != nil {
		t.Fatalf("duplicate Enroll should not error: %v", err)
	}

	count, err := db.CountEnrollments("c-programming")
	if err != nil {
		t.Fatalf("CountEnrollments: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (duplicate enroll should not add row)", count)
	}
}

func TestDeleteEnrollment(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.CreateUser("alice", "p", RoleStudent)
	db.Enroll("alice", "c-programming")

	if !db.IsEnrolled("alice", "c-programming") {
		t.Fatal("alice should be enrolled")
	}

	if err := db.DeleteEnrollment("alice", "c-programming"); err != nil {
		t.Fatalf("DeleteEnrollment: %v", err)
	}

	if db.IsEnrolled("alice", "c-programming") {
		t.Error("alice should not be enrolled after deletion")
	}

	users, _ := db.ListEnrolledUsers("c-programming")
	if len(users) != 0 {
		t.Errorf("enrolled users after deletion = %v, want empty", users)
	}
}

func TestDeleteEnrollment_NonExistent(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Deleting a non-existent enrollment should not error.
	if err := db.DeleteEnrollment("nobody", "c-programming"); err != nil {
		t.Errorf("DeleteEnrollment on non-existent row: %v", err)
	}
}

func TestIsEnrolled(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.CreateUser("alice", "p", RoleStudent)

	if db.IsEnrolled("alice", "c-programming") {
		t.Error("alice should not be enrolled before Enroll")
	}

	db.Enroll("alice", "c-programming")

	if !db.IsEnrolled("alice", "c-programming") {
		t.Error("alice should be enrolled after Enroll")
	}

	if db.IsEnrolled("alice", "python-basics") {
		t.Error("alice should not be enrolled in a different course")
	}
}

func TestListEnrollments_ByCourse(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.CreateUser("alice", "p", RoleStudent)
	db.Enroll("alice", "c-programming")
	db.Enroll("alice", "python-basics")

	courses, err := db.ListEnrollments("alice")
	if err != nil {
		t.Fatalf("ListEnrollments: %v", err)
	}
	if len(courses) != 2 {
		t.Errorf("enrollments = %v, want 2", courses)
	}

	found := map[string]bool{}
	for _, c := range courses {
		found[c] = true
	}
	if !found["c-programming"] || !found["python-basics"] {
		t.Errorf("enrollments = %v", courses)
	}
}

func TestCountEnrollments(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.CreateUser("alice", "p", RoleStudent)
	db.CreateUser("bob", "p", RoleStudent)
	db.CreateUser("carol", "p", RoleStudent)

	db.Enroll("alice", "c-programming")
	db.Enroll("bob", "c-programming")

	count, err := db.CountEnrollments("c-programming")
	if err != nil {
		t.Fatalf("CountEnrollments: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	count, _ = db.CountEnrollments("python-basics")
	if count != 0 {
		t.Errorf("count for empty course = %d, want 0", count)
	}

	db.Enroll("carol", "c-programming")
	count, _ = db.CountEnrollments("c-programming")
	if count != 3 {
		t.Errorf("count after third enroll = %d, want 3", count)
	}
}
