package remote

import (
	"encoding/json"
	"net"
	"testing"

	"ztutor/internal/sandbox"
)

func mustPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close(); b.Close() })
	return a, b
}

func TestHandleConn_DecodeError(t *testing.T) {
	server, client := mustPair(t)
	done := make(chan struct{})
	go func() {
		handleConn(server)
		close(done)
	}()

	// Send garbage — should get an error response.
	client.Write([]byte("not-json\n"))
	var resp ExecResponse
	if err := json.NewDecoder(client).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected error for garbage input")
	}
	client.Close()
	<-done
}

func TestHandleConn_UnknownLanguage(t *testing.T) {
	server, client := mustPair(t)
	done := make(chan struct{})
	go func() {
		handleConn(server)
		close(done)
	}()

	req := ExecRequest{Op: OpRun, Lang: "zz"}
	json.NewEncoder(client).Encode(req)
	var resp ExecResponse
	json.NewDecoder(client).Decode(&resp)
	if resp.Error == "" || resp.Error != "unknown language: zz" {
		t.Errorf("expected unknown language error, got: %q", resp.Error)
	}
	client.Close()
	<-done
}

func TestHandleConn_UnknownOp(t *testing.T) {
	server, client := mustPair(t)
	done := make(chan struct{})
	go func() {
		handleConn(server)
		close(done)
	}()

	req := ExecRequest{Op: "nope", Lang: "c"}
	json.NewEncoder(client).Encode(req)
	var resp ExecResponse
	json.NewDecoder(client).Decode(&resp)
	if resp.Error == "" {
		t.Error("expected error for unknown op")
	}
	client.Close()
	<-done
}

func TestHandleConn_Ping(t *testing.T) {
	server, client := mustPair(t)
	done := make(chan struct{})
	go func() {
		handleConn(server)
		close(done)
	}()

	req := ExecRequest{Op: OpPing}
	json.NewEncoder(client).Encode(req)
	var resp ExecResponse
	json.NewDecoder(client).Decode(&resp)
	if resp.Op != OpPing || resp.Error != "" {
		t.Fatalf("ping response = %+v, want successful ping", resp)
	}
	client.Close()
	<-done
}

func TestHandleConn_AuthFailure(t *testing.T) {
	server, client := mustPair(t)
	done := make(chan struct{})
	oldToken := execToken
	execToken = "secret"
	defer func() { execToken = oldToken }()
	go func() {
		handleConn(server)
		close(done)
	}()

	req := ExecRequest{Op: OpPing}
	json.NewEncoder(client).Encode(req)
	var resp ExecResponse
	if err := json.NewDecoder(client).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "auth failed: invalid or missing token" {
		t.Fatalf("resp.Error = %q, want auth failure", resp.Error)
	}
	client.Close()
	<-done
}

func TestDispatch_RunSuccess(t *testing.T) {
	// Only test dispatch with languages that are available on this system.
	lang := sandbox.GetLanguage("c")
	if lang == nil {
		t.Skip("C toolchain not available")
	}
	resp := dispatch(ExecRequest{
		Op:   OpRun,
		Lang: "c",
		Files: map[string]string{
			"main.c": "int main() { return 0; }",
		},
	})
	if resp.Error != "" {
		t.Errorf("run error: %s", resp.Error)
	}
	if resp.Result == nil || resp.Result.ExitCode != 0 {
		t.Error("expected successful run with exit code 0")
	}
}

func TestDispatch_RunCompileError(t *testing.T) {
	lang := sandbox.GetLanguage("c")
	if lang == nil {
		t.Skip("C toolchain not available")
	}
	resp := dispatch(ExecRequest{
		Op:   OpRun,
		Lang: "c",
		Files: map[string]string{
			"main.c": "this is not valid C",
		},
	})
	if resp.Error != "" {
		t.Skipf("toolchain error: %s", resp.Error)
	}
	if resp.Result == nil || resp.Result.Error == "" {
		t.Error("expected compile error")
	}
}

func TestDispatch_Syntax(t *testing.T) {
	lang := sandbox.GetLanguage("c")
	if lang == nil {
		t.Skip("C toolchain not available")
	}
	resp := dispatch(ExecRequest{
		Op:   OpSyntax,
		Lang: "c",
		Files: map[string]string{
			"main.c": "int main() { return 0; }",
		},
	})
	if resp.Error != "" {
		t.Errorf("syntax check error: %s", resp.Error)
	}
	// Clean code should produce no diagnostics.
	if len(resp.Diags) > 0 {
		t.Errorf("clean code should have no diagnostics, got %d", len(resp.Diags))
	}
}

