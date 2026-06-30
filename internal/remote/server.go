package remote

import (
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"net"
	"os"

	"ztutor/internal/logutil"
	"ztutor/internal/sandbox"
)

var execToken = os.Getenv("ZTUTOR_EXEC_TOKEN")

// ListenAndServe starts a TCP listener on addr and dispatches execution
// requests to the local sandbox. When ZTUTOR_EXEC_TOKEN is set, only requests
// bearing the matching token are accepted. Call this in a goroutine.
func ListenAndServe(addr string) error {
	return ListenAndServeTLS(addr, false, "", "", 0)
}

// ListenAndServeTLS starts a TCP (or TLS) listener. When tlsEnabled is true,
// certFile and keyFile are required. maxConns caps concurrent connections (0 = unlimited).
func ListenAndServeTLS(addr string, tlsEnabled bool, certFile, keyFile string, maxConns int) error {
	var ln net.Listener
	var err error

	if tlsEnabled {
		cert, cerr := tls.LoadX509KeyPair(certFile, keyFile)
		if cerr != nil {
			return cerr
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		ln, err = tls.Listen("tcp", addr, tlsCfg)
	} else {
		ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return err
	}

	logutil.Info("exec server listening on %s (tls: %v, auth: %v, max_conns: %d)",
		addr, tlsEnabled, execToken != "", maxConns)

	var sem chan struct{}
	if maxConns > 0 {
		sem = make(chan struct{}, maxConns)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		if sem != nil {
			select {
			case sem <- struct{}{}:
				go func() {
					defer func() { <-sem }()
					handleConn(conn)
				}()
			default:
				conn.Close()
			}
		} else {
			go handleConn(conn)
		}
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	remoteAddr := conn.RemoteAddr().String()

	var req ExecRequest
	if err := dec.Decode(&req); err != nil {
		logutil.Debug("exec request decode failed from %s: %v", remoteAddr, err)
		enc.Encode(ExecResponse{Error: "decode: " + err.Error()}) //nolint:errcheck
		return
	}
	logutil.Debug("exec request: remote=%s op=%s lang=%s files=%d", remoteAddr, req.Op, req.Lang, len(req.Files))

	if execToken != "" && subtle.ConstantTimeCompare([]byte(req.Token), []byte(execToken)) != 1 {
		logutil.Debug("exec auth failed: remote=%s op=%s lang=%s", remoteAddr, req.Op, req.Lang)
		enc.Encode(ExecResponse{Error: "auth failed: invalid or missing token"}) //nolint:errcheck
		return
	}

	resp := dispatch(req)
	if resp.Error != "" {
		logutil.Debug("exec response error: remote=%s op=%s lang=%s error=%s", remoteAddr, req.Op, req.Lang, resp.Error)
	} else {
		logutil.Debug("exec response ok: remote=%s op=%s lang=%s", remoteAddr, req.Op, req.Lang)
	}
	enc.Encode(resp) //nolint:errcheck
}

// effectiveFiles returns the files map for a request, falling back to the
// legacy Code field when Files is empty (protocol backward compat).
func effectiveFiles(req ExecRequest, lang sandbox.Language) map[string]string {
	if len(req.Files) > 0 {
		return req.Files
	}
	return map[string]string{lang.SourceFileName(): req.Code}
}

func dispatch(req ExecRequest) ExecResponse {
	if req.Op == OpPing {
		return ExecResponse{Op: req.Op}
	}

	lang := sandbox.GetLanguage(req.Lang)
	if lang == nil {
		return ExecResponse{Op: req.Op, Error: "unknown language: " + req.Lang}
	}

	files := effectiveFiles(req, lang)

	switch req.Op {
	case OpRun:
		r, err := sandbox.Run(lang, files, req.BuildCmd, req.Stdin, req.ExtraFlags, req.RuntimeArgs)
		if err != nil {
			return ExecResponse{Op: req.Op, Error: err.Error()}
		}
		return ExecResponse{Op: req.Op, Result: toResult(r)}

	case OpASAN:
		r, err := sandbox.RunWithASAN(lang, files, req.BuildCmd, req.Stdin, req.ExtraFlags, req.RuntimeArgs)
		if err != nil {
			return ExecResponse{Op: req.Op, Error: err.Error()}
		}
		return ExecResponse{Op: req.Op, Result: toResult(r)}

	case OpDebug:
		build, r := sandbox.CompileDebug(lang, files, req.BuildCmd, req.ExtraFlags)
		if build != nil {
			build.Close()
		}
		return ExecResponse{Op: req.Op, CompileResult: toResult(r)}

	case OpAsm:
		asm, err := sandbox.GenerateAssembly(lang, files, req.BuildCmd, req.ExtraFlags)
		if err != nil {
			return ExecResponse{Op: req.Op, Error: err.Error()}
		}
		return ExecResponse{Op: req.Op, Assembly: asm}

	case OpSyntax:
		diags, err := sandbox.SyntaxCheck(lang, files, req.BuildCmd, req.ExtraFlags)
		if err != nil {
			return ExecResponse{Op: req.Op, Error: err.Error()}
		}
		return ExecResponse{Op: req.Op, Diags: toDiags(diags)}

	case OpTests:
		compile, results, err := sandbox.RunAllTests(lang, files, req.BuildCmd, req.ExtraFlags, fromTestInputs(req.Tests))
		if err != nil {
			return ExecResponse{Op: req.Op, Error: err.Error()}
		}
		return ExecResponse{
			Op:            req.Op,
			CompileResult: toResult(compile),
			TestResults:   toTestResults(results),
		}

	default:
		return ExecResponse{Op: req.Op, Error: "unknown op: " + req.Op}
	}
}

func toResult(r *sandbox.Result) *Result {
	if r == nil {
		return nil
	}
	return &Result{Output: r.Output, Stdout: r.Stdout, Stderr: r.Stderr, ExitCode: r.ExitCode, Error: r.Error}
}

func toDiags(ds []sandbox.Diagnostic) []Diagnostic {
	out := make([]Diagnostic, len(ds))
	for i, d := range ds {
		out[i] = Diagnostic{Line: d.Line, Col: d.Col, Kind: d.Kind, Message: d.Message}
	}
	return out
}

func toTestResults(rs []sandbox.TestResult) []TestResult {
	out := make([]TestResult, len(rs))
	for i, r := range rs {
		out[i] = TestResult{Num: r.Num, Passed: r.Passed, Got: r.Got, Want: r.Want, Error: r.Error, ExitCode: r.ExitCode}
	}
	return out
}

func fromTestInputs(ts []TestInput) []sandbox.TestInput {
	out := make([]sandbox.TestInput, len(ts))
	for i, t := range ts {
		out[i] = sandbox.TestInput{
			Stdin:             t.Stdin,
			Args:              t.Args,
			Expected:          t.Expected,
			ExpectedStdout:    t.ExpectedStdout,
			ExpectedStderr:    t.ExpectedStderr,
			HasExpectedStdout: t.HasExpectedStdout,
			HasExpectedStderr: t.HasExpectedStderr,
		}
	}
	return out
}
