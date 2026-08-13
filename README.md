# ztutor

An interactive programming tutor delivered over SSH. Students connect with a standard SSH client, read lessons, write code in a syntax-highlighted editor, and run it in a sandboxed environment -- all from the terminal.

Supports C, C++, Python, Rust, Go, Ruby, and Java out of the box. Adding a new programming language takes around 40 lines of Go. The interface ships in English, Spanish, Arabic, and Chinese, and adding a new display language is a single YAML file.

## How it works

1. The server administrator runs `ztutord` on a Linux or macOS machine.
2. Students connect over SSH -- no client software required.
3. Students read lessons, write code in the built-in editor, and submit for immediate feedback.
4. Code runs in a sandbox with resource limits and namespace isolation.

## Quick start

```bash
make build
./ztutord
```

On first run the server prints a setup token and the SSH address to connect to. The token is valid for 24 hours and locks after 5 failed attempts.

The first connection creates the admin account. Later connections create (or log into) student accounts.

Connect in another terminal:

```bash
ssh yourname@localhost -p 2222
```

Paste the setup token when prompted to create the first (admin) account.

### Local admin dashboard

If you are running `ztutord` on the same machine you are sitting at, use the `-local` flag to open the admin dashboard directly in your terminal instead of SSHing in:

```bash
./ztutord -local
```

The SSH server still starts in the background so students can connect while you manage the server from the same terminal session.

Press `q` or `Ctrl+C` in the local interface to exit; the SSH server shuts down cleanly.

The included `courses/c-programming` course ships 15 free lessons covering C foundations (Module 1). No C experience needed; prior programming experience in any language is assumed. All course content is open. Additional courses are plain directories (or tarballs extracted into `courses/`). If `courses/` is empty or not mounted, the app starts with an empty course menu.

### Local controller support (Linux only)

The local `ztutor` client supports two controller input paths:

**Path 1: native Linux gamepad** (`/dev/input`)

Plug in a controller. If it registers as an `event-joystick` device (most USB/Bluetooth gamepads do), ztutor picks it up automatically. A DualShock 4 or Xbox controller usually works without any extra setup.

| Button | Xbox | PlayStation | Action |
|--------|------|-------------|--------|
| South | A | Cross ✕ | Select / confirm |
| East | B | Circle ○ | Back / cancel |
| West | X | Square □ | Run / submit (Ctrl+S) |
| North | Y | Triangle △ | Hint (?) |
| D-pad / left stick | ↑↓←→ | ↑↓←→ | Navigate |
| LB / L1 or Select | LB | L1 | Previous section (Shift+Tab) |
| RB / R1 | RB | R1 | Next section (Tab) |
| Start / Options | Start | Options | Keybindings overlay (F1) |

If the controller is detected but does not respond, check that your user can read `/dev/input/event*`. On most Linux systems that means adding the user to the `input` group:

```bash
sudo usermod -aG input $USER  # log out and back in after this
```

Or install a udev rule that grants access for a specific USB vendor/product ID.

**Path 2: keyboard mapper (F13-F20)**

Use this if your controller appears as a generic HID device or you want a software remapper like `xboxdrv`, `antimicro`, `reWASD`, or Steam Input. Configure your mapper to send these key codes:

| F-key | Action |
|-------|--------|
| F13 | Select (Enter) |
| F14 | Back (Esc) |
| F15 | Up |
| F16 | Down |
| F17 | Left |
| F18 | Right |
| F19 | Run / submit (Ctrl+S) |
| F20 | Hint (?) |

Both paths work simultaneously; you can have the native driver and a mapper active at the same time.

## Deployment

### Docker

```bash
cp .env.example .env
docker compose up -d
```

Students connect with: `ssh username@yourhost -p 2222`

The Docker service mounts `./courses` into the container as read-only course content. A fresh checkout includes `courses/c-programming` (Module 1, 15 free lessons). Add additional course directories to the mount to make more content available.

### systemd

