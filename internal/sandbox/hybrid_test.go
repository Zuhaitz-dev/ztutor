package sandbox

import "testing"

func TestHybridExecutor_RoutesRunToRemote(t *testing.T) {
	localCalled := false
	remoteCalled := false
	h := &HybridExecutor{
		Local: &MockExecutor{
			RunFn: func(_ Language, _ map[string]string, _, _ string, _, _ []string) (*Result, error) {
				localCalled = true
				return &Result{Output: "local"}, nil
			},
		},
		Remote: &MockExecutor{
			RunFn: func(_ Language, _ map[string]string, _, _ string, _, _ []string) (*Result, error) {
				remoteCalled = true
				return &Result{Output: "remote"}, nil
			},
		},
	}

	result, err := h.Run(nil, nil, "", "", nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Output != "remote" {
		t.Errorf("Output = %q, want remote", result.Output)
	}
	if localCalled {
		t.Error("Run should NOT call local")
	}
	if !remoteCalled {
		t.Error("Run should call remote")
	}
}

func TestHybridExecutor_RoutesAsmToLocal(t *testing.T) {
	localCalled := false
	remoteCalled := false
	h := &HybridExecutor{
		Local: &MockExecutor{
			GenerateAssemblyFn: func(_ Language, _ map[string]string, _ string, _ []string) (string, error) {
				localCalled = true
				return "local_asm", nil
			},
		},
		Remote: &MockExecutor{
			GenerateAssemblyFn: func(_ Language, _ map[string]string, _ string, _ []string) (string, error) {
				remoteCalled = true
				return "remote_asm", nil
			},
		},
	}

	asm, err := h.GenerateAssembly(nil, nil, "", nil)
	if err != nil {
		t.Fatalf("GenerateAssembly: %v", err)
	}
	if asm != "local_asm" {
		t.Errorf("asm = %q, want local_asm", asm)
	}
	if !localCalled {
		t.Error("GenerateAssembly should call local")
	}
	if remoteCalled {
		t.Error("GenerateAssembly should NOT call remote")
	}
}

func TestHybridExecutor_RoutesSyntaxToLocal(t *testing.T) {
	localCalled := false
	remoteCalled := false
	h := &HybridExecutor{
		Local: &MockExecutor{
			SyntaxCheckFn: func(_ Language, _ map[string]string, _ string, _ []string) ([]Diagnostic, error) {
				localCalled = true
				return []Diagnostic{{Kind: "error", Line: 1, Message: "local"}}, nil
			},
		},
		Remote: &MockExecutor{
			SyntaxCheckFn: func(_ Language, _ map[string]string, _ string, _ []string) ([]Diagnostic, error) {
				remoteCalled = true
				return []Diagnostic{{Kind: "error", Line: 1, Message: "remote"}}, nil
			},
		},
	}

	diags, err := h.SyntaxCheck(nil, nil, "", nil)
	if err != nil {
		t.Fatalf("SyntaxCheck: %v", err)
	}
	if len(diags) != 1 || diags[0].Message != "local" {
		t.Errorf("unexpected diags: %v", diags)
	}
	if !localCalled {
		t.Error("SyntaxCheck should call local")
	}
	if remoteCalled {
		t.Error("SyntaxCheck should NOT call remote")
	}
}

func TestHybridExecutor_RoutesInteractiveToLocal(t *testing.T) {
	localCalled := false
	remoteCalled := false
	h := &HybridExecutor{
		Local: &MockExecutor{
			RunInteractiveFn: func(_ string, _ []string) (func([]byte) error, <-chan InteractiveEvent, func(), error) {
				localCalled = true
				return nil, nil, nil, nil
			},
		},
		Remote: &MockExecutor{
			RunInteractiveFn: func(_ string, _ []string) (func([]byte) error, <-chan InteractiveEvent, func(), error) {
				remoteCalled = true
				return nil, nil, nil, nil
			},
		},
	}

	_, _, _, err := h.RunInteractive("cmd", nil)
	if err != nil {
		t.Fatalf("RunInteractive: %v", err)
	}
	if !localCalled {
		t.Error("RunInteractive should call local")
	}
	if remoteCalled {
		t.Error("RunInteractive should NOT call remote")
	}
}

func TestHybridExecutor_RoutesASANToRemote(t *testing.T) {
	localCalled := false
	remoteCalled := false
	h := &HybridExecutor{
		Local: &MockExecutor{
			RunWithASANFn: func(_ Language, _ map[string]string, _, _ string, _, _ []string) (*Result, error) {
				localCalled = true
				return &Result{Output: "local"}, nil
			},
		},
		Remote: &MockExecutor{
			RunWithASANFn: func(_ Language, _ map[string]string, _, _ string, _, _ []string) (*Result, error) {
				remoteCalled = true
				return &Result{Output: "remote"}, nil
			},
		},
	}

	result, err := h.RunWithASAN(nil, nil, "", "", nil, nil)
	if err != nil {
		t.Fatalf("RunWithASAN: %v", err)
	}
	if result.Output != "remote" {
		t.Errorf("Output = %q, want remote", result.Output)
	}
	if localCalled {
		t.Error("RunWithASAN should NOT call local")
	}
	if !remoteCalled {
		t.Error("RunWithASAN should call remote")
	}
}

func TestHybridExecutor_RoutesDebugToRemote(t *testing.T) {
	localCalled := false
	remoteCalled := false
	h := &HybridExecutor{
		Local: &MockExecutor{
			CompileDebugFn: func(_ Language, _ map[string]string, _ string, _ []string) (*DebugBuild, *Result) {
				localCalled = true
				return &DebugBuild{BinaryPath: "local"}, nil
			},
		},
		Remote: &MockExecutor{
			CompileDebugFn: func(_ Language, _ map[string]string, _ string, _ []string) (*DebugBuild, *Result) {
				remoteCalled = true
				return &DebugBuild{BinaryPath: "remote"}, nil
			},
		},
	}

	build, compileResult := h.CompileDebug(nil, nil, "", nil)
	if compileResult != nil && compileResult.Error != "" {
		t.Fatalf("compile error: %s", compileResult.Error)
	}
	if build.BinaryPath != "remote" {
		t.Errorf("BinaryPath = %q, want remote", build.BinaryPath)
	}
	if localCalled {
		t.Error("CompileDebug should NOT call local")
	}
	if !remoteCalled {
		t.Error("CompileDebug should call remote")
	}
}

func TestHybridExecutor_RoutesTestsToRemote(t *testing.T) {
	localCalled := false
	remoteCalled := false
	tests := []TestInput{{Stdin: "", Expected: "hello"}}
	h := &HybridExecutor{
		Local: &MockExecutor{
			RunAllTestsFn: func(_ Language, _ map[string]string, _ string, _ []string, _ []TestInput) (*Result, []TestResult, error) {
				localCalled = true
				return &Result{}, []TestResult{{Num: 1, Passed: true}}, nil
			},
		},
		Remote: &MockExecutor{
			RunAllTestsFn: func(_ Language, _ map[string]string, _ string, _ []string, _ []TestInput) (*Result, []TestResult, error) {
				remoteCalled = true
				return &Result{}, []TestResult{{Num: 1, Passed: true}}, nil
			},
		},
	}

	compileRes, results, err := h.RunAllTests(nil, nil, "", nil, tests)
	if err != nil {
		t.Fatalf("RunAllTests: %v", err)
	}
	_ = compileRes
	_ = results
	if localCalled {
		t.Error("RunAllTests should NOT call local")
	}
	if !remoteCalled {
		t.Error("RunAllTests should call remote")
	}
}
