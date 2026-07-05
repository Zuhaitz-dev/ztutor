package db

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	"ztutor/internal/license"
	"ztutor/internal/logutil"
)

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("create db file: %w", err)
		}
		f.Close()
	}

	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxIdleConns(5)
	conn.SetMaxOpenConns(25)

	if err := migrate(conn); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

// migrations is an ordered list of schema changes. Each entry runs exactly
// once, identified by its index (= version number, 1-based).
// APPEND ONLY — never edit or reorder existing entries.
var migrations = []string{
	// v1: base schema
	`CREATE TABLE IF NOT EXISTS progress (
		username   TEXT NOT NULL,
		lesson_id  TEXT NOT NULL,
		completed  INTEGER DEFAULT 0,
		started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (username, lesson_id)
	);
	CREATE TABLE IF NOT EXISTS settings (
		username TEXT NOT NULL,
		key      TEXT NOT NULL,
		value    TEXT NOT NULL,
		PRIMARY KEY (username, key)
	);
	CREATE TABLE IF NOT EXISTS achievements (
		username    TEXT NOT NULL,
		achievement TEXT NOT NULL,
		earned_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (username, achievement)
	);
	CREATE TABLE IF NOT EXISTS users (
		username   TEXT NOT NULL PRIMARY KEY,
		password   TEXT NOT NULL DEFAULT '',
		role       TEXT NOT NULL DEFAULT 'student',
		enabled    INTEGER NOT NULL DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS enrollments (
		username    TEXT NOT NULL,
		course_id   TEXT NOT NULL,
		enrolled_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (username, course_id)
	);
	CREATE TABLE IF NOT EXISTS challenge_progress (
		username     TEXT NOT NULL,
		challenge_id TEXT NOT NULL,
		course_id    TEXT NOT NULL,
		completed    INTEGER DEFAULT 0,
		attempts     INTEGER DEFAULT 0,
		best_output  TEXT,
		completed_at TIMESTAMP,
		PRIMARY KEY (username, challenge_id)
	)`,

	// v2: add stars + completed_at to progress (idempotent via IF NOT EXISTS)
	`ALTER TABLE progress ADD COLUMN stars INTEGER DEFAULT 0`,
	`ALTER TABLE progress ADD COLUMN completed_at TIMESTAMP`,
	`UPDATE progress SET stars = 1 WHERE completed = 1 AND stars = 0`,

	// v3: indexes for leaderboard and per-user queries
	`CREATE INDEX IF NOT EXISTS idx_progress_username  ON progress (username)`,
	`CREATE INDEX IF NOT EXISTS idx_progress_completed ON progress (completed, username)`,
	`CREATE INDEX IF NOT EXISTS idx_enrollments_user   ON enrollments (username)`,

	// v6: redeemed personal licenses
	`CREATE TABLE IF NOT EXISTS license_redemptions (
		license_id   TEXT NOT NULL PRIMARY KEY,
		username     TEXT NOT NULL,
		license_blob TEXT NOT NULL,
		redeemed_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,
}

// bootstrapVersion is the highest migration already applied by old deployments.
const bootstrapVersion = 4

func migrate(conn *sql.DB) error {
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var current int
	conn.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current)

	// First open with the new versioned system on an existing database:
	// the old code already applied v1–v4 (CREATE TABLE + ALTER TABLE + UPDATE).
	// Record them as done so we don't try to re-run ALTER TABLE and fail.
	if current == 0 && tableHasColumn(conn, "progress", "stars") {
		for v := 1; v <= bootstrapVersion; v++ {
			conn.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (?)`, v)
		}
		conn.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current)
		logutil.Debug("db: bootstrapped existing schema to v%d", current)
	}

	for i, stmt := range migrations {
		ver := i + 1
		if ver <= current {
			continue
		}
		if _, err := conn.Exec(stmt); err != nil {
			return fmt.Errorf("migration v%d: %w", ver, err)
		}
		if _, err := conn.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (?)`, ver); err != nil {
			return fmt.Errorf("record migration v%d: %w", ver, err)
		}
		logutil.Debug("db: migration v%d applied", ver)
	}
	return nil
}

// tableHasColumn returns true if the named column exists in the SQLite table.
func tableHasColumn(conn *sql.DB, table, column string) bool {
	rows, err := conn.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dflt sql.NullString
		var pk int
		if rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk) == nil && name == column {
			return true
		}
	}
	return false
}

type UserRole string

const (
	RoleStudent UserRole = "student"
	RoleAdmin   UserRole = "admin"
)

type User struct {
	Username  string
	Role      UserRole
	Enabled   bool
	CreatedAt time.Time
}

func (db *DB) CreateUser(username, password string, role UserRole) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = db.conn.Exec(
		`INSERT INTO users (username, password, role) VALUES (?, ?, ?)`,
		username, string(hash), string(role),
	)
	return err
}

func (db *DB) Authenticate(username, password string) (*User, error) {
	var hash, role string
	var enabled int
	var createdAt time.Time
	err := db.conn.QueryRow(
		`SELECT password, role, enabled, created_at FROM users WHERE username = ?`,
		username,
	).Scan(&hash, &role, &enabled, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if enabled == 0 {
		return nil, fmt.Errorf("account disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	return &User{
		Username:  username,
		Role:      UserRole(role),
		Enabled:   enabled == 1,
		CreatedAt: createdAt,
	}, nil
}

func (db *DB) GetUser(username string) (*User, error) {
	var role string
	var enabled int
	var createdAt time.Time
	err := db.conn.QueryRow(
		`SELECT role, enabled, created_at FROM users WHERE username = ?`,
		username,
	).Scan(&role, &enabled, &createdAt)
	if err != nil {
		return nil, err
	}
	return &User{
		Username:  username,
		Role:      UserRole(role),
		Enabled:   enabled == 1,
		CreatedAt: createdAt,
	}, nil
}

func (db *DB) SetUserEnabled(username string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := db.conn.Exec(`UPDATE users SET enabled = ? WHERE username = ?`, v, username)
	return err
}

func (db *DB) SetUserPassword(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = db.conn.Exec(`UPDATE users SET password = ? WHERE username = ?`, string(hash), username)
	return err
}

func (db *DB) CountUsers() (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'student'`).Scan(&count)
	return count, err
}

