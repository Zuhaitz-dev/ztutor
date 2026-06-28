---
difficulty: beginner
premium: false
tags: [types, memory, stdint, serialization, sqlite, endianness, overflow]
tutorial:
  - "The size of 'int' and 'long' is not defined by the C standard: it depends on the compiler and target architecture."
  - "On 64-bit Linux 'long' is 8 bytes. On 64-bit Windows 'long' is 4 bytes. Same source, different memory layout."
  - "<stdint.h> provides types like uint32_t and int64_t whose sizes are guaranteed everywhere."
  - "Production binary formats always use fixed-width types. Never 'int' or 'long' for anything that touches disk or the network."
references:
  - "SQLite File Format Specification (https://www.sqlite.org/fileformat.html)"
  - "SQLite Source: src/sqliteInt.h (https://github.com/sqlite/sqlite/blob/master/src/sqliteInt.h)"
  - "C99 Standard: 7.20 Integer types <stdint.h>"
  - "C99 Standard: 7.8 Format conversion of integer types <inttypes.h>: the PRI macros"
  - "man 0p stdint.h"
  - "man 0p inttypes.h"
  - "man 0p limits.h: INT_MAX, UINT32_MAX, CHAR_BIT, and the rest"
  - "ILP64, LP64, LLP64 data models, Wikipedia (https://en.wikipedia.org/wiki/64-bit_computing#64-bit_data_models): why Windows and Linux disagree on 'long'"
  - "The Lost Art of C Structure Packing, Eric Raymond (http://www.catb.org/esr/structure-packing/): type widths, alignment, and padding in depth"
  - "What Every C Programmer Should Know About Undefined Behavior (Part 1), LLVM Blog (https://blog.llvm.org/2011/05/what-every-c-programmer-should-know.html): signed overflow and the compiler's license to eliminate your code"
  - "Catching Integer Overflows in C, David LeBlanc (https://static.googleusercontent.com/media/research.google.com/en//pubs/archive/37456.pdf)"
  - "Endianness, Wikipedia (https://en.wikipedia.org/wiki/Endianness): byte order, network byte order, and bi-endian hardware"
  - "Beej's Guide to C: stdint.h (https://beej.us/guide/bgc/html/#stdint): the most readable introduction to the topic"
  - "GCC Integer Overflow Builtins: __builtin_add_overflow, __builtin_mul_overflow (https://gcc.gnu.org/onlinedocs/gcc/Integer-Overflow-Builtins.html)"
---
# Strict Serialization Invariance: SQLite's Data Layout

## The Problem with Primitive Types

Here is a question: how many bytes does `long` occupy?

The honest answer is: it depends. The C standard specifies *minimums*, not exact sizes.

| Data model | Platform | `int` | `long` | `long long` | pointer |
|------------|----------|-------|--------|-------------|---------|
| LP64 | Linux, macOS 64-bit | 4 | **8** | 8 | 8 |
| LLP64 | Windows 64-bit | 4 | **4** | 8 | 8 |
| ILP32 | 32-bit systems | 4 | 4 | 8 | 4 |

The same C source file, compiled for Linux and Windows 64-bit, produces structs of different sizes. Write a `long` to disk on Linux, read it back on Windows: you are reading 8 bytes with a 4-byte type. The last 4 bytes silently corrupt the next field.

This is not theoretical. It has caused real data corruption in cross-platform software.

## What `<stdint.h>` Provides

The `<stdint.h>` header, standardized in C99, defines types with explicit, guaranteed widths:

| Type | Width | Signed? | Common use |
|------|-------|---------|------------|
| `uint8_t` | 8 bits | No | Bytes, chars, pixel values |
| `uint16_t` | 16 bits | No | Port numbers, small counters |
| `uint32_t` | 32 bits | No | File offsets, IPv4 addresses, CRC32 |
| `uint64_t` | 64 bits | No | File sizes, timestamps, hashes |
| `int8_t` | 8 bits | Yes | Small signed deltas |
| `int32_t` | 32 bits | Yes | Signed 32-bit arithmetic |
| `int64_t` | 64 bits | Yes | Signed 64-bit arithmetic |
| `uintptr_t` | pointer-wide | No | Storing addresses as integers |
| `size_t` | pointer-wide | No | Memory sizes, array indices |

