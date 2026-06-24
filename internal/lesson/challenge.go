package lesson

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Challenge struct {
	ID         string
	Title      string
	Content    string
	Exercise   string
	Difficulty string
	Tags       []string
	Hints      []string
	Tests      []TestCase
	StartsAt   time.Time
	EndsAt     time.Time
	Points     int
}

type challengeFM struct {
	Difficulty string   `yaml:"difficulty"`
	Tags       []string `yaml:"tags"`
	StartsAt   string   `yaml:"starts_at"`
	EndsAt     string   `yaml:"ends_at"`
	Points     int      `yaml:"points"`
}

func LoadChallenges(dir string) ([]Challenge, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var challenges []Challenge
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ch, err := LoadChallenge(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		challenges = append(challenges, *ch)
	}
	return challenges, nil
}

func LoadChallenge(dir string) (*Challenge, error) {
	ch := &Challenge{ID: filepath.Base(dir)}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		path := filepath.Join(dir, name)

		switch {
		case name == "challenge.md":
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			raw := string(data)
			fm, body := parseFrontmatter(raw)
			ch.Difficulty = fm.Difficulty
			ch.Tags = fm.Tags
			ch.Content = body
			ch.Title = extractTitle(ch.Content)

			// Parse challenge-specific YAML fields.
			if strings.HasPrefix(raw, "---\n") {
				idx := strings.Index(raw[4:], "\n---\n")
				if idx >= 0 {
					fmBlock := raw[4 : idx+4]
					var cfm challengeFM
					if err := yaml.Unmarshal([]byte(fmBlock), &cfm); err == nil {
						if cfm.StartsAt != "" {
							ch.StartsAt, _ = time.Parse(time.RFC3339, cfm.StartsAt)
						}
						if cfm.EndsAt != "" {
							ch.EndsAt, _ = time.Parse(time.RFC3339, cfm.EndsAt)
						}
						if cfm.Points > 0 {
							ch.Points = cfm.Points
						}
					}
				}
			}

		case name == "exercise.c", strings.HasPrefix(name, "exercise."):
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			ch.Exercise = string(data)

		case name == "hints.txt":
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			ch.Hints = splitBlocks(string(data))

		case name == "expected.txt":
			data, _ := os.ReadFile(path)
			ch.Tests = append(ch.Tests, TestCase{Expected: string(data)})
		}
	}

	if ch.Title == "" {
		ch.Title = ch.ID
	}

	return ch, nil
}
