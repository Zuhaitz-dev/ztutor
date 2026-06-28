---
difficulty: beginner
premium: false
tags: [types, history, standards, neovim, vim, flags, short-circuit]
tutorial:
  - "C89 had no boolean type. Developers used int: 0 for false, anything else for true."
  - "The problem: error codes like -1 are truthy. A function returning -1 (error) silently passes an 'if (result)' check."
  - "C99 added <stdbool.h> with bool, true, and false. The underlying type is _Bool: a 1-byte unsigned integer."
  - "Short-circuit evaluation: in (a && b), if a is false, b is never evaluated. In (a || b), if a is true, b is never evaluated."
references:
  - "Vim Legacy Source: src/vim.h (https://github.com/vim/vim/blob/master/src/vim.h): search for 'define FALSE' to see the pre-stdbool pattern"
  - "Neovim Core Header: src/nvim/types_defs.h (https://github.com/neovim/neovim/blob/master/src/nvim/types_defs.h)"
  - "C99 Standard: Section 7.16: Boolean type and values <stdbool.h>"
  - "C11 Standard: 6.3.1.1, Boolean, characters, and integers (the promotion rules that affect bool)"
  - "The Boolean Trap, Andrzej Krzemieński (https://akrzemi1.wordpress.com/2011/05/11/the-boolean-trap/): why bool parameters in function APIs are a design smell"
  - "SEI CERT C: EXP46-C: Do not use a bitwise operator with a Boolean-like operand"
  - "Neovim's TriState pattern on GitHub: search 'TriState' in the Neovim repository for real-world usage across the codebase"
  - "Safe Boolean Idiom in C: a pattern for preventing bool/int confusion in older codebases"
  - "GLib gboolean: how the GNOME project handled booleans before C99 (https://docs.gtk.org/glib/alias.gboolean.html)"
---
# Primitive Truth vs. Strict Invariance: The Neovim Refactor

## The Original Problem

C was designed in the early 1970s without a boolean type. The convention was simple: `0` is false, anything else is true. This worked until error codes entered the picture.

```c
/* C89 idiom */
#define FALSE 0
#define TRUE  1

int find_option(const char *name);
// Returns: 1 if found, 0 if not found, -1 if storage error

int status = find_option("show_line_numbers");

if (status) {
    enable_line_numbers();  // BUG: runs when status == -1 (storage error)
}
```

Both `1` (found) and `-1` (error) are truthy, so the `if` cannot distinguish between them. With `int` booleans, the type system offers no protection: nothing in the function signature indicates whether the return value is a yes/no answer or a multi-value status code.

This class of bug is subtle because the code looks correct. The reviewer and the author both read `if (status)` as "if the option exists" without noticing that it also fires on error.

## What `<stdbool.h>` Provides

C99's `<stdbool.h>` defines three names: `bool`, which maps to `_Bool` (a 1-byte unsigned integer type built into C99); `true`, which expands to `1`; and `false`, which expands to `0`. Including the header lets you use the readable names throughout your code.

```c
#include <stdbool.h>

bool is_option_enabled(const char *name) {
    // Return type is now unambiguous: this is a yes/no function
    if (storage_error()) return false;
    return lookup(name) != NULL;
}

bool enabled = is_option_enabled("show_line_numbers");
if (enabled) {
    // This only runs when the option is genuinely enabled
}
```

When you assign an integer to `_Bool`, any nonzero value becomes `1` and `0` stays `0`. The truncation to a strict yes/no is explicit and enforced by the type. Note also that `sizeof(bool) == 1` always, while `sizeof(int) == 4` on most platforms. Returning `bool` instead of `int` is not merely a stylistic choice: it changes the ABI and the amount of data moved.

## Short-Circuit Evaluation

`&&` and `||` use short-circuit evaluation, stopping as soon as the result is determined:

```c
// If ptr is NULL, ptr->value is never evaluated: no crash
if (ptr != NULL && ptr->value > 0) { ... }

// If the cache hits, the expensive function is never called
if (cache_hit(key) || expensive_lookup(key)) { ... }
```

This behavior is guaranteed by the C standard. It is not an optimization; it is the defined semantics. Code that relies on it is correct C.

Short-circuit evaluation also governs side effects:

```c
int x = 0;
false && (x = 1);  // x = 1 is never evaluated; x remains 0
true  || (x = 2);  // x = 2 is never evaluated; x remains 0
```

Relying on side effects inside short-circuit expressions is legal but generally considered a code smell.

## Bitwise vs Logical Operators: A Common Mistake

`&` and `|` are bitwise operators. `&&` and `||` are logical operators. They are not interchangeable when working with boolean values:

```c
int a = 2;  // 0b0010
int b = 1;  // 0b0001

a && b  // 1: logical AND: both nonzero, result is true
a &  b  // 0: bitwise AND: 0b0010 & 0b0001 = 0b0000
```

`a & b` evaluates to zero even though both `a` and `b` are truthy. Using `&` instead of `&&` in a boolean context is a bug that GCC and Clang will not warn about by default. CERT C rule EXP46-C specifically addresses this pitfall.

## The TriState Pattern

Some domains have three genuine states: true, false, and unset or unknown. Vim embedded this concept as `-1` in a plain `int`, which reintroduced the original error-code confusion. When Neovim forked Vim, the project formalized the distinction with an explicit type:

```c
typedef enum {
  kNone  = -1,   // unset / unknown
  kFalse = 0,    // explicitly false
  kTrue  = 1,    // explicitly true
} TriState;
```

The type system now enforces the three-state contract. A `switch` on `TriState` that omits a case produces a compiler warning. An `if (status)` on a `TriState` compiles without a warning, but the type's name signals that `-1` is a valid, meaningful state and not an error sentinel.

The `types_defs.h` in your workspace contains the actual Neovim definitions.

## The Boolean Trap in API Design

A function signature such as:

```c
void create_user(const char *name, bool is_admin, bool is_active, bool send_email);
```

presents what is commonly called the "boolean trap." At the call site, the meaning of each argument is completely opaque:

```c
create_user("alice", true, false, true);
```

Positional boolean parameters are a readability hazard because the caller has no context for what each value means. The preferred pattern is a flags enum or a configuration struct:

```c
create_user("alice", USER_ADMIN | USER_SEND_WELCOME);
```

This is why well-designed C APIs, including GTK, OpenSSL, and POSIX itself, rarely expose boolean parameters. They use flags, enums, or distinct function names instead.

## The Mission

Open `exercise.c`. An option-querying routine currently uses raw `int` returns.

1. Add `#include "types_defs.h"` at the top.
2. Change the return type of `nvim_is_line_num_active` from `int` to `bool`.
3. Replace `return 1` with `return true` and `return 0` with `return false`.
4. Do not modify `main`: the validator uses `sizeof()` on the return value to confirm the type changed from `int` (4 bytes) to `bool` (1 byte).
