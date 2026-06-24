package sandbox

type HybridExecutor struct {
	Local  Executor
	Remote Executor
}

func (h *HybridExecutor) Run(lang Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*Result, error) {
	return h.Remote.Run(lang, files, buildCmd, stdin, extraFlags, runtimeArgs)
}

func (h *HybridExecutor) RunWithASAN(lang Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*Result, error) {
	return h.Remote.RunWithASAN(lang, files, buildCmd, stdin, extraFlags, runtimeArgs)
}

func (h *HybridExecutor) CompileDebug(lang Language, files map[string]string, buildCmd string, extraFlags []string) (*DebugBuild, *Result) {
	return h.Remote.CompileDebug(lang, files, buildCmd, extraFlags)
}

func (h *HybridExecutor) GenerateAssembly(lang Language, files map[string]string, buildCmd string, extraFlags []string) (string, error) {
	return h.Local.GenerateAssembly(lang, files, buildCmd, extraFlags)
}

func (h *HybridExecutor) SyntaxCheck(lang Language, files map[string]string, buildCmd string, extraFlags []string) ([]Diagnostic, error) {
	return h.Local.SyntaxCheck(lang, files, buildCmd, extraFlags)
}

func (h *HybridExecutor) RunAllTests(lang Language, files map[string]string, buildCmd string, extraFlags []string, tests []TestInput) (*Result, []TestResult, error) {
	return h.Remote.RunAllTests(lang, files, buildCmd, extraFlags, tests)
}

func (h *HybridExecutor) RunInteractive(cmd string, args []string) (func([]byte) error, <-chan InteractiveEvent, func(), error) {
	return h.Local.RunInteractive(cmd, args)
}
