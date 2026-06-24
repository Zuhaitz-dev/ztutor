// Package remote implements the ztutor remote execution protocol.
// ztutord listens on a TCP address (config.Exec.Addr) and accepts JSON
// execution requests. The standalone ztutor client connects via remote.Client
// (activated by ZTUTOR_EXEC_ADDR) to run code on the server instead of locally.
package remote

// Op constants for ExecRequest.Op.
const (
	OpRun    = "run"
	OpASAN   = "run_asan"
	OpDebug  = "compile_debug"
	OpAsm    = "asm"
	OpSyntax = "syntax"
	OpTests  = "run_tests"
)

// ExecRequest is sent by the client for every execution operation.
// Files takes precedence over Code; Code is kept for protocol backward compat.
type ExecRequest struct {
	Op          string            `json:"op"`
	Lang        string            `json:"lang"`
	Files       map[string]string `json:"files,omitempty"`
	BuildCmd    string            `json:"build_cmd,omitempty"`
	Code        string            `json:"code,omitempty"` // legacy single-file fallback
	Stdin       string            `json:"stdin,omitempty"`
	ExtraFlags  []string          `json:"extra_flags,omitempty"`
	RuntimeArgs []string          `json:"runtime_args,omitempty"`
	Tests       []TestInput       `json:"tests,omitempty"`
	Token       string            `json:"token,omitempty"` // shared-secret auth token
}

// TestInput mirrors sandbox.TestInput for wire encoding.
type TestInput struct {
	Stdin    string   `json:"stdin"`
	Args     []string `json:"args,omitempty"`
	Expected string   `json:"expected"`
}

// ExecResponse is returned by the server after executing an ExecRequest.
// Which fields are populated depends on the Op.
type ExecResponse struct {
	Op            string       `json:"op"`
	Result        *Result      `json:"result,omitempty"`
	CompileResult *Result      `json:"compile_result,omitempty"`
	TestResults   []TestResult `json:"test_results,omitempty"`
	Assembly      string       `json:"assembly,omitempty"`
	Diags         []Diagnostic `json:"diags,omitempty"`
	Error         string       `json:"error,omitempty"`
}

// Result mirrors sandbox.Result for wire encoding.
type Result struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// TestResult mirrors sandbox.TestResult for wire encoding.
type TestResult struct {
	Num      int    `json:"num"`
	Passed   bool   `json:"passed"`
	Got      string `json:"got"`
	Want     string `json:"want"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code"`
}

// Diagnostic mirrors sandbox.Diagnostic for wire encoding.
type Diagnostic struct {
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}
