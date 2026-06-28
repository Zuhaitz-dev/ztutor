# ztutor

An interactive programming tutor delivered over SSH. Students connect with a standard SSH client, read lessons, write code in a syntax-highlighted editor, and run it in a sandboxed environment -- all from the terminal.

Supports C, C++, Python, Rust, Go, Ruby, and Java out of the box. Adding a new programming language takes around 40 lines of Go. The interface ships in English, Spanish, Arabic, and Chinese, and adding a new display language is a single YAML file.

## How it works

1. The server administrator runs `ztutord` on a Linux machine.
2. Students connect over SSH -- no client software required.
3. Students read lessons, write code in the built-in editor, and submit for immediate feedback.
4. Code runs in a sandbox with resource limits and namespace isolation.

## Quick start

```bash
make build
./ztutord
```

On first run the server prints a setup token and the SSH address to connect to. The token is valid for 24 hours and locks after 5 failed attempts.

Connect in another terminal:

```bash
ssh yourname@localhost -p 2222
```

Paste the setup token when prompted to create the first account.

### Local admin dashboard

If you are running `ztutord` on the same machine you are sitting at, use the `-local` flag to open the admin dashboard directly in your terminal instead of SSHing in:

```bash
./ztutord -local
```

The SSH server still starts in the background so students can connect while you manage the server from the same terminal session. Press `q` or `Ctrl+C` in the admin dashboard to exit; the SSH server shuts down cleanly.

ztutor includes a small starter course with hello world lessons in multiple programming languages.
Full course content can still be distributed separately. If `courses/` is empty
or not mounted, the app starts and shows an empty course menu; add course
directories or `.course` packages to make lessons available.

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
# Set ZTUTOR_LICENSE_PUBKEY in .env
docker compose up -d
```

Students connect with: `ssh username@yourhost -p 2222`

The Docker service mounts `./courses` into the container as read-only course
content. A fresh checkout includes `courses/ztutor-starter`; keep that directory
or replace the mount with the course distribution path for your deployment.

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
  "courses_dir": "./courses",
  "license": {
    "file": "license.key"
  }
}
```

Environment variables:

| Variable | Description |
|----------|-------------|
| `ZTUTOR_DATA_DIR` | Base directory for the database, host key, and license file |
| `ZTUTOR_CONFIG` | Path to the config file |
| `ZTUTOR_LICENSE_PUBKEY` | Hex-encoded Ed25519 public key for license verification |
| `ZTUTOR_LICENSE_FILE` | License file path |
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
enrollment:
  required: false       # false = open access, true = license required
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
premium: true
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

Set `premium: true` to gate a lesson behind a license. Free users see the lesson title with a `[P]` badge. Courses with at least one premium lesson show `[freemium]` in the course list.

## Licensing and tiers

| Feature | Free | Premium | Business |
|---------|------|---------|----------|
| C Programming course | yes | yes | yes |
| Premium course content | no | yes | yes |
| License key | no | per-user | per-org |
| Encrypted course packages | no | yes | yes |
| Multi-user SSH server | no | no | yes |
| Admin dashboard | no | no | yes |
| Custom courses | no | no | yes |
| Private deployment | no | no | yes |

### Issue a license

```bash
go run ./cmd/licensegen/ \
  --key-file ztutor_keys.json \
  --licensee "Acme School" \
  --max-students 100 \
  --courses "c-programming,python-basics" \
  --features "multi_user,admin_ui,interviews" \
  --expires 365d
```

### License features

| Feature flag | Effect |
|--------------|--------|
| `multi_user` | Allows multiple student accounts |
| `admin_ui` | Enables the admin dashboard |
| `interviews` | Unlocks interview sections |

### Encrypted course packages

Premium courses are distributed as `.course` files: AES-256-GCM encrypted archives signed with the publisher's Ed25519 key. Place `.course` files in the `courses_dir`. If the license contains a valid course key, the course loads automatically. Without a valid key, the course appears in the menu as a preview with an `[encrypted]` badge and no lessons.

## Admin dashboard

Available when a license with `admin_ui` is active. The first connection creates the admin account. From the dashboard you can manage students, create and edit lessons (via a guided wizard), manage courses, and view student progress.

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
| Memory (RLIMIT_AS) | 128 MB |
| Max file size (RLIMIT_FSIZE) | 8 MB |
| File descriptors (RLIMIT_NOFILE) | 64 |
| Max processes (RLIMIT_NPROC) | 8 |
| Linux namespace isolation | user, mount, network, PID |

Namespace isolation is enabled automatically when the host kernel supports it. Set `ZTUTOR_NO_NAMESPACES=1` to disable it for containers that do not allow unprivileged namespaces.

## Building

```bash
make build       # production binary with version info
make dev         # development mode (go run)
make test        # run all tests
make lint        # run staticcheck
make clean       # remove binary and database
```

Requires Go 1.22+ and a compiler for each language you want to support (gcc, g++, rustc, etc.).

## Architecture

```
ztutor/
  cmd/
    ztutor/          # main client binary (local mode)
    ztutord/         # SSH server binary
    licensegen/      # license key generation tool
  internal/
    config/          # JSON config loading
    db/              # SQLite: users, progress, enrollments, challenges, settings
    i18n/            # localization (en, es, ar, zh)
    lesson/          # course and lesson manifest parsing
    license/         # Ed25519 license verification
    remote/          # remote execution client and server (ztutord thin backend)
    sandbox/         # language abstraction, compilation, sandboxed execution
    ssh/             # SSH server, PTY handling, authentication
    tui/             # Bubble Tea screens
    version/         # build-time version info
  courses/           # course directories
```

## Technology

- Go 1.22
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) -- TUI framework
- [Chroma](https://github.com/alecthomas/chroma) -- syntax highlighting
- [Glamour](https://github.com/charmbracelet/glamour) -- terminal markdown rendering
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) -- pure-Go SQLite (no cgo required)
- golang.org/x/crypto/ssh -- SSH server
- Ed25519 -- license key signing and course package signing
- Linux namespaces -- sandbox isolation

## License

AGPL-3.0 for open-source use. See [LICENSE](LICENSE).

Commercial licenses are available for organizations that need private deployments, proprietary modifications, or hosted service use without AGPL obligations. See [LICENSE-COMMERCIAL](LICENSE-COMMERCIAL) or contact zuhaitz.zechhub@gmail.com.
