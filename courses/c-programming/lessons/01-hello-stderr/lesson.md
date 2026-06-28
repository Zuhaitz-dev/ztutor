---
difficulty: beginner
premium: false
tags: [io, streams, posix, multiplexing, git, dup2, isatty, syslog]
tutorial:
  - "Every POSIX process starts with three open file descriptors: stdin (0), stdout (1), stderr (2)."
  - "printf() is shorthand for fprintf(stdout, ...): it always writes to file descriptor 1."
  - "stdout is block-buffered: data sits in a buffer until a newline or flush. stderr is unbuffered: bytes go straight to the terminal."
  - "The pattern is: data payloads on stdout, diagnostics and progress on stderr. Mixing them corrupts pipelines."
references:
  - "Git Source: builtin/pack-objects.c (https://github.com/git/git/blob/master/builtin/pack-objects.c)"
  - "Git Source: progress.c (https://github.com/git/git/blob/master/progress.c)"
  - "POSIX.1-2017: 2.5.1 Interaction of File Descriptors and Standard I/O Streams"
  - "man 3 fprintf"
  - "man 3 isatty: detect whether a file descriptor is connected to a terminal"
  - "man 2 dup2: duplicate a file descriptor (the syscall behind shell redirection)"
  - "man 3 syslog / man 3 openlog: structured logging for daemons without a terminal"
  - "man 3 setvbuf: change a stream's buffering mode at runtime"
  - "GNU Coreutils Source: src/ls.c: search for isatty() to see where ls decides between columnar and flat output"
  - "Bash Manual: Redirections (https://www.gnu.org/software/bash/manual/bash.html#Redirections): the spec for left-to-right FD evaluation"
  - "Write a Shell in C, Stephen Brennan (https://brennan.io/2015/01/16/write-a-shell-in-c/): the clearest walkthrough of fork+dup2+exec that exists. Read this."
  - "xv6 Source: sh.c (https://github.com/mit-pdos/xv6-public/blob/master/sh.c): MIT's teaching OS shell, ~400 lines, no abstraction hiding anything"
  - "RFC 5424: The Syslog Protocol (https://tools.ietf.org/html/rfc5424): the wire format behind every syslog() call"
  - "The TTY Demystified, Linus Åkesson (https://www.linusakesson.net/programming/tty/): how file descriptors relate to terminal devices, line discipline, and PTYs"
  - "Advanced Programming in the Unix Environment: Stevens & Rago, Chapter 3: File I/O: dup, dup2, and the kernel FD table in detail"
---
# Multiplexing Streams: Git's Telemetry Architecture

Run `git clone` on a large repository and watch your terminal:

```
Cloning into 'linux'...
remote: Counting objects: 8392007, done.
Receiving objects: 100% (8392007/8392007), 3.21 GiB | 12.4 MiB/s, done.
```

Now redirect stdout to a file:

```bash
git clone https://github.com/torvalds/linux > /dev/null
```

The progress output still appears on your terminal. The binary data goes into `/dev/null`. This separation is not accidental; it is a deliberate architectural decision that every systems program must make.

## The Three Standard Streams

Every process inherits three open file descriptors from its parent:

| FD | Name | C identifier | Default destination |
|----|------|-------------|---------------------|
| 0 | Standard input | `stdin` | Keyboard |
| 1 | Standard output | `stdout` | Terminal |
| 2 | Standard error | `stderr` | Terminal |

Both `stdout` and `stderr` appear on the same terminal by default. The difference is in how they buffer data and what happens when you redirect them.

**`printf("hello\n")`** is identical to **`fprintf(stdout, "hello\n")`**: both write to FD 1.

**`fprintf(stderr, "error\n")`** writes to FD 2.

When a shell pipeline connects `git clone | gzip`, it wires the stdout of `git clone` (FD 1) into the stdin of `gzip` (FD 0). Stderr (FD 2) is left pointing at your terminal. Progress messages survive the pipe because they were never on stdout.

## The Buffering Difference

The distinction that most tutorials omit is buffering behavior. `stdout` is block-buffered, or line-buffered when connected to a terminal. The C runtime accumulates output in a memory buffer and flushes it in chunks. If your program crashes mid-execution, buffered output may be lost entirely, never reaching the terminal or the file.

`stderr` is unbuffered. Every `fprintf(stderr, ...)` call writes immediately to FD 2. If your program segfaults on the next line, the error message is already on screen.

```c
fprintf(stdout, "Processing file...\n"); // may never appear if program crashes
fprintf(stderr, "[DEBUG] entering process_file\n"); // guaranteed to appear
```

This is why crash diagnostics belong on stderr. You want to see every message that was emitted right up to the moment of failure.

## How Programs Detect They Are Being Piped: `isatty()`

Some programs change behavior depending on whether their output is going to a terminal or a pipe. The classic example is `ls`:

```bash
ls          # columnar output, colors
ls | cat    # flat output, one file per line, no colors
```

This is not a shell trick. `ls` calls `isatty(STDOUT_FILENO)`, a POSIX function that returns 1 if the file descriptor is connected to a terminal (a TTY), and 0 if it is connected to a pipe, file, or anything else.