You can verify sizes at compile time:

```c
#include <stdint.h>
#include <stdio.h>

printf("uint32_t: %zu bytes\n", sizeof(uint32_t));  // always 4
printf("uint64_t: %zu bytes\n", sizeof(uint64_t));  // always 8
printf("int:      %zu bytes\n", sizeof(int));        // platform-defined
printf("long:     %zu bytes\n", sizeof(long));       // platform-defined
```

## The Standard's Fundamental Unit: `char`

The C standard does not define storage in terms of bytes. It defines everything in terms of `char`. The `sizeof` operator returns units of `char`, `sizeof(char)` is 1 by definition, and every other size is relative to it. The number of bits per `char` is given by `CHAR_BIT`, declared in `<limits.h>`.

On every desktop, server, and mobile platform in common use, `CHAR_BIT` is 8. The standard does not require this. DSPs from Texas Instruments (the TMS320C series) use 16-bit `char`. Some older Cray machines used 32-bit `char`. On such platforms, `sizeof(int)` can equal 1: not because `int` is narrow, but because `char` is wide.

This has a direct consequence for `<stdint.h>`: `uint8_t` is **optional**. The standard defines it as an unsigned type with exactly 8 bits and no padding, but on a platform where `CHAR_BIT == 16`, no 8-bit type can exist. The larger types (`uint16_t`, `uint32_t`, `uint64_t`) are also technically optional but far more commonly available. When you write `static_assert(CHAR_BIT == 8, "this code requires 8-bit chars");`, you are documenting a real platform constraint rather than a pedantic one.

## Signed Overflow is Undefined Behavior

Signed integer overflow, that is, adding 1 to `INT_MAX` or multiplying two large `int` values, is **undefined behavior**. Not "wraps around." Not "implementation-defined." Undefined. The C standard declares the result unspecified, and compilers exploit this aggressively.

If the compiler can prove that a signed overflow would occur on a particular code path, it is permitted to assume that path is unreachable and eliminate it entirely. This has removed security-critical bounds checks in real software:

```c
// Compiler sees: if (len + extra < len): signed overflow is UB,
// so this can never be true. The check is deleted.
if (len + extra < len) {
    return ERROR_OVERFLOW;
}
memcpy(dst, src, len + extra);  // buffer overflow
```

The fix is `<limits.h>` and pre-check arithmetic:

```c
#include <limits.h>

if (extra > INT_MAX - len) {
    return ERROR_OVERFLOW;
}
```

Or GCC/Clang's checked arithmetic builtins:

```c
int result;
if (__builtin_add_overflow(len, extra, &result)) {
    return ERROR_OVERFLOW;
}
```

Unsigned overflow, by contrast, is fully defined: it wraps modulo 2ⁿ. `UINT32_MAX + 1 == 0` is guaranteed by the standard. This is why many systems programming contexts prefer unsigned arithmetic for sizes and offsets.

## The `PRI` Format Macros: You're Probably Doing This Wrong

`printf("%d", my_uint64_t_value)` is undefined behavior on most platforms. `%d` expects an `int`; passing a `uint64_t` is a type mismatch that reads the wrong number of bytes on any platform where `int` is 32 bits, which is everywhere in practice.

`<inttypes.h>` provides format macros for every fixed-width type:

```c
#include <inttypes.h>

uint64_t val = 18446744073709551615ULL;
printf("%" PRIu64 "\n", val);   // correct on every platform

int64_t signed_val = -1;
printf("%" PRId64 "\n", signed_val);

uint32_t small = 42;
printf("%" PRIu32 "\n", small);
```

The macros expand to platform-specific format strings. On Linux 64-bit, `PRIu64` expands to `"lu"`. On Windows, it expands to `"llu"`. Your source stays portable.

