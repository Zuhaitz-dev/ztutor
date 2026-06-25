package tui

import (
	"testing"

	"ztutor/internal/db"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAppLanguageChangeStaysOnLicenseEntry(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}

	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.Update(NavigateToLicenseEntry{})
	if _, ok := app.current.(*licenseEntryScreen); !ok {
		t.Fatalf("current = %T, want *licenseEntryScreen", app.current)
	}

	if _, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlL}); cmd != nil {
		t.Fatal("language change on license entry should stay in place without async command")
	}
	screen, ok := app.current.(*licenseEntryScreen)
	if !ok {
		t.Fatalf("current = %T, want *licenseEntryScreen", app.current)
	}
	if screen.loc.Lang() != "es" {
		t.Fatalf("license entry lang = %q, want es", screen.loc.Lang())
	}
}

func TestAppSettingsSaveStaysOnSettings(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}

	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.Update(NavigateToSettings{})
	if _, ok := app.current.(*SettingsScreen); !ok {
		t.Fatalf("current = %T, want *SettingsScreen", app.current)
	}

	app.Update(settingsSavedMsg{key: "keymap", value: "vim"})
	if _, ok := app.current.(*SettingsScreen); !ok {
		t.Fatalf("current = %T, want *SettingsScreen after save", app.current)
	}
	if app.keymap != "vim" {
		t.Fatalf("app keymap = %q, want vim", app.keymap)
	}
	if got, _ := database.GetUserSetting("alice", "keymap"); got != "vim" {
		t.Fatalf("persisted keymap = %q, want vim", got)
	}
}
