---
difficulty: beginner
premium: false
tags: [io, security, strings, parsing, protocols, irc, ngircd, buffer-overflow]
tutorial:
  - "scanf(\"%s\", buffer) reads until whitespace with no length limit: it will overflow any fixed buffer given long enough input."
  - "gets() was so dangerous it was removed from C11 entirely. Never use it."
  - "fgets(buffer, size, stream) reads at most size-1 bytes and always null-terminates. Safe baseline for text input."
  - "fgets keeps the trailing newline. You must detect truncation (missing newline) and strip the newline manually."
references:
  - "ngIRCd Source: src/ngircd/conn.c (https://github.com/ngircd/ngircd/blob/master/src/ngircd/conn.c)"
  - "ngIRCd Source: src/ngircd/parse.c (https://github.com/ngircd/ngircd/blob/master/src/ngircd/parse.c)"
  - "RFC 2812: Internet Relay Chat Protocol (https://datatracker.ietf.org/doc/html/rfc2812)"
  - "man 3 fgets"
  - "man 3 getline: POSIX reentrant alternative that allocates its own buffer"
  - "man 3 gets: read the BUGS section, then never use it"
  - "man 3 strtok_r: the reentrant tokenizer (strtok uses a hidden static variable)"
  - "Smashing The Stack For Fun And Profit, Aleph One, Phrack 49 (http://phrack.org/issues/49/14.html): the 1996 paper that made buffer overflows famous. Still readable."
  - "CWE-121: Stack-Based Buffer Overflow (https://cwe.mitre.org/data/definitions/121.html)"
  - "CVE-2023-38408 (OpenSSH): a 2023 remote code execution caused by a buffer handling bug in an authentication agent"
  - "OpenBSD strlcpy and strlcat, Todd Miller & Theo de Raadt (https://www.sudo.ws/posts/1998/10/strlcpy-and-strlcat-consistent-safe-string-copy-and-concatenation/): the original 1998 paper introducing the safe string functions"
  - "POSIX getline(): the correct modern answer to reading arbitrary-length lines (man 3 getline)"
  - "Format String Vulnerabilities, OWASP (https://owasp.org/www-community/attacks/Format_string_attack): when user input reaches printf's format argument"
  - "SEI CERT C: STR35-C: Do not copy data from an unbounded source to a fixed-length array"
---
# Standard I/O Traps: The ngIRCd Framing Architecture

## Why `scanf` and `gets` Are Dangerous

### The `scanf("%s", buffer)` trap

`scanf("%s", buf)` reads characters until it hits whitespace. It has no idea how large `buf` is. Supply 1000 characters into a 16-byte buffer and `scanf` writes all 1000, overflowing past the end of `buf`, overwriting adjacent stack variables, and potentially overwriting the saved return address.

The return address overwrite is the mechanism behind stack buffer overflow exploits: craft input that overwrites the return address with the address of attacker-controlled code, and when the function returns, execution jumps there. This is the vulnerability class that Aleph One described in "Smashing The Stack For Fun And Profit" in 1996, and it still appears in CVEs today.

The bounded form `scanf("%15s", buf)` limits input to 15 characters for a 16-byte buffer. This approach is error-prone, however, because the number in the format string is the buffer size minus one and must be updated manually whenever the buffer size changes.

### `gets()` is gone

`gets(buf)` reads until newline with no size limit whatsoever. There is no safe way to use it. It was deprecated in C99 and removed from the C11 standard. GCC emits a linker warning if you use it.

### Format string vulnerabilities

A separate but related trap: if user-controlled data ever reaches `printf`'s format argument, an attacker can read the stack and write to arbitrary memory addresses:

```c
char user_input[256];
fgets(user_input, sizeof(user_input), stdin);

printf(user_input);  // DANGER if input contains %s, %x, %n
// printf("hello") is fine
// printf("%x %x %x") reads stack values
// printf("%n") writes to a pointer on the stack
```

Always use `printf("%s", user_input)`. The format string must be a string literal and must never be supplied by the user.

