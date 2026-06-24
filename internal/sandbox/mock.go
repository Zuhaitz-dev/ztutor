package sandbox

import "fmt"

// MockExecutor implements Executor with configurable responses for testing.
type MockExecutor struct {
	RunFn              func(Language, map[string]string, string, string, []string, []string) (*Result, error)
	RunWithASANFn      func(Language, map[string]string, string, string, []string, []string) (*Result, error)
	CompileDebugFn     func(Language, map[string]string, string, []string) (*DebugBuild, *Result)
	GenerateAssemblyFn func(Language, map[string]string, string, []string) (string, error)
	SyntaxCheckFn      func(Language, map[string]string, string, []string) ([]Diagnostic, error)
	RunAllTestsFn      func(Language, map[string]string, string, []string, []TestInput) (*Result, []TestResult, error)
	RunInteractiveFn   func(string, []string) (func([]byte) error, <-chan InteractiveEvent, func(), error)
}

var _ Executor = (*MockExecutor)(nil)

func (m *MockExecutor) Run(lang Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*Result, error) {
	if m.RunFn != nil {
		return m.RunFn(lang, files, buildCmd, stdin, extraFlags, runtimeArgs)
	}
	return nil, fmt.Errorf("mock Run not configured")
}

func (m *MockExecutor) RunWithASAN(lang Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*Result, error) {
	if m.RunWithASANFn != nil {
		return m.RunWithASANFn(lang, files, buildCmd, stdin, extraFlags, runtimeArgs)
	}
	return nil, fmt.Errorf("mock RunWithASAN not configured")
}

func (m *MockExecutor) CompileDebug(lang Language, files map[string]string, buildCmd string, extraFlags []string) (*DebugBuild, *Result) {
	if m.CompileDebugFn != nil {
		return m.CompileDebugFn(lang, files, buildCmd, extraFlags)
	}
	return nil, &Result{Error: "mock CompileDebug not configured"}
}

func (m *MockExecutor) GenerateAssembly(lang Language, files map[string]string, buildCmd string, extraFlags []string) (string, error) {
	if m.GenerateAssemblyFn != nil {
		return m.GenerateAssemblyFn(lang, files, buildCmd, extraFlags)
	}
	return "", fmt.Errorf("mock GenerateAssembly not configured")
}

func (m *MockExecutor) SyntaxCheck(lang Language, files map[string]string, buildCmd string, extraFlags []string) ([]Diagnostic, error) {
	if m.SyntaxCheckFn != nil {
		return m.SyntaxCheckFn(lang, files, buildCmd, extraFlags)
	}
	return nil, fmt.Errorf("mock SyntaxCheck not configured")
}

func (m *MockExecutor) RunAllTests(lang Language, files map[string]string, buildCmd string, extraFlags []string, tests []TestInput) (*Result, []TestResult, error) {
	if m.RunAllTestsFn != nil {
		return m.RunAllTestsFn(lang, files, buildCmd, extraFlags, tests)
	}
	return nil, nil, fmt.Errorf("mock RunAllTests not configured")
}

func (m *MockExecutor) RunInteractive(command string, args []string) (func([]byte) error, <-chan InteractiveEvent, func(), error) {
	if m.RunInteractiveFn != nil {
		return m.RunInteractiveFn(command, args)
	}
	return nil, nil, nil, fmt.Errorf("mock RunInteractive not configured")
}

// NewSuccessExecutor returns a MockExecutor that always returns success.
func NewSuccessExecutor() *MockExecutor {
	return &MockExecutor{
		RunFn: func(_ Language, _ map[string]string, _, _ string, _, _ []string) (*Result, error) {
			return &Result{Output: "hello", ExitCode: 0}, nil
		},
		RunWithASANFn: func(_ Language, _ map[string]string, _, _ string, _, _ []string) (*Result, error) {
			return &Result{Output: "hello", ExitCode: 0}, nil
		},
		CompileDebugFn: func(_ Language, _ map[string]string, _ string, _ []string) (*DebugBuild, *Result) {
			return &DebugBuild{}, nil
		},
		RunAllTestsFn: func(_ Language, _ map[string]string, _ string, _ []string, tests []TestInput) (*Result, []TestResult, error) {
			results := make([]TestResult, len(tests))
			for i := range tests {
				results[i] = TestResult{Num: i + 1, Passed: true}
			}
			return &Result{}, results, nil
		},
	}
}

// NewErrorExecutor returns a MockExecutor that always returns errors.
func NewErrorExecutor() *MockExecutor {
	return &MockExecutor{
		RunFn: func(_ Language, _ map[string]string, _, _ string, _, _ []string) (*Result, error) {
			return nil, fmt.Errorf("execution failed")
		},
		RunWithASANFn: func(_ Language, _ map[string]string, _, _ string, _, _ []string) (*Result, error) {
			return nil, fmt.Errorf("asan failed")
		},
		CompileDebugFn: func(_ Language, _ map[string]string, _ string, _ []string) (*DebugBuild, *Result) {
			return nil, &Result{Error: "compile error"}
		},
		RunAllTestsFn: func(_ Language, _ map[string]string, _ string, _ []string, _ []TestInput) (*Result, []TestResult, error) {
			return nil, nil, fmt.Errorf("tests failed")
		},
	}
}
