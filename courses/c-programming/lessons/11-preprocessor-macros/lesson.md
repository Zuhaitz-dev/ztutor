---
difficulty: intermediate
premium: false
tags: [preprocessor, macros, compilation, security, openssl, x-macros, variadic]
tutorial:
  - "The preprocessor runs before compilation. It performs text substitution: no types, no scoping, no runtime cost."
  - "#define creates a text replacement rule. Every occurrence of the name is replaced before the compiler sees anything."
  - "do { ... } while(0) wraps multi-statement macros so they behave like a single statement in if/else."
  - "__FILE__ and __LINE__ expand at compile time to the source filename and line number: no runtime lookup."
references:
  - "OpenSSL Source: crypto/include/internal/cryptlib.h (https://github.com/openssl/openssl/blob/master/crypto/include/internal/cryptlib.h)"
  - "GCC Preprocessor Manual (https://gcc.gnu.org/onlinedocs/cpp/): the complete reference"
  - "gcc -E flag: run only the preprocessor and print output (try: gcc -E yourfile.c | less)"
  - "X-Macros pattern, Wikipedia (https://en.wikipedia.org/wiki/X_macro): generate parallel data structures from a single list"
  - "Variadic macros (__VA_ARGS__): cppreference (https://en.cppreference.com/w/c/preprocessor/replace)"
  - "Token pasting (##) and stringification (#), GCC Preprocessor Manual (https://gcc.gnu.org/onlinedocs/cpp/Concatenation.html)"
  - "#pragma once vs include guards: discussion and portability notes (https://en.wikipedia.org/wiki/Pragma_once)"
  - "MISRA C:2012, Rules 20.x: macro usage restrictions and rationale"
  - "Boost Preprocessor Library (https://www.boost.org/doc/libs/release/libs/preprocessor/): extreme use of the C preprocessor (C++ but applicable)"
  - "The C Preprocessor Is Not a Macro Language: article on why C macros are not Lisp macros"
---
# Preprocessor Security: The OpenSSL Cryptlib Pattern

## What the Preprocessor Does

The C preprocessor is a text processor that runs before the compiler. When you compile a `.c` file, the build involves three stages: preprocessor, compiler, and linker. The preprocessor sees your source; the compiler never sees `#define`, `#include`, or `#ifdef`.

```bash
# See what the preprocessor produces:
gcc -E yourfile.c | less
```

This command expands every `#include`, `#define`, and conditional, showing you exactly what the compiler sees. Running this on any unfamiliar codebase is extremely useful for understanding what macros expand to.

## `#define`: Text Substitution

```c
#define MAX_SIZE 1024
#define PI       3.14159265358979
#define SQUARE(x) ((x) * (x))
```

After preprocessing, every occurrence of `MAX_SIZE` is replaced with `1024`. `SQUARE(a + 1)` becomes `((a + 1) * (a + 1))`. The compiler never knows these names existed.

Macros have no scope (they are visible from the point of definition to the end of the file or an `#undef`), no type (they are text), and no stack frame (they have zero runtime cost).

## Predefined Macros: Compile-Time Context

| Macro | Expands to | Example |
|-------|-----------|---------|
| `__FILE__` | Source filename, as string literal | `"src/main.c"` |
| `__LINE__` | Current line number, as integer | `42` |
| `__DATE__` | Compilation date | `"Jun 27 2026"` |
| `__TIME__` | Compilation time | `"10:23:41"` |
| `__func__` | Current function name (C99) | `"process_packet"` |

These are inserted at compile time with zero runtime cost. An assertion macro that embeds location information:

```c
#define ASSERT(expr) \
    do { \
        if (!(expr)) { \
            fprintf(stderr, "Assertion failed: %s\n  at %s:%d in %s()\n", \
                    #expr, __FILE__, __LINE__, __func__); \
            abort(); \
        } \
    } while(0)
```

The `#expr` syntax (**stringification**) converts the expression itself to a string: `ASSERT(x > 0)` prints `"Assertion failed: x > 0"`, the text of the condition rather than its value.

## The Dangling Else Problem and `do { } while(0)`