| Type | Print macro | Scan macro |
|------|-------------|------------|
| `uint32_t` | `PRIu32` | `SCNu32` |
| `uint64_t` | `PRIu64` | `SCNu64` |
| `int64_t` | `PRId64` | `SCNd64` |
| `uintptr_t` | `PRIuPTR` |: |

## Endianness: Byte Order in Binary Formats

A `uint32_t` with value `0x01020304` occupies 4 bytes in memory. The question is which byte comes first. On little-endian systems (x86, ARM default), the layout is `04 03 02 01`, with the least significant byte first. On big-endian systems (SPARC, network byte order), the layout is `01 02 03 04`, with the most significant byte first.

SQLite chose big-endian for its file format, regardless of the host architecture. This means an x86 machine must byte-swap multi-byte fields when reading or writing the file.

The POSIX network functions handle the common case:

```c
#include <arpa/inet.h>

uint32_t host_val = 0x01020304;
uint32_t network_val = htonl(host_val);  // host to network long (big-endian)
uint32_t back = ntohl(network_val);      // network to host long
```

For reading a `uint32_t` from an arbitrary byte buffer without alignment assumptions:

```c
// WRONG: undefined behavior if buf is not 4-byte aligned
uint32_t val = *(uint32_t *)buf;

// CORRECT: safe regardless of alignment
uint32_t val;
memcpy(&val, buf, sizeof(val));
val = ntohl(val);  // convert from big-endian if needed
```

The `memcpy` approach is not merely pedantic: on ARM, an unaligned pointer dereference can generate a hardware fault.

## Integer Promotion: The `uint8_t` Arithmetic Surprise

C automatically promotes smaller integer types to `int` before arithmetic. This produces genuinely surprising behavior:

```c
uint8_t a = 200;
uint8_t b = 100;

// a + b is computed as int (300), then compared: works correctly
if (a + b > 255) {
    printf("overflow would occur\n");  // prints this
}

// a + b is computed as int (300), then truncated to uint8_t (44)
uint8_t result = a + b;
printf("%d\n", result);  // prints 44, not 44... or an error
```

The addition happens in `int` space with no overflow. The truncation happens on assignment. Most engineers expect the addition to saturate or wrap at 8 bits; it does not.

This is also why `~` (bitwise NOT) on a `uint8_t` produces an `int`, not a `uint8_t`:

```c
uint8_t x = 0xFF;
// ~x is ~0x000000FF = 0xFFFFFF00 as int
// printf with %hhu would print 0, not 0xFF
```

Understanding promotion rules explains an entire category of confusing results in embedded and network code.

## The Production Pattern: SQLite

SQLite must create files that are **byte-identical** across every platform. A `.sqlite` file created on an iPhone must open on a 1990s SPARC workstation. The SQLite file format specification defines every field at the byte level.

To enforce this, SQLite aliases all internal types in `src/sqliteInt.h`:

```c
typedef uint8_t   u8;
typedef uint16_t  u16;
typedef uint32_t  u32;
typedef uint64_t  u64;
```

Every single binary field uses these aliases. The struct's memory layout is deterministic. You can open the file, seek to byte 24, read 4 bytes, byte-swap if needed, and know exactly what you have, on any platform, indefinitely.

A copy of `sqliteInt.h` is in your workspace.

## The Mission

Open `exercise.c`. A previous developer used C primitive types to parse a mock SQLite header byte array. The wrong type widths cause the `sizeof()` validation checks to fail.

1. Add `#include "sqliteInt.h"` at the top of `exercise.c`.
2. Inside `parse_sqlite_header`, fix the variable declarations:
   - `write_version` (offset 18) → `u8` (8-bit unsigned)
   - `reserved_space` (offset 20) → `u16` (16-bit unsigned)
   - `file_change_counter` (offset 24) → `u32` (32-bit unsigned)
3. Keep the extraction macros and printf statements intact: the validator checks `sizeof()` on your variables to confirm correct widths.
