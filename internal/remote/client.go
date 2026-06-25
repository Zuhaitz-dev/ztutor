package remote

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"ztutor/internal/logutil"
	"ztutor/internal/sandbox"
)

// Client implements sandbox.Executor by forwarding all calls to a remote
// ztutord execution endpoint over TCP (one connection per request).
type Client struct {
	addr  string
	token string
	tls   bool
}

// NewClient returns a Client that connects to addr for each execution request.
// If ZTUTOR_EXEC_TOKEN is set, it is included as the shared-secret auth token.
func NewClient(addr string) *Client {
	return NewClientTLS(addr, false)
}

// NewClientTLS returns a Client with optional TLS.
func NewClientTLS(addr string, useTLS bool) *Client {
	return &Client{
		addr:  addr,
		token: os.Getenv("ZTUTOR_EXEC_TOKEN"),
		tls:   useTLS,
	}
}

// NewClientWithToken returns a Client with an explicit auth token and optional TLS.
// Use this when the token comes from user configuration rather than the environment.
func NewClientWithToken(addr, token string, useTLS bool) *Client {
	return &Client{addr: addr, token: token, tls: useTLS}
}

func (c *Client) call(req ExecRequest) (ExecResponse, error) {
	req.Token = c.token
	logutil.Debug("remote exec request: op=%s lang=%s addr=%s tls=%v files=%d", req.Op, req.Lang, c.addr, c.tls, len(req.Files))

	var conn net.Conn
	var err error
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	if c.tls {
		conn, err = tls.DialWithDialer(dialer, "tcp", c.addr, &tls.Config{MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.Dial("tcp", c.addr)
	}
	if err != nil {
		return ExecResponse{}, fmt.Errorf("remote exec: dial %s: %w", c.addr, err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return ExecResponse{}, fmt.Errorf("remote exec: send: %w", err)
	}

	var resp ExecResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return ExecResponse{}, fmt.Errorf("remote exec: recv: %w", err)
	}
	if resp.Error != "" {
		logutil.Debug("remote exec response error: op=%s lang=%s error=%s", req.Op, req.Lang, resp.Error)
		return ExecResponse{}, fmt.Errorf("remote exec: %s", resp.Error)
	}
	logutil.Debug("remote exec response ok: op=%s lang=%s", req.Op, req.Lang)
	return resp, nil
}

func (c *Client) Ping() error {
	_, err := c.call(ExecRequest{Op: OpPing})
	return err
}

func (c *Client) Run(lang sandbox.Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*sandbox.Result, error) {
	resp, err := c.call(ExecRequest{
		Op: OpRun, Lang: lang.Name(),
		Files: files, BuildCmd: buildCmd,
		Stdin: stdin, ExtraFlags: extraFlags, RuntimeArgs: runtimeArgs,
	})
	if err != nil {
		return nil, err
	}
	return fromResult(resp.Result), nil
}

func (c *Client) RunWithASAN(lang sandbox.Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*sandbox.Result, error) {
	resp, err := c.call(ExecRequest{
		Op: OpASAN, Lang: lang.Name(),
		Files: files, BuildCmd: buildCmd,
		Stdin: stdin, ExtraFlags: extraFlags, RuntimeArgs: runtimeArgs,
	})
	if err != nil {
		return nil, err
	}
	return fromResult(resp.Result), nil
}

func (c *Client) CompileDebug(lang sandbox.Language, files map[string]string, buildCmd string, extraFlags []string) (*sandbox.DebugBuild, *sandbox.Result) {
	resp, err := c.call(ExecRequest{
		Op: OpDebug, Lang: lang.Name(),
		Files: files, BuildCmd: buildCmd, ExtraFlags: extraFlags,
	})
	if err != nil {
		return nil, &sandbox.Result{Error: err.Error()}
	}
	return nil, fromResult(resp.CompileResult)
}

func (c *Client) GenerateAssembly(lang sandbox.Language, files map[string]string, buildCmd string, extraFlags []string) (string, error) {
	resp, err := c.call(ExecRequest{
		Op: OpAsm, Lang: lang.Name(),
		Files: files, BuildCmd: buildCmd, ExtraFlags: extraFlags,
	})
	if err != nil {
		return "", err
	}
	return resp.Assembly, nil
}

func (c *Client) SyntaxCheck(lang sandbox.Language, files map[string]string, buildCmd string, extraFlags []string) ([]sandbox.Diagnostic, error) {
	resp, err := c.call(ExecRequest{
		Op: OpSyntax, Lang: lang.Name(),
		Files: files, BuildCmd: buildCmd, ExtraFlags: extraFlags,
	})
	if err != nil {
		return nil, err
	}
	return fromDiags(resp.Diags), nil
}

func (c *Client) RunAllTests(lang sandbox.Language, files map[string]string, buildCmd string, extraFlags []string, tests []sandbox.TestInput) (*sandbox.Result, []sandbox.TestResult, error) {
	wired := make([]TestInput, len(tests))
	for i, t := range tests {
		wired[i] = TestInput{Stdin: t.Stdin, Args: t.Args, Expected: t.Expected}
	}
	resp, err := c.call(ExecRequest{
		Op: OpTests, Lang: lang.Name(),
		Files: files, BuildCmd: buildCmd, ExtraFlags: extraFlags, Tests: wired,
	})
	if err != nil {
		return nil, nil, err
	}
	return fromResult(resp.CompileResult), fromTestResults(resp.TestResults), nil
}

func (c *Client) RunInteractive(command string, args []string) (func([]byte) error, <-chan sandbox.InteractiveEvent, func(), error) {
	return nil, nil, nil, fmt.Errorf("remote exec: interactive mode requires SSH connection to ztutord")
}

// wire type → sandbox type converters

func fromResult(r *Result) *sandbox.Result {
	if r == nil {
		return &sandbox.Result{}
	}
	return &sandbox.Result{Output: r.Output, ExitCode: r.ExitCode, Error: r.Error}
}

func fromDiags(ds []Diagnostic) []sandbox.Diagnostic {
	out := make([]sandbox.Diagnostic, len(ds))
	for i, d := range ds {
		out[i] = sandbox.Diagnostic{Line: d.Line, Col: d.Col, Kind: d.Kind, Message: d.Message}
	}
	return out
}

func fromTestResults(rs []TestResult) []sandbox.TestResult {
	out := make([]sandbox.TestResult, len(rs))
	for i, r := range rs {
		out[i] = sandbox.TestResult{Num: r.Num, Passed: r.Passed, Got: r.Got, Want: r.Want, Error: r.Error, ExitCode: r.ExitCode}
	}
	return out
}
