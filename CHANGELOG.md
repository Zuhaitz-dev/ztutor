# Changelog

## v0.1.18 -- GDB in TUI, course fixes, content checks, versioning

### Added
- GDB debugger now runs inside the TUI instead of taking over the terminal. Ctrl+G compiles with debug symbols and opens GDB in the output panel. Ctrl+G toggles fullscreen output, Ctrl+Q exits.
- Sandbox resource limits are configurable via `ztutor.json` (sandbox section) or `ZTUTOR_SANDBOX_MAX_*` env vars.
- Content integrity tests verify headers compile, include paths resolve, expected.txt is non-empty, and lesson frontmatter is valid.
- `make bump VER=x.y.z` target for version tagging.
- CI step to verify course manifests are up to date.

### Changed
- GDB's `shell` command is now disabled via init file to prevent sandbox escape.
- Sandbox PATH is now a curated whitelist (`/usr/local/bin:/usr/bin:/bin`) instead of inheriting the host PATH.
- User code input is capped at 100 files and 10 MB total to prevent DoS.
- `make test` now verifies course manifests before running tests.

### Fixed
- License entry screen: typing 'q' in a file path no longer exits the screen.
- Signal tests (SIGFPE, SIGSEGV) no longer fail in containerized CI or ARM64 environments.
- Darwin build: `Pdeathsig` field moved to Linux-only platform file.
- Setup token now logged as a prefix instead of full plaintext.
- Database connection pool is now bounded at 25 connections.
- Dead code removed: `runGDBSession`, `launchGDB` callback, and GDB quit/restart loop.
- Node 02: `sqliteInt.h` typedefs fixed to use `<stdint.h>` types instead of missing SQLite headers.
- Node 07: `expected.txt` updated to match starter code output.
- Node 11: `cryptlib.h` moved to `internal/` directory to match include path.
- GDB ANSI escape codes stripped from output display.
- Several error paths now check and report previously ignored `os.WriteFile` and `rand.Read` errors.
- TOCTOU race closed in setup token validation.
- Log injection blocked for user-controlled fields in remote exec server logs.
- Exec server now shuts down gracefully with context cancellation.
- `BuildTarGz` no longer accumulates deferred file closures.
- `ensureProg` now logs a warning on rename failure.

### Removed
- `runGDBSession` SSH PTY proxy (replaced by in-TUI GDB).
- `launchGDB` callback from NewApp and TUIProvider.
