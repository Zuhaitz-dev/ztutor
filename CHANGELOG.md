# Changelog

## v0.2.0 -- Fully open: licensing removed

### Added
- Everything is now open. All courses and lessons are available to every user.
- First account created on a fresh server is always the admin account.
- Windows support: the client and server build and run natively; the sandbox compiles, runs, and tests student code, and interactive mode plus the in-TUI debugger work via ConPTY. Builds ship as `ztutor.exe`/`ztutord.exe`. Windows CI (build + vet, then the full suite with MinGW).

### Changed
- Dependencies moved to current major releases: bubbletea v1, bubbles v1, glamour v1, lipgloss v1, modernc.org/sqlite v1.56, chroma v2.27. Requires Go 1.25+.
- `premium` and `enrollment` keys in existing course content are ignored (harmless).
- Connect screen no longer offers a license entry option.

### Removed
- License system (`internal/license`): tiers, premium gating, seat limits, interview gating, license entry/summary screens.
- Encrypted `.course` packages (`internal/crypt`, `cmd/coursepack`): courses are plain directories.
- `cmd/licensegen` and its runtime artifacts (`dev_license_keys.json`, `license_test.key`, `license_test_school.key`).
- Patron/credits tiers (`Maecenas` section in the Credits screen).
- `enrollment.required` gating in course.yaml and `premium` lesson badges.
- `ZTUTOR_LICENSE_PUBKEY` / `ZTUTOR_LICENSE_FILE` env and config plumbing, Makefile targets, Docker/service wiring, and `LICENSE-COMMERCIAL`.
- `license_redemptions` database table (dropped by a new migration).

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
