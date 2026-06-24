package sandbox

// Executor abstracts the code execution backend. LocalExecutor wraps the
// existing sandbox functions; RemoteExecutor forwards to ztutord via TCP.
type Executor interface {
	// files is a map of filename → content; buildCmd overrides the language compiler.
	Run(lang Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*Result, error)
	RunWithASAN(lang Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*Result, error)
	CompileDebug(lang Language, files map[string]string, buildCmd string, extraFlags []string) (*DebugBuild, *Result)
	GenerateAssembly(lang Language, files map[string]string, buildCmd string, extraFlags []string) (string, error)
	SyntaxCheck(lang Language, files map[string]string, buildCmd string, extraFlags []string) ([]Diagnostic, error)
	RunAllTests(lang Language, files map[string]string, buildCmd string, extraFlags []string, tests []TestInput) (*Result, []TestResult, error)
	RunInteractive(command string, args []string) (write func([]byte) error, events <-chan InteractiveEvent, kill func(), err error)
}

// LocalExecutor delegates to the package-level sandbox functions (system GCC).
type LocalExecutor struct{}

func (LocalExecutor) Run(lang Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*Result, error) {
	return Run(lang, files, buildCmd, stdin, extraFlags, runtimeArgs)
}

func (LocalExecutor) RunWithASAN(lang Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*Result, error) {
	return RunWithASAN(lang, files, buildCmd, stdin, extraFlags, runtimeArgs)
}

func (LocalExecutor) CompileDebug(lang Language, files map[string]string, buildCmd string, extraFlags []string) (*DebugBuild, *Result) {
	return CompileDebug(lang, files, buildCmd, extraFlags)
}

func (LocalExecutor) GenerateAssembly(lang Language, files map[string]string, buildCmd string, extraFlags []string) (string, error) {
	return GenerateAssembly(lang, files, buildCmd, extraFlags)
}

func (LocalExecutor) SyntaxCheck(lang Language, files map[string]string, buildCmd string, extraFlags []string) ([]Diagnostic, error) {
	return SyntaxCheck(lang, files, buildCmd, extraFlags)
}

func (LocalExecutor) RunAllTests(lang Language, files map[string]string, buildCmd string, extraFlags []string, tests []TestInput) (*Result, []TestResult, error) {
	return RunAllTests(lang, files, buildCmd, extraFlags, tests)
}

func (LocalExecutor) RunInteractive(command string, args []string) (func([]byte) error, <-chan InteractiveEvent, func(), error) {
	return RunInteractive(command, args)
}

// RemoteExecutor is implemented by internal/remote.Client.
// Use remote.NewClient(addr) to obtain an Executor that forwards calls to ztutord.
