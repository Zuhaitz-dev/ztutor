package db

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// StudentCredential holds the plaintext password for a newly created account.
// Only returned at import time — passwords are stored hashed in the DB.
type StudentCredential struct {
	Username string
	Password string
}

// ImportResult summarises a bulk student import.
type ImportResult struct {
	Created []StudentCredential
	Skipped []string // usernames that already existed
	Errors  []string // per-row errors
}

// ImportStudentsCSV bulk-creates student accounts from a CSV file.
// Format: one row per student — username[,password]
//   - If password is omitted, one is generated automatically.
//   - Existing usernames are silently skipped.
//   - maxStudents == 0 means no limit; non-zero caps total students in the DB.
//
// Returns ImportResult even when err != nil (partial results may be available).
func (db *DB) ImportStudentsCSV(data []byte, maxStudents int) (ImportResult, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.Comment = '#'
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return ImportResult{}, fmt.Errorf("parse CSV: %w", err)
	}

	var result ImportResult

	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		username := strings.TrimSpace(record[0])
		if username == "" {
			continue
		}

		// Skip existing accounts.
		var count int
		db.conn.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&count)
		if count > 0 {
			result.Skipped = append(result.Skipped, username)
			continue
		}

		// Enforce seat limit.
		if maxStudents > 0 {
			current, err := db.CountUsers()
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: cannot count users: %v", username, err))
				break
			}
			if current >= maxStudents {
				result.Errors = append(result.Errors,
					fmt.Sprintf("seat limit reached (%d), %s and remaining rows skipped", maxStudents, username))
				break
			}
		}

		password := ""
		if len(record) > 1 {
			password = strings.TrimSpace(record[1])
		}
		if password == "" {
			password = generatePassword()
		}

		if err := db.CreateUser(username, password, RoleStudent); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", username, err))
			continue
		}
		result.Created = append(result.Created, StudentCredential{
			Username: username,
			Password: password,
		})
	}

	return result, nil
}

// generatePassword returns a random 10-character alphanumeric password.
func generatePassword() string {
	const chars = "abcdefghjkmnpqrstuvwxyz23456789" // exclude ambiguous: 0/O/1/I/l
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 10)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return string(b)
}

// ExportProgressCSV returns a narrow-format CSV of all completed lesson
// progress across all students.
// Columns: username, lesson_id, stars, completed_at
// Professors can pivot this in Excel/Sheets into any gradebook view they need.
func (db *DB) ExportProgressCSV() ([]byte, error) {
	rows, err := db.conn.Query(`
		SELECT p.username, p.lesson_id, p.stars,
		       COALESCE(p.completed_at, p.started_at, '') as activity
		FROM progress p
		JOIN users u ON u.username = p.username AND u.role = 'student'
		WHERE p.completed = 1
		ORDER BY p.username, p.lesson_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"username", "lesson_id", "stars", "completed_at"})

	for rows.Next() {
		var username, lessonID, activity string
		var stars int
		if err := rows.Scan(&username, &lessonID, &stars, &activity); err != nil {
			return nil, err
		}
		_ = w.Write([]string{username, lessonID, strconv.Itoa(stars), activity})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), rows.Err()
}

// ExportRosterCSV returns a per-student summary CSV.
// Columns: username, enabled, total_stars, lessons_done, streak, last_active, joined
func (db *DB) ExportRosterCSV() ([]byte, error) {
	rows, err := db.conn.Query(`
		SELECT
			u.username,
			u.enabled,
			COALESCE(SUM(CASE WHEN p.completed = 1 THEN p.stars ELSE 0 END), 0) AS total_stars,
			COALESCE(COUNT(CASE WHEN p.completed = 1 THEN 1 END), 0) AS lessons_done,
			u.created_at
		FROM users u
		LEFT JOIN progress p ON p.username = u.username
		WHERE u.role = 'student'
		GROUP BY u.username, u.enabled, u.created_at
		ORDER BY u.username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"username", "enabled", "total_stars", "lessons_done", "streak", "last_active", "joined"})

	for rows.Next() {
		var username, createdAt string
		var enabled, totalStars, lessonsDone int
		if err := rows.Scan(&username, &enabled, &totalStars, &lessonsDone, &createdAt); err != nil {
			return nil, err
		}
		enabledStr := "yes"
		if enabled == 0 {
			enabledStr = "no"
		}
		streak, _ := db.GetUserSetting(username, "streak")
		if streak == "" {
			streak = "0"
		}
		lastActive, _ := db.GetUserSetting(username, "last_login")
		if lastActive == "" {
			lastActive = "—"
		}
		_ = w.Write([]string{
			username,
			enabledStr,
			strconv.Itoa(totalStars),
			strconv.Itoa(lessonsDone),
			streak,
			lastActive,
			createdAt,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), rows.Err()
}