## The `fgets()` Solution

`fgets(buffer, size, stream)` reads at most `size - 1` bytes, stops at a newline (keeping it), and always null-terminates. It is the safe baseline for line-oriented text input.

```c
char buffer[64];
fgets(buffer, sizeof(buffer), stdin);
```

Two behaviors require explicit handling.

**1. The trailing newline is included.** If the user types `hello` and presses Enter, `buffer` contains `"hello\n"`. You must strip it:

```c
char *nl = strchr(buffer, '\n');
if (nl) {
    *nl = '\0';
    if (nl > buffer && *(nl-1) == '\r') {
        *(nl-1) = '\0';  // strip \r\n (Windows / IRC line endings)
    }
}
```

**2. If the line is longer than `size-1` bytes, `fgets` truncates.** The truncated buffer has no newline, which is your signal:

```c
if (strchr(buffer, '\n') == NULL) {
    // line was too long: handle the error, drain remaining input
    int c;
    while ((c = getchar()) != '\n' && c != EOF); // discard rest of line
    return ERROR_LINE_TOO_LONG;
}
```

Without detecting truncation before passing input to a protocol parser, you produce malformed protocol frames silently.

## POSIX `getline()`: The Modern Answer

If you do not know the maximum line length in advance, POSIX `getline()` allocates memory automatically:

```c
char *line = NULL;
size_t cap = 0;
ssize_t len;

while ((len = getline(&line, &cap, stdin)) != -1) {
    // line is allocated by getline, reused on subsequent calls
    if (len > 0 && line[len-1] == '\n') line[len-1] = '\0';
    process(line);
}
free(line);
```

`getline` reallocates `line` as needed, handling arbitrarily long lines without overflow. The caller owns the buffer and must `free` it.

`getline` is not part of C11; it is a POSIX extension available on Linux, macOS, and most Unix systems. For portable code or on Windows, you must implement it yourself or use a library.

## `strtok_r`: Thread-Safe Tokenization

The standard `strtok()` function uses a hidden `static` variable to remember its position between calls, making it non-reentrant and unsafe in multithreaded code:

```c
// strtok is dangerous in multithreaded code or nested parsing
char *token = strtok(line, ",");  // sets a hidden global pointer
while (token) {
    // if anything here calls strtok on a different string, state is lost
    token = strtok(NULL, ",");
}
```

The POSIX replacement, `strtok_r()`, accepts an explicit state pointer:

```c
char *saveptr;
char *token = strtok_r(line, ",", &saveptr);  // state is in saveptr, not global
while (token) {
    token = strtok_r(NULL, ",", &saveptr);
}
```

Because `saveptr` holds the parse state rather than a hidden global, multiple independent tokenizations can be in flight simultaneously.

## The Production Pattern: ngIRCd

IRC (RFC 2812) is line-oriented with a hard 512-byte per-message limit, terminated by `\r\n`. The **ngIRCd** daemon uses a two-layer approach. The framing layer (`conn.c`) reads raw bytes with bounded I/O, locates `\r\n`, and checks the 512-byte limit; it never passes partial or truncated frames downstream. The parser layer (`parse.c`) receives only complete, validated frames and processes IRC command grammar.

The `ngircd_conn.h` in your workspace defines the connection buffer structure. The lesson uses a 16-byte buffer to make truncation easy to trigger.

## The Mission

Open `exercise.c`. Implement a resilient IRC command framing parser.

1. Add `#include "ngircd_conn.h"` at the top.
2. Inside `parse_client_stream`, use `fgets` to read from `stdin` into `conn->read_buffer`. Use `sizeof(conn->read_buffer)` as the size limit.
3. Detect truncation: if `strchr(conn->read_buffer, '\n')` returns NULL, print `ERROR: IRC line too long\n` and return.
4. Strip the trailing `\n`. If preceded by `\r`, strip that too.
5. If the result matches `JOIN #ztutor`, print `OK: joined channel\n`.
6. Otherwise, print `ERROR: unknown IRC command\n`.
