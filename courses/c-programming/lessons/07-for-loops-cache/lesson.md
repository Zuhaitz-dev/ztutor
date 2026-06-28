---
difficulty: intermediate
premium: false
tags: [performance, memory, cache, ffmpeg, optimization, alignment, prefetch, numa]
tutorial:
  - "CPUs fetch memory in 64-byte cache lines. Accessing sequential bytes is nearly free: the next line is prefetched automatically."
  - "Non-sequential access (column-major on a row-major array) causes a cache miss on every access: up to 100x slower."
  - "FFmpeg adds stride padding to each image row so every row starts at an aligned address. Use stride, not width, to advance rows."
  - "False sharing: two threads modifying different variables in the same cache line fight over that line, serializing what should be parallel."
references:
  - "FFmpeg Source: libavutil/imgutils.c (https://github.com/FFmpeg/FFmpeg/blob/master/libavutil/imgutils.c)"
  - "FFmpeg Source: libavutil/frame.h (https://github.com/FFmpeg/FFmpeg/blob/master/libavutil/frame.h)"
  - "What Every Programmer Should Know About Memory, Ulrich Drepper (https://people.freedesktop.org/~ajax/nm-commit-rant.html): the definitive 100-page treatment. Chapters 3-6 cover everything in this lesson."
  - "Gallery of Processor Cache Effects, Igor Ostrovsky (http://igoro.com/archive/gallery-of-processor-cache-effects/): eight experiments with measured timings. Read this."
  - "Producing Wrong Data Without Doing Anything Obviously Wrong, Mytkowicz et al. (https://dl.acm.org/doi/10.1145/1508244.1508275): measurement pitfalls in performance work"
  - "False Sharing, Intel (https://www.intel.com/content/www/us/en/developer/articles/technical/avoiding-and-identifying-false-sharing-among-threads.html)"
  - "Cache-Oblivious Algorithms, Frigo, Leiserson, Prokop, Ramachandran (https://jhu-dsst.github.io/pdfs/FrigoLePrRa99.pdf): algorithms that perform well at every cache level"
  - "GCC __builtin_prefetch documentation (https://gcc.gnu.org/onlinedocs/gcc/Other-Builtins.html): manual prefetch hints"
  - "perf stat and perf stat -e cache-misses: the Linux tool for measuring actual cache miss rates"
  - "Agner Fog's Optimization Manuals (https://www.agner.org/optimize/): microarchitecture details for every Intel/AMD generation"
---
# Memory Alignment: The FFmpeg Stride Architecture

## The Memory Hierarchy

Your CPU does not fetch bytes from RAM one at a time. It maintains a hierarchy of faster, smaller storage:

| Level | Typical latency | Typical size (per core) |
|-------|----------------|------------------------|
| L1 cache | 4 cycles | 32–64 KB |
| L2 cache | 12 cycles | 256 KB – 1 MB |
| L3 cache | 30–40 cycles | 4–64 MB (shared) |
| Main RAM | ~200 cycles | GBs |
| SSD (NVMe) | ~50,000 cycles | TBs |

The L1 cache is approximately 50x faster than RAM, and the SSD is roughly 250x slower than RAM.

The unit of transfer between RAM and cache is a **cache line**, typically 64 bytes on modern x86. When you access one byte, the CPU fetches the entire 64-byte line containing it. Bytes 0 through 63 of an array all land in the same cache line, so accessing any one of them loads all 64 into cache at no additional cost.

## Spatial Locality: Why Sequential Access Is Fast

```c
int arr[1024];
int sum = 0;

// Sequential access: each cache miss loads 16 ints (64 bytes / 4 bytes each)
// Only 64 cache misses total for 1024 elements
for (int i = 0; i < 1024; i++)
    sum += arr[i];

// Random access: every access is potentially a different cache line
// Up to 1024 cache misses for 1024 elements
int indices[1024] = { /* random */ };
for (int i = 0; i < 1024; i++)
    sum += arr[indices[i]];
```

Sequential access on a 1024-element array produces roughly 64 cache misses. Random access can produce up to 1024. At 200 cycles per miss, random access is approximately 32x slower on this input. For larger arrays the gap grows further because L2 and L3 caches also fill up.

## Row-Major vs Column-Major: The Transpose Benchmark

C stores 2D arrays in **row-major** order: all elements of row 0 first, then row 1, and so on. `matrix[r][c]` is at offset `r * cols + c` in memory.

