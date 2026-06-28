---
difficulty: beginner
premium: false
tags: [scope, debugging, control-flow, linux, git, static, lifetime]
tutorial:
  - "In C, every pair of curly braces introduces a new block scope. Variables declared inside are invisible outside."
  - "When an inner block declares a variable with the same name as an outer one, the inner one shadows it completely."
  - "While the inner block runs, all reads and writes go to the inner variable. The outer is untouched."
  - "When the inner block exits, the inner variable is destroyed. Any changes to it are gone."
references:
  - "Linus Torvalds on -Wshadow (LKML, 2006) (https://lkml.org/lkml/2006/11/28/239): the original thread, including rebuttals, worth reading in full"
  - "Git Source: diff-delta.c (https://github.com/git/git/blob/master/diff-delta.c)"
  - "GCC Warning Options: -Wshadow, -Wshadow=local, -Wshadow=compatible-local (https://gcc.gnu.org/onlinedocs/gcc/Warning-Options.html)"
  - "MISRA C:2012, Rule 5.3: identifiers in inner scope shall not hide identifiers in outer scope"
  - "C99 Standard: 6.2.1 Scopes of identifiers: the formal rules for block, file, function, and prototype scope"
  - "SEI CERT C: DCL01-C: Do not reuse variable names in subscopes"
  - "C Gotchas, The Scope of Variables (https://www.embedded.com/c-gotchas/): practical embedded systems perspective on scope bugs"
  - "Static Local Variables in C: a pattern every C programmer should know (man 3 strtok_r for a real example of why they're dangerous)"
---
# Block Scope Masking: The Torvalds Scoping Rule

## The Four Kinds of Scope in C

C has four distinct scopes:

| Scope | Where declared | Lifetime |
|-------|---------------|----------|
| **Block scope** | Inside `{ }` | Until the closing `}` |
| **File scope** | Outside all functions | Entire program |
| **Function scope** | Labels (`goto` targets) | Entire function |
| **Prototype scope** | Parameter names in declarations | The declaration only |

Most day-to-day C involves block scope and file scope. Understanding them precisely prevents an entire category of silent bugs.

## How Block Scope Works

```c
int x = 10;           // file scope (or outer block scope)

{
    int x = 99;       // block scope: new variable, shadows outer x
    printf("%d\n", x); // 99
}                     // inner x is destroyed here

printf("%d\n", x);   // 10: outer x was never touched
```

The two `x` variables are entirely separate objects. They occupy different stack slots, or different memory regions entirely if one is file-scope. The name `x` refers to whichever declaration is innermost at each point in the code.

## C89 vs C99: The Loop Variable Change

In C89, variables could not be declared inside a `for` statement header. They had to be declared at the top of a block:

```c
/* C89 */
int i;
for (i = 0; i < 10; i++) { ... }
// i is still in scope here: it leaks out of the loop

/* C99 */
for (int i = 0; i < 10; i++) { ... }
// i only exists inside the for loop: gone after the closing }
```

The C89 behavior meant loop counters leaked into surrounding scope, enabling accidental re-use and misreading of stale values. C99 corrected this. Always compile with `-std=c99` or later and use `for (int i = ...)`: the variable is destroyed at the closing brace.

## Static Local Variables: Lifetime vs Scope

A variable can have block scope, meaning it is only visible inside the function, while simultaneously having program lifetime, meaning it persists for the entire execution:

```c
int counter(void) {
    static int count = 0;  // initialized once, persists between calls
    count++;
    return count;
}

printf("%d\n", counter()); // 1
printf("%d\n", counter()); // 2
printf("%d\n", counter()); // 3
```

`static` local variables are initialized once, before `main` runs, and are never destroyed. The name `count` is only visible inside `counter()`, satisfying block scope, but the storage resides in the data segment rather than the stack.

This pattern is appropriate for caches, call counters, and once-initialized resources. It is inappropriate for multithreaded code, because the single instance is shared across all threads without any locking, and for reentrant contexts. The standard library's `strtok()` uses this pattern internally, which is why it is not thread-safe. `strtok_r()` was created as the reentrant replacement.

## The Danger: Silent State Loss

The dangerous form of shadowing occurs when a new variable is declared where the intent was to update the outer one:

```c
int entry_index = 0;

for (int i = 0; i < count; i++) {
    if (entries[i].valid) {
        int entry_index = i; // BUG: creates a new variable, shadows the outer one
        entry_index++;       // updates the local copy, then throws it away
    }
}

// entry_index is still 0: all the work was lost
printf("Final index: %d\n", entry_index);
```

Without `-Wshadow`, no warning is issued. The outer `entry_index` is never assigned. The inner one is created, incremented, and silently destroyed on every iteration. The loop accomplishes nothing.

## The `-Wshadow` Flag and the Torvalds Debate

GCC's `-Wshadow` flag warns on any shadowing, including harmless cases such as:

```c
#include <string.h>  // declares index() as a POSIX function

void create_delta_index(void) {
    int index = 0;  // -Wshadow fires: shadows the global 'index()' function
    ...
}
```

In 2006, Linus Torvalds argued on the Linux Kernel Mailing List that this was too aggressive. Using `index` as a local variable does not conflict with the `index()` string function in any practical way, and he famously rejected enabling `-Wshadow` across the kernel.

GCC 7 introduced a more targeted variant: **`-Wshadow=local`**, which warns only when a local variable shadows another local variable in the same function. This catches the dangerous case, such as a loop counter shadowing an outer tracking variable, without firing on harmless global-name collisions. Modern projects tend to use `-Wshadow=local` rather than blanket `-Wshadow`.

## The Production Pattern

In Git's `diff-delta.c`, the delta-compression engine manages `entry_index` across nested loops and conditional blocks. A misplaced `int entry_index` inside an inner `if` block causes the outer tracking variable to stall, generating incorrect delta sizes.

The fix is always the same: remove the inner declaration and use the outer variable directly.

## The Mission

Open `exercise.c`. Due to a shadowed variable inside `if (entry_valid)`, the outer `entry_index` counter is never updated: the function always reports its initial value.

1. Find the variable declaration inside the `if (entry_valid)` block that shadows the outer `entry_index`.
2. Remove the inner `int` declaration so the `entry_index++` line operates on the outer variable.
3. Do not change the `printf` statements: the validator checks their output.
