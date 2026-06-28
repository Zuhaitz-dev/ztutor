---
difficulty: intermediate
premium: false
tags: [memory, assembly, gdb, stack, debugging, internals, security]
tutorial:
  - "The call stack grows downward. Each function call pushes a frame containing the return address and local variables."
  - "The return address is where execution resumes after the function returns. Overwriting it (stack buffer overflow) is a classic exploit."
  - "GDB's backtrace walks a linked list of frame pointers: each frame records where the previous frame started."
  - "Stack canaries (-fstack-protector) insert a random sentinel value between locals and the return address: checked on return."
references:
  - "GDB Source: gdb/frame.c (https://sourceware.org/git/gdb.git)"
  - "System V AMD64 ABI, Chapter 3 (https://refspecs.linuxfoundation.org/elf/x86-64-abi-0.99.pdf): the calling convention that defines how frames are laid out"
  - "DWARF Debugging Standard (https://dwarfstd.org/): how debuggers map machine addresses back to source lines"
  - "How Does the Stack Work?, Eli Bendersky (https://eli.thegreenplace.net/2011/09/06/stack-frame-layout-on-x86-64): the clearest explanation of x86-64 frame layout"
  - "Smashing The Stack For Fun And Profit, Aleph One, Phrack 49 (http://phrack.org/issues/49/14.html): the paper that made return address overwrites famous"
  - "GCC Stack Protection: -fstack-protector, -fstack-protector-strong, -fstack-protector-all (https://gcc.gnu.org/onlinedocs/gcc/Instrumentation-Options.html)"
  - "AddressSanitizer: A Fast Address Sanity Checker (https://www.usenix.org/legacy/events/atc12/tech/full_papers/Serebryany.pdf): the USENIX paper describing how ASAN works"
  - "GCC __builtin_return_address(n) documentation (https://gcc.gnu.org/onlinedocs/gcc/Return-Address.html): getting return addresses from C"
  - "Tail Call Optimization: what it is and when compilers do it (https://wiki.c2.com/?TailCallOptimization)"
  - "Beej's Guide to C: The Stack and the Heap (https://beej.us/guide/bgc/html/#the-stack)"
  - "alloca(): man 3 alloca, dynamic stack allocation and why it is dangerous"
---
# Architectural Unwinding: The GDB Frame Logic

## What the Call Stack Is

Every running thread has a **stack**: a contiguous region of virtual memory used to manage function calls. On x86-64 (and most architectures), the stack grows *downward*, meaning each new allocation occupies a lower address than the previous one.

The **stack pointer** (`RSP` on x86-64) points to the top of the current frame (the lowest used address). The **frame pointer** (`RBP`) points to the base of the current frame.

