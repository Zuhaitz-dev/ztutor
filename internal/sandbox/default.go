//go:build !tcc

package sandbox

import "sync"

var defaultExecMu sync.RWMutex
var defaultExec Executor = LocalExecutor{}

// DefaultExecutor returns the current default executor (thread-safe).
func DefaultExecutor() Executor {
	defaultExecMu.RLock()
	defer defaultExecMu.RUnlock()
	return defaultExec
}

// SetDefaultExecutor replaces the default executor (thread-safe).
// Call before starting the TUI; concurrent access is protected.
func SetDefaultExecutor(e Executor) {
	defaultExecMu.Lock()
	defer defaultExecMu.Unlock()
	defaultExec = e
}
