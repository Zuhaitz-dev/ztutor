---
difficulty: advanced
premium: false
tags: [preprocessor, macros, linux, kernel, compiler-extensions, generic, static-assert]
tutorial:
  - "Macros substitute text, not values. If you pass x++ to a macro, the ++ executes once per occurrence in the expansion."
  - "MIN(a++, b) expands to ((a++) < (b) ? (a++) : (b)): a++ runs twice if a < b. Silent counter corruption."
  - "The fix: GCC Statement Expressions ({ ... }) with typeof() cache arguments into local temps before comparing."
  - "C11 _Generic provides type-generic expressions without GCC extensions and without multiple evaluation."
references:
  - "Linux Kernel Source: include/linux/minmax.h (https://github.com/torvalds/linux/blob/master/include/linux/minmax.h): read the comment at the bottom, the kernel documents the danger explicitly"
  - "GCC Statement Expressions (https://gcc.gnu.org/onlinedocs/gcc/Statement-Exprs.html)"
  - "GCC typeof operator (https://gcc.gnu.org/onlinedocs/gcc/Typeof.html)"
  - "C11 _Generic: cppreference (https://en.cppreference.com/w/c/language/generic): type-generic expressions in standard C"
  - "C11 static_assert: cppreference (https://en.cppreference.com/w/c/error/static_assert): compile-time assertions"
  - "CERT C: PRE31-C: Avoid side effects in arguments to unsafe macros"
  - "When to use inline functions instead of macros: SEI CERT C Coding Standard"
  - "Clang __attribute__((overloadable)): function overloading in C (https://clang.llvm.org/docs/AttributeReference.html#overloadable)"
  - "__attribute__((warn_unused_result)), GCC (https://gcc.gnu.org/onlinedocs/gcc/Common-Function-Attributes.html): force callers to check return values"
  - "K&R C, Section 4.11.2: Macros with arguments: the original warning about side effects, in the language's own reference"
---
# Macro Gotchas: The Double Evaluation Trap

## Why Macros Are Not Functions

Functions evaluate each argument once before entering the body. Macros substitute text, pasting each argument literally and potentially multiple times.

```c
#define SQUARE(x) x * x

int a = 3;
int r = SQUARE(a + 1);
// Expands to: a + 1 * a + 1
// Evaluated as: a + (1 * a) + 1 = 3 + 3 + 1 = 7: not 16
```

Parenthesizing fixes the arithmetic priority:

```c
#define SQUARE(x) ((x) * (x))
SQUARE(a + 1)  // ((a+1) * (a+1)) = 16 ✓
```

But parentheses cannot fix evaluation count.

## The Double Evaluation Bug

```c
#define MIN(a, b) ((a) < (b) ? (a) : (b))

int x = 3;
int result = MIN(x++, 5);
```

After expansion:

```c
int result = ((x++) < (5) ? (x++) : (5));
```

The ternary evaluates the condition first: `x++` executes, `x` becomes 4, and the condition is true (3 < 5). Then it evaluates the left branch: `x++` executes again, `x` becomes 5, and `result` receives 4.

The expected outcome is `result = 3` with `x = 4`. The actual outcome is `result = 4` with `x = 5`. One extra increment occurs silently, with no warning and no compiler error. The Linux kernel's `minmax.h` documents this explicitly:

> *"Use these carefully: no type checking, and uses the arguments multiple times. Use for obvious constants only."*

The bug is completely silent. It surfaces only as a miscounted retry, an off-by-one in a loop limit, or a corrupted sequence number.

## The Statement Expression Fix

GCC and Clang support **statement expressions**, a compound statement inside `({ ... })` that evaluates to its last expression:

```c
#define SAFE_MIN(x, y) ({        \
    typeof(x) _x = (x);         \
    typeof(y) _y = (y);         \
    _x < _y ? _x : _y;          \
})
```

