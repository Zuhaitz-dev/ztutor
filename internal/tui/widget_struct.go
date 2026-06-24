package tui

import (
	"fmt"
	"regexp"
	"strings"

	"ztutor/internal/i18n"

	te "github.com/charmbracelet/lipgloss"
)

type structField struct {
	typ  string
	name string
}

type structDef struct {
	name   string
	fields []structField
}

type StructInspectorWidget struct {
	structs []structDef
	visible bool
	loc     *i18n.Locale
}

func newStructInspectorWidget(loc *i18n.Locale) *StructInspectorWidget {
	return &StructInspectorWidget{loc: loc}
}

func (w *StructInspectorWidget) Available() bool { return true }
func (w *StructInspectorWidget) IsVisible() bool { return w.visible }
func (w *StructInspectorWidget) Toggle()         { w.visible = !w.visible; w.parseFromCode("") }
func (w *StructInspectorWidget) Hide()           { w.visible = false }

func (w *StructInspectorWidget) SetCode(code string) {
	w.parseFromCode(code)
}

func (w *StructInspectorWidget) parseFromCode(code string) {
	w.structs = nil
	// Regex to find C struct definitions
	re := regexp.MustCompile(`(?s)struct\s+(\w+)\s*\{([^}]+)\}`)
	matches := re.FindAllStringSubmatch(code, -1)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		sd := structDef{name: m[1]}
		body := m[2]
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
				continue
			}
			line = strings.TrimRight(line, ";")
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := parts[len(parts)-1]
				typParts := parts[:len(parts)-1]
				for strings.HasPrefix(name, "*") {
					typParts = append(typParts, "*")
					name = name[1:]
				}
				typ := strings.Join(typParts, " ")
				sd.fields = append(sd.fields, structField{typ: typ, name: name})
			}
		}
		if len(sd.fields) > 0 {
			w.structs = append(w.structs, sd)
		}
	}
}

func (w *StructInspectorWidget) View() string {
	if !w.visible || len(w.structs) == 0 {
		return ""
	}
	hdrStyle := te.NewStyle().Bold(true).Foreground(te.Color("117"))
	structStyle := te.NewStyle().Foreground(te.Color("214")).Bold(true)
	typStyle := te.NewStyle().Foreground(te.Color("39"))
	nameStyle := te.NewStyle().Foreground(te.Color("252"))
	dimStyle := te.NewStyle().Foreground(te.Color("240"))

	var b strings.Builder
	b.WriteString(hdrStyle.Render(" " + w.loc.T("exercise.struct.header") + " "))
	b.WriteString("\n")

	for _, sd := range w.structs {
		b.WriteString("\n" + structStyle.Render("struct "+sd.name+" {"))
		b.WriteString("\n")
		for _, f := range sd.fields {
			b.WriteString(fmt.Sprintf("  %s %s\n",
				typStyle.Render(f.typ),
				nameStyle.Render(f.name)))
		}
		b.WriteString(dimStyle.Render("}"))
		b.WriteString("\n")
	}
	return b.String()
}
