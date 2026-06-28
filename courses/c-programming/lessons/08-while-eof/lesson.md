---
difficulty: beginner
premium: false
tags: [io, streams, eof, posix, coreutils, buffering]
tutorial:
  - "getchar() returns int, not char. EOF is -1, which doesn't fit in a char on most platforms: it gets confused with 0xFF."
  - "The correct pattern: int c; while ((c = getchar()) != EOF) { ... }"
  - "The assignment happens inside the while condition. The character is stored AND checked against EOF in one expression."
  - "feof() checks a flag AFTER a read fails: don't loop on feof(), loop on the return value of the read function."
references:
  - "GNU Coreutils Source: wc.c (https://github.com/coreutils/coreutils/blob/master/src/wc.c)"
  - "Unix V7 Source: usr/src/cmd/wc.c (https://minnie.tuhs.org/cgi-bin/utree.pl?file=V7/usr/src/cmd/wc.c): the 1979 original, 50 lines of C"
  - "The C Programming Language, 2nd ed., Kernighan & Ritchie, Section 1.5: character input/output (the canonical EOF loop, written by the language's own authors)"
  - "man 3 getchar"
  - "man 3 feof: and why you almost never need it"
  - "man 3 ungetc: push one character back into a stream"
  - "man 3 setvbuf: control buffering mode (unbuffered, line-buffered, fully-buffered)"
  - "man 3 fflush: force a buffer flush"
  - "Lions' Commentary on Unix 6th Edition, John Lions (https://en.wikipedia.org/wiki/Lions%27_Commentary_on_UNIX_6th_Edition): the legendary bootleg commentary on Unix source code"
  - "Advanced Programming in the Unix Environment: Stevens & Rago, Chapter 5: Standard I/O Library: covers buffering, setvbuf, fflush in depth"
---
# Stream Processing: The GNU `wc` Pattern

## The `int c` Requirement: Why Not `char`?

This is one of the most common beginner mistakes in C:

```c
char c;  // WRONG
while ((c = getchar()) != EOF) {
    count++;
}
```

The problem is type width. `getchar()` returns an `int`, with a full range of `0` through `255` (valid characters) plus the special value `EOF`, which is defined as `-1`.

On most platforms, `char` is **signed** and 8 bits wide. Its range is `-128` to `127`. When you assign the unsigned byte value `255` (0xFF) to a signed `char`, it gets reinterpreted as `-1`, which is the same bit pattern as `EOF`.

If the input stream contains the byte `0xFF`, which is valid in binary files, ISO-8859 encoded text, UTF-8 multi-byte sequences, and image data, the loop terminates prematurely. The `0xFF` byte is mistaken for end-of-file.

The fix is to use `int`:

```c
int c;  // CORRECT: can hold 0–255 AND -1 without collision
while ((c = getchar()) != EOF) {
    count++;
}
```

`int` is typically 32 bits, so all 256 byte values and the -1 sentinel are distinct.

## The Assignment-in-Condition Pattern

```c
while ((c = getchar()) != EOF) { ... }
```

The inner parentheses are required. Without them, operator precedence changes the meaning:

```c
// WITHOUT inner parens:
while (c = (getchar() != EOF)) { ... }
// assigns 1 (true) or 0 (false) to c: the character is lost
```

With the parentheses: `getchar()` is called, the result is stored in `c`, then `c` is compared to `EOF`. The character is preserved in `c` for use inside the loop body.

GCC and Clang will warn about `if (c = expr)` without parens, assuming it is a mistaken `==`. The inner parens suppress this warning and signal intentionality.

## The Double-Consume Trap

```c
while (getchar() != EOF) {
    putchar(getchar());  // BUG: reads and discards every other character
}
```

Each `getchar()` call consumes one character from the stream. The first call, in the condition, reads character N and discards it. The second call, in the body, reads character N+1 and processes it. Characters at even positions are silently dropped.

The assignment-in-condition pattern prevents this: `getchar()` is called exactly once per loop iteration.

## `feof()` and the Wrong-Loop Trap

`feof(stream)` returns nonzero if the end-of-file flag has been set on `stream`. It is tempting to write:

```c
while (!feof(stdin)) {           // WRONG
    int c = getchar();
    count++;
}
```

This loop runs one extra iteration. After `getchar()` returns `EOF`, the end-of-file flag is set, but the check happens at the top of the loop before the next `getchar()`. The body runs once more with `c == EOF`, incorrectly incrementing the count.

`feof()` is the right tool for diagnosing why a read failed after the loop exits:

```c
while ((c = getchar()) != EOF) {
    count++;
}

if (feof(stdin)) {
    // Normal end-of-file
} else {
    // ferror(stdin) is nonzero: an I/O error occurred
    perror("stdin");
}
```

## `ungetc()`: Peeking One Character Ahead

Parsers often need to examine the next character to decide what to do, then put it back if they do not consume it:

```c
int peek = getchar();

if (isdigit(peek)) {
    ungetc(peek, stdin);   // push it back
    parse_number();        // which will read it again
} else {
    ungetc(peek, stdin);
    parse_identifier();
}
```

`ungetc(c, stream)` pushes exactly one character back into the stream's buffer. The next `getchar()` or `fread()` reads it back. Only one character of pushback is guaranteed by the C standard.

## Stream Buffering: `setvbuf` and `fflush`

The C runtime uses three buffering modes:

| Mode | Constant | When output is written |
|------|----------|----------------------|
| Unbuffered | `_IONBF` | Immediately, byte by byte |
| Line-buffered | `_IOLBF` | When a `\n` is written or the buffer fills |
| Fully-buffered | `_IOFBF` | When the buffer fills (default for files) |

`stdout` is line-buffered when connected to a terminal, and fully-buffered when connected to a pipe or file. `stderr` is unbuffered.

You can change this behavior with `setvbuf()`:

```c
// Make stdout unbuffered: every printf goes out immediately
setvbuf(stdout, NULL, _IONBF, 0);

// Set a custom 8KB buffer for a file stream
setvbuf(fp, NULL, _IOFBF, 8192);
```

`fflush(stream)` forces any buffered output out immediately:

```c
printf("Starting...");
fflush(stdout);   // ensures the message appears before a slow operation
do_slow_thing();
```

This matters in programs that print progress messages without trailing newlines. Without `fflush`, the output sits in the buffer and appears all at once after the slow operation completes.

## The `wc` Architecture

The original Unix `wc`, written in 1979 in roughly 50 lines of C, counts characters, words, and lines in a stream using the `getchar()` loop. The V7 source (linked in references) is worth reading: it is one of the smallest complete Unix utilities.

The modern GNU `wc` handles multi-byte UTF-8 characters, POSIX locale settings, and multiple files, but the core loop structure is the same.

## The Mission

Open `exercise.c`. Implement a minimal stream character counter.

1. Declare `int c;` (not `char`).
2. Write `while ((c = getchar()) != EOF)`: assignment inside the condition.
3. Inside the loop, increment a counter. Do not check `c != EOF` inside the loop: the while condition already handles it.
4. After the loop, print the count: `%d\n`.