When you call a function:
1. The CPU pushes the **return address** (the instruction after the `call`) onto the stack
2. The function prologue pushes the caller's `RBP` (saves it)
3. Sets `RBP = RSP` (establishes this frame's base)
4. Decrements `RSP` by the size of local variables (allocates them)

```
High addresses (stack bottom: initial RSP at program start)
──────────────────────────────────────────────────────────
│  main()'s local variables                               │
│  saved RBP (→ OS frame)                                 │
│  return address (→ OS startup code)                     │  ← main's frame base (RBP when in main)
──────────────────────────────────────────────────────────
│  foo()'s local variables                                │
│  saved RBP (→ main's frame)                             │
│  return address (→ main)                                │  ← foo's frame base
──────────────────────────────────────────────────────────
│  bar()'s local variables                                │
│  saved RBP (→ foo's frame)                              │
│  return address (→ foo)                                 │  ← bar's frame base (current RBP)
──────────────────────────────────────────────────────────
Low addresses (stack grows this direction) ← current RSP
```

When `bar()` returns, the CPU reads the return address, restores `RBP` from the saved value, adjusts `RSP`, and jumps. The frame for `bar` is gone.

## Frame Pointer Unwinding: How GDB Does `bt`

Each frame's `RBP` contains the address of the previous frame's saved `RBP`. This forms a **linked list**:

```
current RBP → [saved RBP (prev frame base)] [return address]
               ↓
prev RBP → [saved RBP (prev-prev frame base)] [return address]
             ↓
...
```

GDB's `bt` command walks this list:
1. Read current `RBP`
2. At `RBP + 8`: the return address → look up source file/line via DWARF
3. At `RBP + 0`: the saved `RBP` → this is the previous frame's base
4. Repeat from step 2 with the previous frame's base

This is also how crash reporters, profilers, and the perf tool generate stack traces from a running process.

The `__builtin_return_address(n)` GCC intrinsic gives you the return address at depth `n` without a debugger:

```c
void *ra0 = __builtin_return_address(0);  // my direct caller
void *ra1 = __builtin_return_address(1);  // my caller's caller
```

## Stack Overflow

Each thread has a fixed stack size (typically 8 MB on Linux; check with `ulimit -s`). There is no automatic growth. Running off the bottom accesses unmapped memory:

```c
void recurse(int depth) {
    char big[1024 * 64];  // 64 KB per frame
    recurse(depth + 1);   // never returns
}
// After ~120 calls: SIGSEGV: stack exhausted
```

The remedy is to limit recursion depth, convert to iteration, or move large data to the heap. Variable-Length Arrays (`char buf[n]`) and `alloca()` are particularly dangerous because they allocate on the stack at runtime, and a large `n` can exhaust the stack in a single call.

## Stack Canaries: `-fstack-protector`

Buffer overflows that overwrite the return address are a classic exploit vector. Modern compilers insert a defense: a **stack canary**.

Between the local variables and the saved frame pointer/return address, the compiler inserts a secret random value (the canary) at function entry. Before returning, it verifies the canary is unchanged. If a buffer overflow overwrote memory between the locals and the return address, it almost certainly overwrote the canary.

```
│  buffer[128]    │  ← local variable
│  CANARY VALUE   │  ← secret, set on entry, checked on return
│  saved RBP      │
│  return address │
```

If the canary is wrong on return, the runtime calls `__stack_chk_fail()`, which prints an error and aborts. The attack is stopped before the malicious return address can execute.

The GCC and Clang flags for stack protection offer graduated coverage. `-fstack-protector` protects functions with string buffers or calls to `alloca`. `-fstack-protector-strong` extends coverage to all functions with local arrays. `-fstack-protector-all` applies protection to every function, providing maximum coverage at a small overhead cost.

This mechanism does not prevent the overflow itself; it prevents the exploit. The program still crashes, but it crashes predictably rather than executing attacker code.

## AddressSanitizer: Runtime Detection

`-fsanitize=address` (ASAN) instruments every memory access with a shadow memory check. It detects stack buffer overflows, heap buffer overflows, use after free, and use of uninitialized memory (with `-fsanitize=memory`).

The overhead is roughly 2x memory and 2x time, which is acceptable for development but not production. ASAN catches bugs that would otherwise be silent data corruption.

```bash
gcc -fsanitize=address -g prog.c -o prog
./prog
# If a buffer overflow occurs:
# ASAN: stack-buffer-overflow on address 0x7fff...
# READ of size 4 at ... thread T0
#     #0 0x... in vulnerable_function prog.c:12
```

## Tail Call Optimization

When the last thing a function does is call another function and return its result, the compiler can reuse the current frame instead of creating a new one:

```c
// Without TCO: each call grows the stack
int sum(int n, int acc) {
    if (n == 0) return acc;
    return sum(n - 1, acc + n);  // tail call: last operation
}
```

With optimization (`-O2`), GCC converts this recursion into a loop, reusing the frame. The call can proceed to depth 1,000,000 without overflowing. Without TCO, it would overflow around depth 8,000 to 10,000.

C does not guarantee tail call optimization, unlike Scheme or Haskell. You must verify the generated assembly or use `-foptimize-sibling-calls` explicitly. When stack depth matters, converting to iteration explicitly is more reliable than depending on TCO.

## The Mission

Open `exercise.c`. You are given `stack_memory`, a raw `uintptr_t` array representing a dump of stack memory.

1. Implement `unwind_stack`.
2. Walk the array in steps of 2. Each pair is `[saved_rbp, return_address]`.
3. Print each `return_address` found.
4. Stop when you encounter a `0` sentinel.
5. Do not modify the `printf` format in `main`.
