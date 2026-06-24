package sandbox

type Diagnostic struct {
	Line    int
	Col     int
	Kind    string // "error", "warning", or "note"
	Message string
}