Without the `do-while` wrapper, multi-statement macros break in `if/else`:

```c
#define LOG_AND_DIE(msg) \
    fprintf(stderr, msg); \
    abort();

if (error)
    LOG_AND_DIE("fatal");
else
    continue;
```

After preprocessing:
```c
if (error)
    fprintf(stderr, "fatal");  // only this is in the if
    abort();                   // ALWAYS executes
else                           // ERROR: else without if
    continue;
```

`do { ... } while(0)` creates a single compound statement:

```c
#define LOG_AND_DIE(msg) \
    do { \
        fprintf(stderr, msg); \
        abort(); \
    } while(0)
```

Now `LOG_AND_DIE("fatal");` is a single statement. The semicolon terminates `while(0)`. The `do` body executes exactly once. This pattern appears in the Linux kernel, OpenSSL, GLib, and virtually every professional C codebase.

## Variadic Macros: Wrapping printf

```c
#define LOG(level, fmt, ...) \
    fprintf(stderr, "[%s] " fmt "\n", level, ##__VA_ARGS__)

LOG("ERROR", "connection refused: %s", strerror(errno));
// expands to:
// fprintf(stderr, "[ERROR] connection refused: %s\n", "ERROR", strerror(errno));
```

`...` in the macro parameter list matches any number of arguments. `__VA_ARGS__` pastes them in. The `##` before `__VA_ARGS__` suppresses a trailing comma when no variadic arguments are given, so `LOG("INFO", "started")` works without a syntax error.

## Token Pasting with `##`

```c
#define MAKE_HANDLER(name) \
    void handle_##name(void) { \
        printf("handling " #name "\n"); \
    }

MAKE_HANDLER(click)   // generates void handle_click(void) { ... }
MAKE_HANDLER(scroll)  // generates void handle_scroll(void) { ... }
```

`##` concatenates two tokens and `#` stringifies the argument. Together they enable **code generation** from a macro, which is useful for eliminating repetition when you have many similar functions, types, or table entries.

## The X-Macro Pattern: Generate Parallel Structures

The X-Macro pattern defines a list once and expands it differently in multiple places:

```c
// Define the list once
#define ERROR_CODES \
    X(ERR_NONE,    "no error") \
    X(ERR_TIMEOUT, "connection timed out") \
    X(ERR_REFUSED, "connection refused") \
    X(ERR_OOM,     "out of memory")

// Generate the enum
typedef enum {
    #define X(code, msg) code,
    ERROR_CODES
    #undef X
} ErrorCode;

// Generate the string table
static const char *error_strings[] = {
    #define X(code, msg) msg,
    ERROR_CODES
    #undef X
};
```

The list is defined once. Adding a new error code requires a single line in `ERROR_CODES`, after which the enum, the string table, and any other generated structures update automatically. Without X-Macros, adding a new code requires changes in three or more places.

This pattern is used in the Linux kernel, SQLite, and many embedded projects.

## Include Guards vs `#pragma once`

Every header file needs protection against multiple inclusion:

```c
/* Traditional include guard */
#ifndef MYHEADER_H
#define MYHEADER_H

/* ... header content ... */

#endif /* MYHEADER_H */
```

`#pragma once` is a simpler alternative supported by GCC, Clang, and MSVC:

```c
#pragma once

/* ... header content ... */
```

`#pragma once` is not part of the C standard, but it functions correctly on every platform you are likely to target. It avoids the naming collision problem where two files accidentally use the same guard macro name, and it is faster in some compilers. Most new projects use it; most existing codebases use traditional guards.

## The Mission

Open `exercise.c`. You are working on OpenSSL's cryptlib security layer.

1. Open `cryptlib.h` and implement `OSS_ASSERT(expr)`:
   - Use `do { ... } while(0)` to prevent the dangling else problem
   - When `NDEBUG` is not defined: if `expr` is false, print to stderr with `__FILE__` and `__LINE__`, then `abort()`
   - When `NDEBUG` is defined: use `(void)(expr)`: evaluate the expression but generate no code
2. In `exercise.c`, use `OSS_ASSERT` to validate an invariant before a cryptographic operation.
