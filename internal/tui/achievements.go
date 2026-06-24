package tui

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Achievement describes a single in-game achievement.
type Achievement struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Icon   string `yaml:"icon"`
	Desc   string `yaml:"desc"`
	Secret bool   `yaml:"secret,omitempty"`
}

var allAchievements = []Achievement{
	{ID: "first_compile", Name: "First Blood", Icon: "[C]", Desc: "Compiled your first program"},
	{ID: "first_pass", Name: "Hello, World", Icon: "[P]", Desc: "Passed your first exercise"},
	{ID: "one_shot", Name: "One Shot", Icon: "[1]", Desc: "Passed on the very first attempt"},
	{ID: "comeback", Name: "Comeback Kid", Icon: "[+]", Desc: "Passed after 5+ failed attempts"},
	{ID: "perfect", Name: "Perfectionist", Icon: "[★]", Desc: "Earned ★★★ on a lesson"},
	{ID: "five_perfect", Name: "Gold Standard", Icon: "[★5]", Desc: "Earned ★★★ on 5 different lessons"},
	{ID: "clean_coder", Name: "Clean Coder", Icon: "[-]", Desc: "Passed with zero compiler warnings"},
	{ID: "debugger", Name: "Debugger", Icon: "[D]", Desc: "Launched GDB"},
	{ID: "disassembler", Name: "Disassembler", Icon: "[A]", Desc: "Viewed assembly output"},
	{ID: "interactive", Name: "Interactive", Icon: "[I]", Desc: "Ran a program interactively"},
	{ID: "sanitized", Name: "Sanitized", Icon: "[S]", Desc: "Ran the ASAN memory checker"},
	{ID: "graduate", Name: "Graduate", Icon: "[G]", Desc: "Completed all lessons in a course"},
	// Secret achievements — hidden until discovered.
	{ID: "segfault_king", Name: "Segfault King", Icon: "[X]", Desc: "Earned the classic C crash badge", Secret: true},
	{ID: "into_the_loop", Name: "Into the Loop", Icon: "[~]", Desc: "Hit the execution time limit", Secret: true},
	{ID: "beer", Name: "Correct Header", Icon: "[B]", Desc: "Found the most important C header", Secret: true},
	{ID: "konami", Name: "Up Up Down Down", Icon: "[*]", Desc: "You know the code. Mochi knew you would.", Secret: true},
}

var customAchievements []Achievement

// LoadCustomAchievements reads a YAML file and appends non-conflicting
// achievements to the runtime list. Safe to call multiple times.
func LoadCustomAchievements(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var custom []Achievement
	if err := yaml.Unmarshal(data, &custom); err != nil {
		return
	}
	known := make(map[string]bool)
	for _, a := range allAchievements {
		known[a.ID] = true
	}
	for _, a := range customAchievements {
		known[a.ID] = true
	}
	for _, a := range custom {
		if a.ID != "" && !known[a.ID] {
			customAchievements = append(customAchievements, a)
			known[a.ID] = true
		}
	}
}

// SaveCustomAchievements writes the current custom achievement list to disk
// and appends a new entry, returning the updated list.
func appendAndSaveAchievement(a Achievement, path string) error {
	customAchievements = append(customAchievements, a)
	data, err := yaml.Marshal(customAchievements)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	return os.WriteFile(path, data, 0644)
}

func deleteAndSaveAchievement(id, path string) error {
	var kept []Achievement
	for _, a := range customAchievements {
		if a.ID != id {
			kept = append(kept, a)
		}
	}
	customAchievements = kept
	if len(kept) == 0 {
		return os.Remove(path)
	}
	data, err := yaml.Marshal(kept)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// AllAchievements returns built-in and custom achievements combined.
func AllAchievements() []Achievement {
	out := make([]Achievement, 0, len(allAchievements)+len(customAchievements))
	out = append(out, allAchievements...)
	out = append(out, customAchievements...)
	return out
}

func achievementByID(id string) *Achievement {
	for i := range allAchievements {
		if allAchievements[i].ID == id {
			return &allAchievements[i]
		}
	}
	for i := range customAchievements {
		if customAchievements[i].ID == id {
			return &customAchievements[i]
		}
	}
	return nil
}
