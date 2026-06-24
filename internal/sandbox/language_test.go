package sandbox

import (
	"testing"
)

func TestGetLanguage_Known(t *testing.T) {
	lang := GetLanguage("c")
	if lang == nil {
		t.Fatal("GetLanguage(c) returned nil")
	}
	if lang.Name() != "c" {
		t.Errorf("Name = %q, want c", lang.Name())
	}
	if !lang.IsCompiled() {
		t.Error("C should be compiled")
	}
	if !lang.HasAssembly() {
		t.Error("C should support assembly")
	}
	if !lang.HasSanitizers() {
		t.Error("C should support sanitizers")
	}
	if !lang.HasDebugger() {
		t.Error("C should support debugger")
	}
}

func TestGetLanguage_Unknown(t *testing.T) {
	lang := GetLanguage("zz")
	if lang != nil {
		t.Errorf("GetLanguage(zz) should be nil, got %v", lang)
	}
}

func TestGetLanguage_Python(t *testing.T) {
	lang := GetLanguage("python")
	if lang == nil {
		t.Fatal("GetLanguage(python) returned nil")
	}
	if lang.IsCompiled() {
		t.Error("Python should not be compiled")
	}
	if lang.HasAssembly() {
		t.Error("Python should not support assembly")
	}
	if lang.HasSanitizers() {
		t.Error("Python should not support sanitizers")
	}
}

func TestGetLanguage_Go(t *testing.T) {
	lang := GetLanguage("go")
	if lang == nil {
		t.Fatal("GetLanguage(go) returned nil")
	}
	if !lang.IsCompiled() {
		t.Error("Go should be compiled")
	}
	if !lang.HasAssembly() {
		t.Error("Go should support assembly")
	}
}

func TestGetLanguage_Ruby(t *testing.T) {
	lang := GetLanguage("ruby")
	if lang == nil {
		t.Fatal("GetLanguage(ruby) returned nil")
	}
	if lang.IsCompiled() {
		t.Error("Ruby should not be compiled")
	}
}

func TestGetLanguage_Java(t *testing.T) {
	lang := GetLanguage("java")
	if lang == nil {
		t.Fatal("GetLanguage(java) returned nil")
	}
	if !lang.IsCompiled() {
		t.Error("Java should be compiled")
	}
}

func TestHealthCheck(t *testing.T) {
	warnings := HealthCheck()
	// HealthCheck should not panic; warnings may be empty or contain toolchain info.
	_ = warnings
}

func TestSourceFileName(t *testing.T) {
	c := GetLanguage("c")
	if c == nil {
		t.Skip("C not available")
	}
	if c.SourceFileName() != "main.c" {
		t.Errorf("SourceFileName = %q, want main.c", c.SourceFileName())
	}

	py := GetLanguage("python")
	if py == nil {
		t.Skip("Python not available")
	}
	if py.SourceFileName() != "main.py" {
		t.Errorf("SourceFileName = %q, want main.py", py.SourceFileName())
	}

	rb := GetLanguage("ruby")
	if rb == nil {
		t.Skip("Ruby not available")
	}
	if rb.SourceFileName() != "main.rb" {
		t.Errorf("SourceFileName = %q, want main.rb", rb.SourceFileName())
	}
}

func TestSourceExtension(t *testing.T) {
	for _, name := range []string{"c", "python", "go", "ruby", "java", "cpp"} {
		lang := GetLanguage(name)
		if lang == nil {
			continue
		}
		ext := lang.SourceExtension()
		if ext == "" {
			t.Errorf("%s: SourceExtension is empty", name)
		}
		if ext[0] != '.' {
			t.Errorf("%s: SourceExtension %q should start with '.'", name, ext)
		}
	}
}

func TestSyntaxHighlighting(t *testing.T) {
	// Syntax highlighting is configured per-course (not per-language), so
	// LanguageInfo doesn't expose a SyntaxHighlighting method. The course
	// loader sets the Chroma lexer name based on the language ID.
	for _, name := range []string{"c", "cpp", "python", "go", "ruby", "java", "rust"} {
		lang := GetLanguage(name)
		if lang == nil {
			continue
		}
		// Language must at least report a name for highlight selection.
		if lang.Name() == "" {
			t.Errorf("%s: Name is empty", name)
		}
	}
}

func TestInteractiveCommand(t *testing.T) {
	// C should have an interactive command (binary path itself).
	c := GetLanguage("c")
	if c == nil {
		t.Skip("C not available")
	}
	cmd, args := c.InteractiveCommand("./prog")
	if cmd != "./prog" {
		t.Errorf("C InteractiveCommand = %q, want ./prog", cmd)
	}
	if len(args) != 0 {
		t.Errorf("C InteractiveCommand args should be empty, got %v", args)
	}

	// Python should use python3 as the command.
	py := GetLanguage("python")
	if py == nil {
		t.Skip("Python not available")
	}
	cmd2, _ := py.InteractiveCommand("./main.py")
	if cmd2 != "python3" {
		t.Errorf("Python InteractiveCommand = %q, want python3", cmd2)
	}
}
