package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("db file missing: %v", err)
	}
}

func TestMarkStarted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.MarkStarted("alice", "lesson-01"); err != nil {
		t.Fatalf("MarkStarted: %v", err)
	}

	progress, err := db.Progress("alice")
	if err != nil {
		t.Fatalf("Progress: %v", err)
	}
	if progress["lesson-01"] != 0 {
		t.Errorf("started but not completed lesson should have 0 stars, got %d", progress["lesson-01"])
	}
}

func TestMarkCompleted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.MarkCompleted("alice", "lesson-01", 3); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}

	progress, err := db.Progress("alice")
	if err != nil {
		t.Fatalf("Progress: %v", err)
	}
	if progress["lesson-01"] != 3 {
		t.Errorf("stars = %d, want 3", progress["lesson-01"])
	}
}

func TestMarkCompleted_OnlyImproves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.MarkCompleted("alice", "lesson-01", 3); err != nil {
		t.Fatalf("MarkCompleted alice 3: %v", err)
	}
	if err := db.MarkCompleted("alice", "lesson-01", 1); err != nil {
		t.Fatalf("MarkCompleted alice 1: %v", err)
	}

	progress, _ := db.Progress("alice")
	if progress["lesson-01"] != 3 {
		t.Errorf("stars = %d, want 3 (should not downgrade)", progress["lesson-01"])
	}

	if err := db.MarkCompleted("alice", "lesson-01", 2); err != nil {
		t.Fatalf("MarkCompleted alice 2: %v", err)
	}
	progress, _ = db.Progress("alice")
	if progress["lesson-01"] != 3 {
		t.Errorf("stars = %d, want 3 (should not downgrade)", progress["lesson-01"])
	}

	if err := db.MarkCompleted("bob", "lesson-01", 1); err != nil {
		t.Fatalf("MarkCompleted bob 1: %v", err)
	}
	progress, err = db.Progress("bob")
	if err != nil {
		t.Fatalf("Progress bob: %v", err)
	}
	if progress["lesson-01"] != 1 {
		t.Errorf("bob stars = %d, want 1", progress["lesson-01"])
	}
}

func TestProgress_MultiUser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.MarkCompleted("alice", "l1", 3)
	db.MarkCompleted("alice", "l2", 2)
	db.MarkCompleted("bob", "l1", 1)
	db.MarkCompleted("bob", "l3", 3)

	alice, _ := db.Progress("alice")
	bob, _ := db.Progress("bob")

	if len(alice) != 2 || alice["l1"] != 3 || alice["l2"] != 2 {
		t.Errorf("alice progress = %v", alice)
	}
	if len(bob) != 2 || bob["l1"] != 1 || bob["l3"] != 3 {
		t.Errorf("bob progress = %v", bob)
	}
}

func TestLeaderboard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.MarkCompleted("alice", "l1", 3)
	db.MarkCompleted("alice", "l2", 3)
	db.MarkCompleted("bob", "l1", 2)
	db.MarkCompleted("bob", "l2", 2)
	db.MarkCompleted("bob", "l3", 2)
	db.MarkCompleted("carol", "l1", 1)

	entries, err := db.Leaderboard()
	if err != nil {
		t.Fatalf("Leaderboard: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("leaderboard should not be empty")
	}

	totalFor := func(user string) int {
		for _, e := range entries {
			if e.Username == user {
				return e.TotalStars
			}
		}
		return -1
	}

	if totalFor("alice") != 6 {
		t.Errorf("alice total = %d, want 6", totalFor("alice"))
	}
	if totalFor("bob") != 6 {
		t.Errorf("bob total = %d, want 6", totalFor("bob"))
	}
}

func TestAchievements(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.GrantAchievement("alice", "first_pass"); err != nil {
		t.Fatalf("GrantAchievement: %v", err)
	}

	if err := db.GrantAchievement("alice", "first_pass"); err != nil {
		t.Fatalf("duplicate GrantAchievement: %v", err)
	}

	if err := db.GrantAchievement("alice", "debugger"); err != nil {
		t.Fatalf("GrantAchievement: %v", err)
	}

	achievements, err := db.GetAchievements("alice")
	if err != nil {
		t.Fatalf("GetAchievements: %v", err)
	}

	if len(achievements) != 2 {
		t.Fatalf("achievements = %v (len=%d), want 2", achievements, len(achievements))
	}

	has := func(id string) bool {
		for _, a := range achievements {
			if a == id {
				return true
			}
		}
		return false
	}

	if !has("first_pass") || !has("debugger") {
		t.Errorf("achievements = %v, want [first_pass debugger]", achievements)
	}

	bobAch, _ := db.GetAchievements("bob")
	if len(bobAch) != 0 {
		t.Errorf("bob should have no achievements, got %v", bobAch)
	}
}

func TestUserSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	v, err := db.GetUserSetting("alice", "keymap")
	if err != nil {
		t.Fatalf("GetUserSetting unset: %v", err)
	}
	if v != "" {
		t.Errorf("unset key should return empty, got %q", v)
	}

	db.SetUserSetting("alice", "keymap", "vim")
	v, err = db.GetUserSetting("alice", "keymap")
	if err != nil {
		t.Fatalf("GetUserSetting after set: %v", err)
	}
	if v != "vim" {
		t.Errorf("setting after set = %q, want vim", v)
	}

	db.SetUserSetting("alice", "keymap", "default")
	v, err = db.GetUserSetting("alice", "keymap")
	if err != nil {
		t.Fatalf("GetUserSetting after update: %v", err)
	}
	if v == "" {
		t.Errorf("setting after update should not be empty")
	}
}

func TestCountLessonsWithMinStars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.MarkCompleted("alice", "l1", 3)
	db.MarkCompleted("alice", "l2", 3)
	db.MarkCompleted("alice", "l3", 2)
	db.MarkCompleted("alice", "l4", 1)

	count, err := db.CountLessonsWithMinStars("alice", 3)
	if err != nil {
		t.Fatalf("CountLessonsWithMinStars: %v", err)
	}
	if count != 2 {
		t.Errorf("count(>=3) = %d, want 2", count)
	}

	count, _ = db.CountLessonsWithMinStars("alice", 2)
	if count != 3 {
		t.Errorf("count(>=2) = %d, want 3", count)
	}
}

func TestUpdateStreak(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	streak := db.UpdateStreak("alice")
	if streak != 1 {
		t.Errorf("first day streak = %d, want 1", streak)
	}

	streak2 := db.UpdateStreak("alice")
	if streak2 != 1 {
		t.Errorf("same day streak = %d, want 1", streak2)
	}
}

func TestCreateAndAuthUser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.CreateUser("alice", "secret123", RoleStudent); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	u, err := db.Authenticate("alice", "secret123")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if u.Role != RoleStudent {
		t.Errorf("role = %q, want student", u.Role)
	}

	_, err = db.Authenticate("alice", "wrong")
	if err == nil {
		t.Error("expected auth failure with wrong password")
	}

	_, err = db.Authenticate("bob", "anything")
	if err == nil {
		t.Error("expected auth failure for unknown user")
	}
}

func TestUserRoles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.CreateUser("admin", "admin123", RoleAdmin)
	db.CreateUser("student", "student123", RoleStudent)

	admin, _ := db.Authenticate("admin", "admin123")
	if admin.Role != RoleAdmin {
		t.Errorf("admin role = %q", admin.Role)
	}

	stu, _ := db.Authenticate("student", "student123")
	if stu.Role != RoleStudent {
		t.Errorf("student role = %q", stu.Role)
	}
}

func TestUserEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.CreateUser("alice", "pass", RoleStudent)

	db.SetUserEnabled("alice", false)
	_, err = db.Authenticate("alice", "pass")
	if err == nil {
		t.Error("expected auth failure for disabled user")
	}

	db.SetUserEnabled("alice", true)
	_, err = db.Authenticate("alice", "pass")
	if err != nil {
		t.Error("expected auth success after re-enabling")
	}
}

func TestSetUserPassword(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	db.CreateUser("alice", "oldpass", RoleStudent)

	db.SetUserPassword("alice", "newpass")

	_, err = db.Authenticate("alice", "oldpass")
	if err == nil {
		t.Error("old password should no longer work")
	}

	_, err = db.Authenticate("alice", "newpass")
	if err != nil {
		t.Error("new password should work")
	}
}

func TestCountAndListUsers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	count, _ := db.CountUsers()
	if count != 0 {
		t.Errorf("initial count = %d, want 0", count)
	}

	db.CreateUser("s1", "p1", RoleStudent)
	db.CreateUser("s2", "p2", RoleStudent)
	db.CreateUser("admin", "p3", RoleAdmin)

	count, _ = db.CountUsers()
	if count != 2 {
		t.Errorf("student count = %d, want 2", count)
	}

	hasUsers, _ := db.HasUsers()
	if !hasUsers {
		t.Error("HasUsers should be true")
	}

	users, _ := db.ListUsers()
	if len(users) != 3 {
		t.Fatalf("ListUsers length = %d, want 3", len(users))
	}
}

func TestGenerateSetupToken(t *testing.T) {
	t1 := GenerateSetupToken()
	t2 := GenerateSetupToken()
	if t1 == t2 {
		t.Error("tokens should be unique")
	}
	if len(t1) != 22 {
		t.Errorf("token length = %d, want 22", len(t1))
	}
}

func TestSecureCompare(t *testing.T) {
	if !SecureCompare("abc", "abc") {
		t.Error("identical strings should match")
	}
	if SecureCompare("abc", "abd") {
		t.Error("different strings should not match")
	}
	if SecureCompare("abc", "ab") {
		t.Error("different length should not match")
	}
	if SecureCompare("abc", "ABC") {
		t.Error("case difference should not match")
	}
}
