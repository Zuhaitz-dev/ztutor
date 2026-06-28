---
difficulty: intermediate
premium: false
tags: [history, performance, control-flow, optimization, simd, compilers]
tutorial:
  - "Loop overhead (the branch instruction at the end of each iteration) can dominate execution time in tight inner loops."
  - "Loop unrolling duplicates the loop body N times to reduce branch frequency, but creates a 'leftover' partial iteration problem."
  - "Duff's Device solves this using switch statement fallthrough to jump into the middle of a do-while loop."
  - "Modern compilers at -O2 or higher will auto-unroll and auto-vectorize loops, so Duff's Device is a teaching artifact, not modern practice."
references:
  - "Tom Duff's Original 1984 net.lang.c Usenet post (https://groups.google.com/g/net.lang.c/c/3KFq-67DzdQ): read the thread, including the replies from Dennis Ritchie and others"
  - "Duff's Device, Wikipedia (https://en.wikipedia.org/wiki/Duff%27s_device): thorough walkthrough with assembly output"
  - "Coroutines in C, Simon Tatham (https://www.chiark.greenend.org.uk/~sgtatham/coroutines.html): uses Duff's Device to implement coroutines, one of the most creative C hacks ever written"
  - "GCC Loop Optimization Options: -funroll-loops, -fvectorize (https://gcc.gnu.org/onlinedocs/gcc/Optimize-Options.html)"
  - "Intel Intrinsics Guide (https://www.intel.com/content/www/us/en/docs/intrinsics-guide/index.html): the modern equivalent, SIMD does 16-64 bytes per instruction"
  - "Agner Fog's Optimization Manuals (https://www.agner.org/optimize/): the definitive source on x86 microarchitecture and what the CPU actually does"
  - "C23 [[fallthrough]] attribute: cppreference (https://en.cppreference.com/w/c/language/attributes/fallthrough): the standard way to document intentional fallthrough"
  - "What Every Programmer Should Know About Memory, Ulrich Drepper: chapter 6 covers prefetching and loop optimization"
  - "Loop Unrolling, Wikipedia (https://en.wikipedia.org/wiki/Loop_unrolling): the concept before the device"
---
# The "Disgusting" Device: Tom Duff's Lucasfilm Discovery

## Why Loop Overhead Matters

Every `for` or `while` loop generates overhead beyond the loop body itself. At the end of each iteration, the CPU must decrement the counter, compare it to the limit, and conditionally branch back to the top. On the Evans & Sutherland Picture System II that Tom Duff was programming at Lucasfilm in 1983, this branch-and-count sequence cost roughly as many clock cycles as the actual work being done, meaning half of total execution time was spent managing the loop rather than copying data.

## Standard Loop Unrolling

The classic fix is to manually repeat the loop body N times so that the branch-and-count overhead occurs N times less often:

```c
// 4x unrolled
while (count >= 4) {
    *dst++ = *src++;
    *dst++ = *src++;
    *dst++ = *src++;
    *dst++ = *src++;
    count -= 4;
}
// Leftover 0-3 elements
while (count--) {
    *dst++ = *src++;
}
```

This works, but it introduces two loops. If `count` starts at 7, the first loop runs once (4 elements) and the second loop runs 3 times. In assembly, you could jump into the middle of the unrolled block to handle the leftovers directly, but C has no `goto` into the middle of a loop. Or does it?

## The Device

Duff noticed that C's `switch` statement is essentially a computed goto: it jumps to whichever `case` label matches the expression. Crucially, the grammar places no restriction on where `case` labels may appear relative to control flow structures. He combined this observation with a `do-while` loop:

```c
send(to, from, count)
register short *to, *from;
register count;
{
    register n = (count + 7) / 8;
    switch (count % 8) {
    case 0: do { *to = *from++;
    case 7:      *to = *from++;
    case 6:      *to = *from++;
    case 5:      *to = *from++;
    case 4:      *to = *from++;
    case 3:      *to = *from++;
    case 2:      *to = *from++;
    case 1:      *to = *from++;
                 } while (--n > 0);
    }
}
```

Execution flow for `count = 10`:
- `n = (10 + 7) / 8 = 2`
- `count % 8 = 2`, so `switch` jumps to `case 2`
- Executes `case 2` and `case 1` (2 iterations) due to fallthrough
- `--n > 0` is true (n becomes 1), loops back to `case 0`
- Executes all 8 cases (8 iterations), total: 10 ✓
- `--n > 0` is false (n becomes 0), exits

Duff wrote: *"Disgusting, no? But it compiles and runs just fine on all known C compilers. Dennis Ritchie has endorsed it as legal C. I feel a combination of pride and revulsion at this discovery."*

## What It Teaches

The device illustrates several important principles of C and systems programming.

C's grammar is more permissive than it appears. Case labels are just jump targets, and the grammar does not constrain where they may appear relative to other control flow structures. Duff found the gap between what C allows and what anyone expected.

Fallthrough is a feature, not merely an oversight. The device functions only because switch cases fall through by default. The `break` keyword is the exception in switch semantics, not the rule.

Profiling should precede optimization. Duff profiled his code, identified the specific bottleneck (branch overhead in a tight copy loop), and applied a targeted fix. This is the correct sequence: measure first, then act on evidence.

## Coroutines via Duff's Device

In 2000, Simon Tatham published a creative application of the same principle: coroutines implemented using Duff's Device. The key insight is that the device is a state machine that resumes execution at a computed offset. By storing that offset in a struct and jumping to it on the next call, you obtain cooperative multitasking:

```c
int function(struct coroutine *coro) {
    switch (coro->state) {
    case 0:
        coro->state = 1;
        return value1;
    case 1:
        coro->state = 2;
        return value2;
    case 2:
        return DONE;
    }
}
```

Tatham's paper develops this further, creating a macro that hides the switch/case machinery entirely. It is well worth reading.

## Modern Compilers: You Rarely Need This

GCC and Clang at `-O2` or higher will unroll loops automatically when the iteration count is known or bounded. At `-O3` they will also auto-vectorize, converting the loop body to SIMD instructions that process 16, 32, or 64 bytes per instruction. The 64-bit register-to-register copy that Duff's Device handles in 8 copies per iteration, a modern SIMD instruction (`_mm256_store_si256`) handles in a single instruction operating on 256 bits (32 bytes) at once.

Duff's Device is a historical artifact that illustrates how hardware pipelines work and what compilers do on your behalf. In new code, let the compiler optimize loops and reach for SIMD only when profiling reveals a genuine bottleneck.

The `[[fallthrough]]` attribute introduced in C23 provides a standard way to document intentional fallthrough for both compilers and readers:

```c
switch (x) {
case 1:
    do_something();
    [[fallthrough]];  // I mean to fall through
case 2:
    do_more();
    break;
}
```

## The Mission

Open `exercise.c`. Implement Tom Duff's `send()` loop to stream shorts to a mock memory-mapped hardware register.

1. Locate the `switch` statement and `do-while` loop.
2. Fill in the missing `case` labels (`7` through `1`).
3. Every `case` executes: `*to = *from++;`
4. Do not add `break` statements: the fallthrough is essential.
5. Do not modify the validation `printf` statements.