func (db *DB) ListUsers() ([]User, error) {
	rows, err := db.conn.Query(
		`SELECT username, role, enabled, created_at FROM users ORDER BY username`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var role string
		var enabled int
		if err := rows.Scan(&u.Username, &role, &enabled, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Role = UserRole(role)
		u.Enabled = enabled == 1
		users = append(users, u)
	}
	return users, nil
}

func (db *DB) HasUsers() (bool, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count > 0, err
}

func GenerateSetupToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		logutil.Fatal("generate setup token: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func GenerateStudentPassword() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		logutil.Fatal("generate student password: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)[:10]
}

func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (db *DB) GetUserSetting(username, key string) (string, error) {
	var val string
	err := db.conn.QueryRow(
		`SELECT value FROM settings WHERE username = ? AND key = ?`,
		username, key,
	).Scan(&val)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

func (db *DB) SetUserSetting(username, key, value string) error {
	_, err := db.conn.Exec(
		`INSERT INTO settings (username, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(username, key) DO UPDATE SET value = excluded.value`,
		username, key, value,
	)
	return err
}

// LeaderboardEntry is one row in the all-time leaderboard.
type LeaderboardEntry struct {
	Username    string
	TotalStars  int
	LessonsDone int
}

// Leaderboard returns the top 20 users ordered by total stars then lessons done.
func (db *DB) Leaderboard() ([]LeaderboardEntry, error) {
	rows, err := db.conn.Query(`
		SELECT username, SUM(stars) as total_stars, COUNT(*) as lessons_done
		FROM progress
		WHERE completed = 1
		GROUP BY username
		ORDER BY total_stars DESC, lessons_done DESC
		LIMIT 20
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.Username, &e.TotalStars, &e.LessonsDone); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (db *DB) MarkStarted(username, lessonID string) error {
	_, err := db.conn.Exec(
		`INSERT OR IGNORE INTO progress (username, lesson_id) VALUES (?, ?)`,
		username, lessonID,
	)
	return err
}

// MarkCompleted records a lesson as completed with the given star rating (1–3).
// Only updates stars if the new value is higher than the stored one.
// completed_at is updated whenever the student achieves a new personal best.
func (db *DB) MarkCompleted(username, lessonID string, stars int) error {
	_, err := db.conn.Exec(
		`INSERT INTO progress (username, lesson_id, completed, stars, completed_at)
		 VALUES (?, ?, 1, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(username, lesson_id) DO UPDATE SET
		   completed    = 1,
		   stars        = MAX(stars, excluded.stars),
		   completed_at = CASE WHEN excluded.stars >= stars THEN CURRENT_TIMESTAMP
		                       ELSE completed_at END`,
		username, lessonID, stars,
	)
	return err
}

// Progress returns a map of lesson ID → best star rating (0 = not completed).
func (db *DB) Progress(username string) (map[string]int, error) {
	rows, err := db.conn.Query(
		`SELECT lesson_id, stars FROM progress WHERE username = ? AND completed = 1`,
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	progress := make(map[string]int)
	for rows.Next() {
		var lessonID string
		var stars int
		if err := rows.Scan(&lessonID, &stars); err != nil {
			return nil, err
		}
		progress[lessonID] = stars
	}
	return progress, nil
}

// GrantAchievement records an achievement for username. If the achievement was
// already granted it is silently ignored (INSERT OR IGNORE).
func (db *DB) GrantAchievement(username, id string) error {
	_, err := db.conn.Exec(
		`INSERT OR IGNORE INTO achievements (username, achievement) VALUES (?, ?)`,
		username, id,
	)
	return err
}

// GetAchievements returns the list of achievement IDs already earned by username.
func (db *DB) GetAchievements(username string) ([]string, error) {
	rows, err := db.conn.Query(
		`SELECT achievement FROM achievements WHERE username = ?`,
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// UpdateStreak updates the consecutive-day login streak for username and
// returns the current streak count. Call once per session on login.
func (db *DB) UpdateStreak(username string) int {
	today := time.Now().Format("2006-01-02")
	lastLogin, _ := db.GetUserSetting(username, "last_login")

	if lastLogin == today {
		streakStr, _ := db.GetUserSetting(username, "streak")
		streak, _ := strconv.Atoi(streakStr)
		if streak == 0 {
			streak = 1
		}
		return streak
	}

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	streak := 1
	if lastLogin == yesterday {
		prevStr, _ := db.GetUserSetting(username, "streak")
		prev, _ := strconv.Atoi(prevStr)
		streak = prev + 1
	}

	if err := db.SetUserSetting(username, "last_login", today); err != nil {
		logutil.Error("failed to save last_login for %s: %v", username, err)
	}
	if err := db.SetUserSetting(username, "streak", strconv.Itoa(streak)); err != nil {
		logutil.Error("failed to save streak for %s: %v", username, err)
	}
	return streak
}

// CountLessonsWithMinStars returns the number of lessons completed by username
// with at least minStars stars.
func (db *DB) CountLessonsWithMinStars(username string, minStars int) (int, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM progress WHERE username = ? AND completed = 1 AND stars >= ?`,
		username, minStars,
	).Scan(&count)
	return count, err
}

// Enroll records a user enrollment in a course. This is idempotent (INSERT OR IGNORE).
func (db *DB) Enroll(username, courseID string) error {
	_, err := db.conn.Exec(
		`INSERT OR IGNORE INTO enrollments (username, course_id) VALUES (?, ?)`,
		username, courseID,
	)
	return err
}

// DeleteEnrollment removes a user enrollment from a course.
func (db *DB) DeleteEnrollment(username, courseID string) error {
	_, err := db.conn.Exec(
		`DELETE FROM enrollments WHERE username = ? AND course_id = ?`,
		username, courseID,
	)
	return err
}

// IsEnrolled returns true if the user is enrolled in the given course.
func (db *DB) IsEnrolled(username, courseID string) bool {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM enrollments WHERE username = ? AND course_id = ?`,
		username, courseID,
	).Scan(&count)
	return err == nil && count > 0
}

// ChallengeEntry is one row in a challenge leaderboard.
type ChallengeEntry struct {
	Username    string
	Score       int
	CompletedAt time.Time
}

// CountEnrollments returns the number of users enrolled in a course.
func (db *DB) CountEnrollments(courseID string) (int, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM enrollments WHERE course_id = ?`,
		courseID,
	).Scan(&count)
	return count, err
}

// SubmitChallenge increments attempts, marks completed, and stores best output.
func (db *DB) SubmitChallenge(username, challengeID, courseID, output string) error {
	_, err := db.conn.Exec(
		`INSERT INTO challenge_progress (username, challenge_id, course_id, completed, attempts, best_output, completed_at) VALUES (?, ?, ?, 1, 1, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(username, challenge_id) DO UPDATE SET
		   completed = 1,
		   attempts = attempts + 1,
		   best_output = CASE WHEN LENGTH(COALESCE(best_output,'')) < LENGTH(?) THEN best_output ELSE ? END,
		   completed_at = CURRENT_TIMESTAMP`,
		username, challengeID, courseID, output, output, output,
	)
	return err
}

// ChallengeProgress returns a map of challenge_id → attempts count for the user in the given course.
func (db *DB) ChallengeProgress(username, courseID string) (map[string]int, error) {
	rows, err := db.conn.Query(
		`SELECT challenge_id, attempts FROM challenge_progress WHERE username = ? AND course_id = ?`,
		username, courseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	progress := make(map[string]int)
	for rows.Next() {
		var cid string
		var attempts int
		if err := rows.Scan(&cid, &attempts); err != nil {
			return nil, err
		}
		progress[cid] = attempts
	}
	return progress, nil
}

// ChallengeLeaderboard returns ranked entries for a challenge, ordered by fewest attempts then earliest completion time.
func (db *DB) ChallengeLeaderboard(courseID, challengeID string, limit int) ([]ChallengeEntry, error) {
	rows, err := db.conn.Query(
		`SELECT username, attempts, completed_at
		 FROM challenge_progress
		 WHERE course_id = ? AND challenge_id = ? AND completed = 1
		 ORDER BY attempts ASC, completed_at ASC
		 LIMIT ?`,
		courseID, challengeID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ChallengeEntry
	for rows.Next() {
		var e ChallengeEntry
		var completedAt time.Time
		if err := rows.Scan(&e.Username, &e.Score, &completedAt); err != nil {
			return nil, err
		}
		e.CompletedAt = completedAt
		entries = append(entries, e)
	}
	return entries, nil
}

// ListEnrolledUsers returns all usernames enrolled in the given course.
func (db *DB) ListEnrolledUsers(courseID string) ([]string, error) {
	rows, err := db.conn.Query(
		`SELECT username FROM enrollments WHERE course_id = ? ORDER BY enrolled_at`,
		courseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var usernames []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		usernames = append(usernames, u)
	}
	return usernames, nil
}

// ListEnrollments returns the list of course IDs the user is enrolled in.
func (db *DB) ListEnrollments(username string) ([]string, error) {
	rows, err := db.conn.Query(
		`SELECT course_id FROM enrollments WHERE username = ? ORDER BY enrolled_at`,
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// RedeemPersonalLicense records a one-account personal license redemption and
// grants the unlocked courses to that user. Re-redeeming the same license for
// the same username is allowed and keeps course grants idempotent.
func (db *DB) RedeemPersonalLicense(username string, info license.Info, licenseBlob []byte) error {
	if username == "" {
		return fmt.Errorf("username required")
	}
	if !info.IsPersonal() {
		return fmt.Errorf("license is not personal")
	}
	if info.LicenseID == "" {
		return fmt.Errorf("personal license missing license_id")
	}
	if info.Username != "" && info.Username != username {
		return fmt.Errorf("license belongs to %s", info.Username)
	}
	if len(licenseBlob) == 0 {
		return fmt.Errorf("license blob required")
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var redeemedBy string
	err = tx.QueryRow(`SELECT username FROM license_redemptions WHERE license_id = ?`, info.LicenseID).Scan(&redeemedBy)
	switch {
	case err == nil:
		if redeemedBy != username {
			return fmt.Errorf("license already redeemed by another user")
		}
	case errors.Is(err, sql.ErrNoRows):
		if _, err := tx.Exec(
			`INSERT INTO license_redemptions (license_id, username, license_blob) VALUES (?, ?, ?)`,
			info.LicenseID, username, string(licenseBlob),
		); err != nil {
			return err
		}
	case err != nil:
		return err
	}

	for _, courseID := range info.UnlockedCourses {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO enrollments (username, course_id) VALUES (?, ?)`,
			username, courseID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) ListRedeemedLicenseBlobs(username string) ([][]byte, error) {
	rows, err := db.conn.Query(
		`SELECT license_blob FROM license_redemptions WHERE username = ? ORDER BY redeemed_at`,
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blobs [][]byte
	for rows.Next() {
		var blob string
		if err := rows.Scan(&blob); err != nil {
			return nil, err
		}
		blobs = append(blobs, []byte(blob))
	}
	return blobs, nil
}