```
matrix[0][0]  matrix[0][1]  matrix[0][2]  ← row 0 (contiguous in RAM)
matrix[1][0]  matrix[1][1]  matrix[1][2]  ← row 1
matrix[2][0]  matrix[2][1]  matrix[2][2]  ← row 2
```

Iterating row by row is cache-friendly because each step advances 4 bytes to the next element in the same cache line:

```c
for (int r = 0; r < N; r++)
    for (int c = 0; c < N; c++)
        sum += matrix[r][c];
```

Iterating column by column is cache-hostile because each step advances `cols * 4` bytes, landing on a different cache line that is very likely not yet cached:

```c
for (int c = 0; c < N; c++)    // outer loop on columns
    for (int r = 0; r < N; r++) // inner loop on rows
        sum += matrix[r][c];    // jumps N*4 bytes per step
```

For a 4096×4096 `int` matrix, the column-major version is typically 5 to 10 times slower. The code is structurally identical; only the loop order differs.

Matrix transposition is one of the first cache-optimization exercises in systems courses for precisely this reason. The naive transpose is cache-unfriendly in one direction, while a cache-oblivious blocked transpose maintains locality in both.

## The Stride Concept: FFmpeg

Video buffers introduce an additional consideration. FFmpeg allocates image rows **padded** to the next multiple of 16, 32, or 64 bytes so that every row starts at a SIMD-aligned address:

```
width = 1920 pixels = 1920 bytes (grayscale)
stride = 2048 bytes (next multiple of 64 after 1920 = 1984, rounded to 2048)

memory layout:
[row 0: 1920 pixels][128 bytes padding][row 1: 1920 pixels][128 bytes padding]...
```

To advance from row `r` to row `r+1`, you add `stride` bytes, not `width`:

```c
uint8_t *row = buffer;
for (int r = 0; r < height; r++) {
    for (int c = 0; c < width; c++) {
        row[c] = process(row[c]);
    }
    row += stride;  // skip the padding, land at the start of the next row
}
```

Using `width` instead of `stride` walks into the padding bytes, producing incorrect results and misaligning all subsequent row accesses.

The padding also provides the write alignment that SIMD instructions such as `_mm256_storeu_si256` and `_mm128_store_si128` require. Aligned stores are faster, and some SIMD instructions will fault on unaligned addresses.

## Manual Prefetching

When the access pattern is sequential but the CPU's hardware prefetcher is not fast enough, for example after a branch or a function call breaks the stream, you can issue explicit prefetch hints:

```c
// Prefetch for read, high temporal locality
for (int i = 0; i < N; i++) {
    __builtin_prefetch(&data[i + 16], 0, 3);  // fetch ahead 16 elements
    process(data[i]);
}
```

`__builtin_prefetch(addr, rw, locality)` accepts three arguments. The `rw` argument specifies the access type: 0 for read, 1 for write. The `locality` argument, ranging from 0 to 3, controls cache retention: 3 means keep the line in all cache levels, while 0 means single use and avoids polluting the cache for data that will not be reused.

This is a hint, not a guarantee. The compiler may ignore it. As with all optimizations, profiling should precede its use.

## False Sharing: The Multithreaded Cache Trap

Two variables that occupy the same cache line will conflict in multithreaded code even when they are logically independent:

```c
// BAD: counter[0] and counter[1] share a cache line
int counter[2] = {0, 0};

// Thread 0 modifies counter[0]
// Thread 1 modifies counter[1]
// The CPU treats the whole cache line as a unit: they fight over it
```

When Thread 0 writes `counter[0]`, the entire cache line is marked dirty in Thread 0's L1 cache, invalidating Thread 1's copy. Thread 1 must then fetch the line before writing `counter[1]`, which in turn invalidates Thread 0's copy. The two threads are serialized by the cache coherency protocol despite never logically sharing data. This is false sharing.

The standard fix is to pad each variable to occupy its own cache line:

```c
// GOOD: each counter occupies its own cache line
struct {
    int value;
    char pad[60];  // pad to 64 bytes total
} counter[2];
```

Alternatively, use `alignas(64)` from C11 to guarantee cache-line alignment without manual padding.

## The Mission

Open `exercise.c`. Implement a grayscale filter over a mock FFmpeg video frame.

1. Add `#include "ffmpeg_frame.h"`.
2. In `process_frame`, write a nested loop: outer over `height`, inner over `width`.
3. Index each pixel as `buffer[row * stride + col]`.
4. Apply: `buffer[row * stride + col] = buffer[row * stride + col] / 2`.
5. Do not modify the validation output.