func TestDispatch_SyntaxError(t *testing.T) {
	lang := sandbox.GetLanguage("c")
	if lang == nil {
		t.Skip("C toolchain not available")
	}
	resp := dispatch(ExecRequest{
		Op:   OpSyntax,
		Lang: "c",
		Files: map[string]string{
			"main.c": "int main() { return }",
		},
	})
	if resp.Error != "" {
		t.Skipf("toolchain error: %s", resp.Error)
	}
	if len(resp.Diags) == 0 {
		t.Error("expected diagnostics for syntax error")
	}
}

func TestDispatch_Asm(t *testing.T) {
	lang := sandbox.GetLanguage("c")
	if lang == nil {
		t.Skip("C toolchain not available")
	}
	resp := dispatch(ExecRequest{
		Op:   OpAsm,
		Lang: "c",
		Files: map[string]string{
			"main.c": "int main() { return 0; }",
		},
	})
	if resp.Error != "" {
		t.Skipf("asm error: %s", resp.Error)
	}
	if resp.Assembly == "" {
		t.Error("expected assembly output")
	}
}

func TestDispatch_Tests(t *testing.T) {
	lang := sandbox.GetLanguage("c")
	if lang == nil {
		t.Skip("C toolchain not available")
	}
	resp := dispatch(ExecRequest{
		Op:   OpTests,
		Lang: "c",
		Files: map[string]string{
			"main.c": "int main() { return 0; }",
		},
		Tests: []TestInput{{Expected: "", Stdin: ""}},
	})
	if resp.Error != "" {
		t.Skipf("tests error: %s", resp.Error)
	}
	if resp.CompileResult == nil || resp.CompileResult.Error != "" {
		t.Errorf("compile failed: %v", resp.CompileResult)
	}
}

func TestDispatch_UnknownLanguage(t *testing.T) {
	resp := dispatch(ExecRequest{
		Op:   OpRun,
		Lang: "zz",
	})
	if resp.Error != "unknown language: zz" {
		t.Errorf("got %q, want unknown language error", resp.Error)
	}
}

func TestResultConversion_RoundTrip(t *testing.T) {
	r := &sandbox.Result{
		Output:   "hello",
		ExitCode: 42,
		Error:    "oops",
	}
	wire := toResult(r)
	back := fromResult(wire)
	if back.Output != r.Output || back.ExitCode != r.ExitCode || back.Error != r.Error {
		t.Errorf("round-trip mismatch: %+v != %+v", r, back)
	}
}

func TestResultConversion_Nil(t *testing.T) {
	if toResult(nil) != nil {
		t.Error("toResult(nil) should be nil")
	}
	if fromResult(nil) == nil {
		t.Error("fromResult(nil) should not be nil")
	}
	if fromResult(nil).ExitCode != 0 {
		t.Error("fromResult(nil) should have ExitCode 0")
	}
}

func TestEffectiveFiles_LegacyCodeFallback(t *testing.T) {
	lang := sandbox.GetLanguage("c")
	if lang == nil {
		t.Skip("C toolchain not available")
	}
	got := effectiveFiles(ExecRequest{Code: "int main(){return 0;}"}, lang)
	if got[lang.SourceFileName()] == "" {
		t.Fatalf("effectiveFiles did not populate legacy code fallback: %v", got)
	}
}

func TestFromTestInputs_PreservesStreamExpectations(t *testing.T) {
	in := []TestInput{{
		Stdin:             "in",
		Args:              []string{"a"},
		ExpectedStdout:    "out",
		ExpectedStderr:    "err",
		HasExpectedStdout: true,
		HasExpectedStderr: true,
	}}
	got := fromTestInputs(in)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].ExpectedStdout != "out" || got[0].ExpectedStderr != "err" || !got[0].HasExpectedStdout || !got[0].HasExpectedStderr {
		t.Fatalf("fromTestInputs lost stream expectations: %+v", got[0])
	}
}

func TestClient_BadAddr(t *testing.T) {
	client := NewClient("127.0.0.1:1")
	_, err := client.Run(sandbox.GetLanguage("c"), nil, "", "", nil, nil)
	if err == nil {
		t.Error("expected error connecting to bad address")
	}
}

func TestClient_AuthRejection(t *testing.T) {
	// Start a local server that requires a token.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen not permitted in this environment: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()

	// Set token that doesn't match client.
	oldToken := execToken
	execToken = "secret"
	defer func() { execToken = oldToken }()

	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			handleConn(conn)
		}
	}()

	// Client without token should get auth error.
	client := NewClient(addr)
	_, err = client.Run(sandbox.GetLanguage("c"), nil, "", "", nil, nil)
	if err == nil {
		t.Error("expected auth error")
	}
}