`typeof(x)` (a GCC/Clang extension) expands to the type of `x` without evaluating `x`. The macro evaluates `x` once into `_x` and `y` once into `_y`, then compares the copies.

```c
int x = 3;
int result = SAFE_MIN(x++, 5);
// expands to:
// ({ typeof(x++) _x = (x++); typeof(5) _y = (5); _x < _y ? _x : _y; })
// x++ runs exactly once → _x = 3, x = 4
// result = 3, x = 4 ✓
```

This is the approach used by the kernel's `min()` and `max()` macros in `include/linux/minmax.h`.

## C11 `_Generic`: Type-Generic Without Extensions

C11 introduced `_Generic`, a compile-time type switch that selects an expression based on the type of a controlling expression:

```c
#define min(x, y) _Generic((x), \
    int:    min_int,             \
    long:   min_long,            \
    float:  min_float,           \
    double: min_double           \
)(x, y)
```

`_Generic((x), int: ..., float: ...)` evaluates to whichever branch matches the type of `x` at compile time. Combined with inline functions, this provides type-generic expressions with full type checking and exactly-once evaluation, all in standard C11 without any GCC extensions.

## `static_assert`: Compile-Time Assertions

C11 added `static_assert(expr, message)`, an assertion that fires at compile time if `expr` is false:

```c
#include <assert.h>

static_assert(sizeof(int) == 4, "int must be 32 bits on this platform");
static_assert(sizeof(void *) == 8, "pointers must be 64 bits: 32-bit not supported");
static_assert(CHAR_BIT == 8, "this code assumes 8-bit bytes");
```

The compiler emits an error with the message if the condition fails. There is no runtime overhead and no test required. `static_assert` catches platform assumptions at build time rather than at test time, and it is useful for ensuring struct sizes match wire formats, validating that fixed-width types have expected widths, and documenting platform requirements as machine-checked constraints.

## `__attribute__((warn_unused_result))`: Enforcing Error Checking

Functions that return error codes are frequently ignored by callers:

```c
int connect(int sockfd, const struct sockaddr *addr, socklen_t len);

// Callers frequently forget to check:
connect(sock, &addr, sizeof(addr));  // error ignored!
```

The `warn_unused_result` attribute instructs the compiler to warn if the return value is discarded:

```c
int connect(int sockfd, const struct sockaddr *addr, socklen_t len)
    __attribute__((warn_unused_result));

// Now:
connect(sock, &addr, sizeof(addr));       // warning: ignoring return value
int err = connect(sock, &addr, sizeof(addr));  // no warning
(void) connect(sock, &addr, sizeof(addr));     // explicit discard, no warning
```

The C standard library's `fwrite`, `fclose`, and similar functions arguably should carry this attribute. Many security vulnerabilities stem from unchecked return values.

## When to Use `static inline` Instead of Macros

If you are writing C99 or later and do not need type-genericity across incompatible types, a `static inline` function is almost always preferable to a macro:

```c
static inline int min_int(int a, int b) {
    return a < b ? a : b;
}
```

Inline functions evaluate arguments exactly once, provide full type checking so that passing a pointer where an integer is expected is a compile-time error, are debuggable (you can step into them in GDB), and produce identical or better machine code compared to a macro equivalent at `-O1` and above.

Macros remain appropriate when you genuinely need type-genericity that `_Generic` cannot cover, compile-time string manipulation via `#` and `##`, compatibility with C89 where `static inline` is not guaranteed, or the Linux kernel's specific case where statement expressions are available.

## The Mission

Open `exercise.c`. A network driver uses the unsafe `MIN` macro with `retries++`, causing the retry counter to advance twice per call and corrupting retry tracking.

1. Open `include/linux/minmax.h`. Implement `SAFE_MIN(x, y)` using a statement expression and `typeof()` to cache both arguments before comparing.
2. In `exercise.c`, replace `MIN(retries++, limit)` with `SAFE_MIN(retries++, limit)`.
3. The validator checks that `retries` incremented exactly once.