```c
#include <unistd.h>

if (isatty(STDOUT_FILENO)) {
    // connected to a terminal: format for human reading
} else {
    // connected to a pipe or file: format for machine consumption
}
```

`grep --color=auto` does the same thing. `git log` does the same thing (pager vs no pager). Any program that functions correctly in both interactive and scripted contexts is using `isatty()`.

You can see the exact line in the GNU Coreutils source: `src/ls.c` contains `isatty (STDOUT_FILENO)` early in the initialization path. The result controls column formatting, color codes, and quoting style.

## The Double-Redirect Ordering Trap

The shell evaluates redirections **left to right**. These two commands look equivalent but are not:

```bash
./prog > out.txt 2>&1    # stderr goes into out.txt ✓
./prog 2>&1 > out.txt    # stderr still goes to the terminal ✗
```

In the second form, `2>&1` executes first: it duplicates FD 1 (which is currently the terminal) into FD 2. Then `> out.txt` redirects FD 1 to the file. At this point FD 2 is still pointing at the terminal, because it was redirected before FD 1 changed.

In the first form, `> out.txt` executes first: FD 1 now points to `out.txt`. Then `2>&1` duplicates FD 1 (now the file) into FD 2. Both streams end up in the file.

This is one of the most common shell mistakes made by experienced engineers. Understanding FD table manipulation makes it obvious: it is not magic, it is sequential `dup2()` calls.

## What `dup2()` Is: The Syscall Behind Redirection

When bash processes `> out.txt`, it does not set any environment variable or pass a flag to your program. It calls `dup2()`:

```c
int fd = open("out.txt", O_WRONLY | O_CREAT | O_TRUNC, 0644);
dup2(fd, STDOUT_FILENO);  // make FD 1 point to out.txt
close(fd);
exec("./prog", ...);       // run the program: it inherits the modified FD table
```

`dup2(oldfd, newfd)` makes `newfd` refer to the same underlying file description as `oldfd`. After this call, writing to FD 1 writes to `out.txt`. The program has no idea the redirection happened; it just calls `printf()` as usual.

This is also how shell pipes work:

```c
int pipefd[2];
pipe(pipefd);          // pipefd[0] = read end, pipefd[1] = write end

// In the child (producer):
dup2(pipefd[1], STDOUT_FILENO);  // stdout → pipe write end
exec("git clone", ...);

// In the other child (consumer):
dup2(pipefd[0], STDIN_FILENO);   // stdin → pipe read end
exec("gzip", ...);
```

Stephen Brennan's "Write a Shell in C" tutorial (in the references) builds this exact mechanism from scratch in a few hundred lines. It is one of the best short C projects available. The xv6 `sh.c` source does the same thing with no abstraction layer.

## Syslog: When There Is No Terminal

Production daemons, such as web servers, database engines, and system services, run without a terminal attached. Writing to stderr would go nowhere, or be discarded to `/dev/null`. They use `syslog` instead:

```c
#include <syslog.h>

openlog("myservice", LOG_PID | LOG_CONS, LOG_DAEMON);

syslog(LOG_ERR,  "failed to bind to port %d: %m", port);
syslog(LOG_INFO, "server started, pid %d", getpid());

closelog();
```

`syslog()` sends structured log entries to the system logging daemon (`syslogd` or `journald` on modern Linux). Entries include a priority level (`LOG_ERR`, `LOG_WARNING`, `LOG_INFO`, `LOG_DEBUG`) and a facility (`LOG_DAEMON`, `LOG_USER`, `LOG_AUTH`). The logging daemon routes them to files, the journal, or a remote log server based on configuration.

The `%m` format specifier is specific to `syslog`: it expands to the string representation of the current `errno` value, equivalent to `strerror(errno)`.

On modern systemd-based Linux, `journalctl -u myservice` shows the structured log entries. The structured metadata, including timestamp, PID, priority, and unit name, is stored separately from the message text. This is why `journalctl` can filter by severity, time range, or process without grep.

## The Production Pattern

Git's `progress.c` emits all human-facing progress output through `fprintf(stderr, ...)`. The binary packfile data goes to stdout. This separation makes `git pack-objects | gzip > repo.pack` work correctly: progress reaches the human, and compressed data reaches the file.

In this lesson you will build a simulation of this exact pattern.

## The Mission

Open `exercise.c`. Implement the telemetry loop inside `simulate_git_pack_objects`.

1. Write a `for` loop from index `1` to `total_objects` (inclusive).
2. For every object, write a debug trace to **`stderr`**: `[DEBUG] Scanning memory for object %d\n`
3. When the index is divisible by `progress_interval`, write a progress update to **`stderr`**: `[PROGRESS] Pack progress: %d%% completed\n`
4. After the loop, write the final payload signature to **`stdout`**: `SHA1_STREAM_BLOB: 0x7a2f5b9c\n`

Key notes:
- `fprintf(stderr, ...)` for FD 2, `printf(...)` or `fprintf(stdout, ...)` for FD 1
- `%%` in a format string prints a literal `%`
- The validator captures both streams separately: output on the wrong stream fails the test