```bash
sudo cp ztutord /usr/local/bin/ztutord
sudo useradd -r -s /bin/false ztutor
sudo mkdir -p /var/lib/ztutor /opt/ztutor/courses
sudo chown ztutor:ztutor /var/lib/ztutor
sudo cp -r courses/ /opt/ztutor/courses/
sudo cp ztutor.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ztutor
```

The default SSH port is 2222. To use port 22, set `"addr": ":22"` in `ztutor.json` or use a reverse proxy.

### Windows

The client (`ztutor.exe`) and server (`ztutord.exe`) build and run natively on Windows. Compile, run, tests, remote execution, the interactive mode, and the in-TUI debugger all work. Because Windows has no Linux namespace isolation, the sandbox falls back to per-process resource limits (none on Windows) — the same degraded posture as macOS.

Requirements for running student code on a Windows host:

- **C/C++** — MinGW-w64 (`gcc`, `g++`), e.g. `choco install mingw`
- **Python** — Python 3 on `PATH`
- **Rust** — rustup + the stable toolchain
- **Go** — the Go toolchain
- **Ruby / Java** — ruby, a JRE
- **Debugger** — GDB (MinGW-w64 provides `gdb.exe`) for C/C++; dlv for Go

Run the server from a terminal:

```powershell
.\ztutord.exe -local      # local dashboard in this terminal + SSH server
```

Run it as a service:

```powershell
sc.exe create ztutor binPath= "C:\ztutor\ztutord.exe" start= auto
sc.exe start ztutor
```

Student code and lesson content are written to the directory the server runs in; point `ztutor.json`'s `courses_dir` and `db.path` somewhere writable.

## Configuration

Create `ztutor.json` next to the binary:

```json
{
  "keymap": "default",
  "ssh": {
    "addr": ":2222",
    "host_key": "ztutor_host_key"
  },
  "db": {
    "path": "ztutor.db"
  },
  "courses_dir": "./courses"
}
```

Environment variables:

| Variable | Description |
|----------|-------------|
| `ZTUTOR_DATA_DIR` | Base directory for the database and host key |
| `ZTUTOR_CONFIG` | Path to the config file |
| `ZTUTOR_NO_NAMESPACES=1` | Disable Linux namespace isolation (for environments that do not support it) |
| `ZTUTOR_EXEC_ADDR` | Client-side remote execution server address |
| `ZTUTOR_EXEC_TOKEN` | Shared token for remote execution requests |
| `ZTUTOR_EXEC_TLS=1` | Use TLS for client-side remote execution |
| `ZTUTOR_GAMEPAD=0` | Disable native local gamepad input |
| `ZTUTOR_GAMEPAD_DEVICE` | Force a specific Linux event device path |

## Courses

Courses live in the `courses/` directory. Each course has a `course.yaml` manifest and one subdirectory per section.

### Course manifest

```yaml
id: c-programming
title: C Programming
description: "Learn C from hello world to pointers."
language: c
order: 1
sections:
  - id: lessons
    title: Lessons
    type: exercises
    dir: lessons/
    toolchain:
      available_tools: [compile, debug, assembly, interactive, sanitizers]
  - id: interviews
    title: Interview Questions
    type: interviews
    dir: interviews/
    toolchain:
      available_tools: [compile, debug]
toolchain:
  source_extension: .c
  syntax_highlighting: c
```

### Section types

| Type | Purpose |
|------|---------|
| `exercises` | Progressive coding lessons |
| `interviews` | Technical interview practice |
| `quizzes` | Multiple-choice concept checks |
| `exams` | Timed assessments |
| `challenges` | Recurring coding challenges |

### Supported languages

| Language | ID | Debugger | Assembly view |
|----------|----|----------|---------------|
| C | `c` | gdb | yes |
| C++ | `cpp` | gdb | yes |
| Python | `python` | pdb3 | no |
| Rust | `rust` | rust-gdb | yes |
| Go | `go` | dlv | yes |
| Ruby | `ruby` | byebug | no |
| Java | `java` | jdb | no |

