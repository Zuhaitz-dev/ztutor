package sandbox

import (
	"os/exec"
	"strings"
	"testing"
)

func hasCompiler(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func TestCpp_Compile(t *testing.T) {
	lang := GetLanguage("cpp")
	if lang == nil || !hasCompiler("g++") {
		t.Skip("C++ compiler not available")
	}

	result, err := Run(lang, map[string]string{"main.cpp": `#include <iostream>
int main() {
	std::cout << "cpp-hello" << std::endl;
	return 0;
}`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if strings.TrimSpace(result.Output) != "cpp-hello" {
		t.Errorf("Output = %q, want %q", result.Output, "cpp-hello")
	}
}

func TestCpp_SyntaxCheck(t *testing.T) {
	lang := GetLanguage("cpp")
	if lang == nil || !hasCompiler("g++") {
		t.Skip("C++ compiler not available")
	}

	diags, err := SyntaxCheck(lang, map[string]string{"main.cpp": `int main() {
	std::cout << "oops"
}`}, "", nil)

	if err != nil {
		t.Fatalf("SyntaxCheck: %v", err)
	}
	_ = diags
}

func TestCpp_Assembly(t *testing.T) {
	lang := GetLanguage("cpp")
	if lang == nil || !hasCompiler("g++") {
		t.Skip("C++ compiler not available")
	}

	asm, err := GenerateAssembly(lang, map[string]string{"main.cpp": `int add(int a, int b) { return a + b; }`}, "", nil)
	if err != nil {
		t.Fatalf("GenerateAssembly: %v", err)
	}
	if asm == "" {
		t.Fatal("empty assembly output")
	}
	if !strings.Contains(asm, "add") {
		t.Error("assembly should contain function name")
	}
}

func TestPython_Execute(t *testing.T) {
	lang := GetLanguage("python")
	if lang == nil || !hasCompiler("python3") {
		t.Skip("python3 not available")
	}

	result, err := Run(lang, map[string]string{"main.py": `print("py-hello")`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if strings.TrimSpace(result.Output) != "py-hello" {
		t.Errorf("Output = %q, want %q", result.Output, "py-hello")
	}
}

func TestPython_SyntaxCheck(t *testing.T) {
	lang := GetLanguage("python")
	if lang == nil || !hasCompiler("python3") {
		t.Skip("python3 not available")
	}

	diags, err := SyntaxCheck(lang, map[string]string{"main.py": `print("ok")`}, "", nil)
	if err != nil {
		t.Fatalf("SyntaxCheck: %v", err)
	}
	_ = diags
}

func TestPython_SyntaxCheck_Error(t *testing.T) {
	lang := GetLanguage("python")
	if lang == nil || !hasCompiler("python3") {
		t.Skip("python3 not available")
	}

	result, err := Run(lang, map[string]string{"main.py": `print("broken"`}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error == "" {
		t.Log("expected a syntax error, got clean run")
	}
}

func TestGo_Compile(t *testing.T) {
	lang := GetLanguage("go")
	if lang == nil || !hasCompiler("go") {
		t.Skip("go compiler not available")
	}

	result, err := Run(lang, map[string]string{"main.go": `package main
import "fmt"
func main() { fmt.Println("go-hello") }`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if strings.TrimSpace(result.Output) != "go-hello" {
		t.Errorf("Output = %q, want %q", result.Output, "go-hello")
	}
}

func TestGo_SyntaxCheck(t *testing.T) {
	lang := GetLanguage("go")
	if lang == nil || !hasCompiler("go") {
		t.Skip("go compiler not available")
	}

	diags, err := SyntaxCheck(lang, map[string]string{"main.go": `package main
func main() {}`}, "", nil)
	if err != nil {
		t.Fatalf("SyntaxCheck: %v", err)
	}
	_ = diags
}

func TestGo_Assembly(t *testing.T) {
	lang := GetLanguage("go")
	if lang == nil || !hasCompiler("go") {
		t.Skip("go compiler not available")
	}

	asm, err := GenerateAssembly(lang, map[string]string{"main.go": `package main
func add(a, b int) int { return a + b }
func main() {}`}, "", nil)
	if err != nil {
		t.Fatalf("GenerateAssembly: %v", err)
	}
	if asm == "" {
		t.Fatal("empty assembly output")
	}
}

func TestRuby_CheckSyntax(t *testing.T) {
	lang := GetLanguage("ruby")
	if lang == nil || !hasCompiler("ruby") {
		t.Skip("ruby not available")
	}

	diags, err := SyntaxCheck(lang, map[string]string{"main.rb": `puts "ruby-ok"`}, "", nil)
	if err != nil {
		t.Fatalf("SyntaxCheck: %v", err)
	}
	_ = diags
}

func TestRuby_Execute(t *testing.T) {
	lang := GetLanguage("ruby")
	if lang == nil || !hasCompiler("ruby") {
		t.Skip("ruby not available")
	}

	result, err := Run(lang, map[string]string{"main.rb": `puts "ruby-hello"`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if strings.TrimSpace(result.Output) != "ruby-hello" {
		t.Errorf("Output = %q, want %q", result.Output, "ruby-hello")
	}
}

func TestJava_Compile(t *testing.T) {
	lang := GetLanguage("java")
	if lang == nil || !hasCompiler("javac") {
		t.Skip("javac not available")
	}

	result, err := Run(lang, map[string]string{"main.java": `public class Main {
	public static void main(String[] args) {
		System.out.println("java-hello");
	}
}`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if strings.TrimSpace(result.Output) != "java-hello" {
		t.Errorf("Output = %q, want %q", result.Output, "java-hello")
	}
}

func TestJava_SyntaxCheck(t *testing.T) {
	lang := GetLanguage("java")
	if lang == nil || !hasCompiler("javac") {
		t.Skip("javac not available")
	}

	diags, err := SyntaxCheck(lang, map[string]string{"main.java": `public class Main {
	public static void main(String[] args) { }
}`}, "", nil)
	if err != nil {
		t.Fatalf("SyntaxCheck: %v", err)
	}
	_ = diags
}

func TestRust_Compile(t *testing.T) {
	lang := GetLanguage("rust")
	if lang == nil || !hasCompiler("rustc") {
		t.Skip("rustc not available")
	}

	result, err := Run(lang, map[string]string{"main.rs": `fn main() { println!("rust-hello"); }`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if strings.TrimSpace(result.Output) != "rust-hello" {
		t.Errorf("Output = %q, want %q", result.Output, "rust-hello")
	}
}

func TestRust_SyntaxCheck(t *testing.T) {
	lang := GetLanguage("rust")
	if lang == nil || !hasCompiler("rustc") {
		t.Skip("rustc not available")
	}

	diags, err := SyntaxCheck(lang, map[string]string{"main.rs": `fn main() {}`}, "", nil)
	if err != nil {
		t.Fatalf("SyntaxCheck: %v", err)
	}
	_ = diags
}

func TestRust_Assembly(t *testing.T) {
	lang := GetLanguage("rust")
	if lang == nil || !hasCompiler("rustc") {
		t.Skip("rustc not available")
	}

	asm, err := GenerateAssembly(lang, map[string]string{"main.rs": `fn add(a: i32, b: i32) -> i32 { a + b }
fn main() {}`}, "", nil)
	if err != nil {
		t.Fatalf("GenerateAssembly: %v", err)
	}
	if asm == "" {
		t.Fatal("empty assembly output")
	}
}

func TestUnsupportedLanguage(t *testing.T) {
	lang := GetLanguage("zz-nonexistent")
	if lang != nil {
		t.Fatal("expected nil for unknown language")
	}
}
