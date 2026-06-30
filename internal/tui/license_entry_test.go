package tui

import (
	"strings"
	"testing"

	"ztutor/internal/i18n"
	"ztutor/internal/license"
)

func TestLicenseEntryScreen_ShowsInvalidMessage(t *testing.T) {
	screen := NewLicenseEntryScreen(i18n.New("en"), 80, 24, nil)
	model, _ := screen.Update(licenseEntryDoneMsg{err: license.ErrExpired})
	updated := model.(*licenseEntryScreen)
	view := stripANSI(updated.View())
	if !strings.Contains(view, "Invalid license") || !strings.Contains(view, "license expired") {
		t.Fatalf("view missing invalid license feedback:\n%s", view)
	}
}