Adding a new programming language requires approximately 40 lines in `internal/sandbox/`. Adding a new display language requires one YAML file in `internal/i18n/locales/`.

## Lesson format

Each lesson is a directory under the section's `dir`:

```
lessons/
  01-hello-world/
    lesson.md       # YAML frontmatter + markdown content
    exercise.c      # starter code
    expected.txt    # expected program output
    stdin.txt       # optional stdin fed to the program
    hints.txt       # hints separated by ---
    trivia.txt      # trivia separated by ---
```

### lesson.md

```markdown
---
difficulty: intermediate
tags: [pointers, memory]
tutorial:
  - "First tutorial beat."
  - "Second tutorial beat."
references:
  - K&R 2nd ed., 5.1
  - man 3 malloc
---
# Pointers in C

Lesson content in markdown. Code blocks are syntax-highlighted.
```

All lessons are open to every user. The `premium` frontmatter key (from older courses) is parsed but ignored.

## Admin dashboard

The first connection creates the admin account. From the dashboard you can manage students, create and edit lessons (via a guided wizard), manage courses, and view student progress.

The admin dashboard is accessible two ways:

- **SSH:** `ssh adminname@yourhost -p 2222` (the server detects the admin role and opens the admin TUI instead of the student one)
- **Local terminal:** run `ztutord -local` to open the dashboard directly in the current terminal without needing SSH. Useful for single-machine setups.

The interface is fully localized (English, Spanish, Arabic, Chinese) and respects right-to-left layout when Arabic is selected.

## Security notes

The setup token printed at startup is valid for **24 hours** and is permanently locked after **5 failed attempts**. Restart `ztutord` to generate a new token. Once the first user account exists, the token is no longer accepted.

The database uses SQLite in WAL mode. All writes are automatically serialized using a 5-second busy-timeout retry, so concurrent SSH sessions never produce lock errors.

## Sandbox security

Student code runs under the following limits:

| Limit | Value |
|-------|-------|
| Run timeout | 5 seconds |
| Compile timeout | 10 seconds |
| Max file size (RLIMIT_FSIZE) | 8 MB |
| File descriptors (RLIMIT_NOFILE) | 64 |
| Core dumps (RLIMIT_CORE) | disabled |
| Namespace isolation (Linux only) | user, mount, network, PID |

Namespace isolation is enabled automatically when the host kernel supports it. Set `ZTUTOR_NO_NAMESPACES=1` to disable it for containers that do not allow unprivileged namespaces.

## Building

```bash
make build       # production binary with version info
make dev         # development mode (go run)
make test        # run all tests
make lint        # run gofmt check and go vet
make clean       # remove binary and database
```

Requires Go 1.25+ and a compiler for each language you want to support (gcc, g++, rustc, etc.).

## Architecture

```
ztutor/
  cmd/
    ztutor/          # main client binary (local mode)
    ztutord/         # SSH server binary
  internal/
    config/          # JSON config loading
    db/              # SQLite: users, progress, enrollments, challenges, settings
    i18n/            # localization (en, es, ar, zh)
    lesson/          # course and lesson manifest parsing
    remote/          # remote execution client and server (ztutord thin backend)
    sandbox/         # language abstraction, compilation, sandboxed execution
    ssh/             # SSH server, PTY handling, authentication
    tui/             # Bubble Tea screens
    version/         # build-time version info
  courses/           # course directories
```

## Technology

- Go 1.25
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) -- TUI framework
- [Chroma](https://github.com/alecthomas/chroma) -- syntax highlighting
- [Glamour](https://github.com/charmbracelet/glamour) -- terminal markdown rendering
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) -- pure-Go SQLite (no cgo required)
- golang.org/x/crypto/ssh -- SSH server
- Linux namespaces -- sandbox isolation (Linux only; macOS runs without namespace isolation)

## License

AGPL-3.0. See [LICENSE](LICENSE).
