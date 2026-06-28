---
difficulty: intermediate
premium: false
tags: [pointers, api-design, curl, memory, state-mutation, const, restrict]
tutorial:
  - "C always passes by value: functions receive copies. Modifying a copy does not affect the original."
  - "Pass a pointer to let a function modify the caller's variable: swap(&a, &b) works, swap(a, b) does not."
  - "The opaque pointer pattern hides a struct's internals: callers hold a pointer to a type they cannot inspect."
  - "const T *p means the data is read-only through this pointer. T * const p means the pointer itself cannot be reassigned."
references:
  - "libcurl Source: include/curl/curl.h (https://github.com/curl/curl/blob/master/include/curl/curl.h)"
  - "libcurl Source: lib/setopt.c (https://github.com/curl/curl/blob/master/lib/setopt.c)"
  - "Everything You Need To Know About Pointers In C, Peter Hosey (https://boredzo.org/pointers/): the clearest visual treatment of pointer mechanics"
  - "Beej's Guide to C: Pointers (https://beej.us/guide/bgc/html/#pointers)"
  - "K&R C, 2nd edition, Section 5.2: Pointers and Function Arguments: the original explanation"
  - "C cdecl: http://cdecl.org: translate 'const char * const *argv' to English instantly"
  - "GLib's GObject: https://docs.gtk.org/gobject/: a large-scale C project built entirely on opaque pointers and reference counting"
  - "SQLite's handle pattern: sqlite3* (https://www.sqlite.org/cintro.html): another canonical opaque pointer API"
  - "C restrict keyword: cppreference (https://en.cppreference.com/w/c/language/restrict): tells the compiler two pointers don't alias, enables vectorization"
  - "Return Value Optimization (RVO): how compilers avoid copying structs on return (https://en.wikipedia.org/wiki/Copy_elision)"
---
# API Design: Opaque Handles and Variadic Mutation

## Pass-by-Value: Why `swap()` Fails

C always passes function arguments by value. The function receives a *copy*, and modifying that copy has no effect on the original.

```c
void swap(int a, int b) {
    int temp = a;
    a = b;
    b = temp;
    // a and b are stack-allocated copies: destroyed on return
}

int x = 5, y = 10;
swap(x, y);
printf("%d %d\n", x, y);  // 5 10: nothing changed
```

Stack layout during the call:
```
main's frame:   x=5    y=10
                ↓      ↓   (values copied into)
swap's frame:   a=5    b=10  temp=5
                             a←10, b←5  (swap happens here)
return:                       (frame destroyed, changes lost)
main's frame:   x=5    y=10  (unchanged)
```

To let a function modify the caller's variable, pass its **address**:

```c
void swap(int *a, int *b) {
    int temp = *a;
    *a = *b;      // write through the pointer to x in main's frame
    *b = temp;    // write through the pointer to y in main's frame
}

swap(&x, &y);    // pass addresses of x and y
printf("%d %d\n", x, y);  // 10 5: works
```

`&x` is the address of `x`. `*a` inside the function is "the value at address `a`", which is `x` itself.

## `const` and Pointer Qualifiers

`const` with pointers has two distinct meanings depending on position. The rule is to read the declaration right-to-left.

```c
const int *p;        // "p is a pointer to const int": data is read-only, pointer can change
int * const p;       // "p is a const pointer to int": pointer is fixed, data can change
const int * const p; // "p is a const pointer to const int": both fixed
```

Use `cdecl.org` to parse complex declarations. For function parameters:

```c
// I won't modify the data you point at: safe to pass a const pointer to me
size_t strlen(const char *s);

// I won't change which buffer you gave me, but I will write into it
void *memset(void *s, int c, size_t n);
```

`const` in function signatures is a contract. It enables the compiler to enforce read-only access and allows callers to pass `const` data safely.

## The `restrict` Keyword

`restrict` tells the compiler that two pointers in a function do not alias, meaning they do not point to overlapping memory. This allows the compiler to generate better code, particularly SIMD:

```c
// Without restrict: compiler must assume dst and src might overlap
// Every write to dst might change what src reads next
void copy(uint8_t *dst, uint8_t *src, size_t n);

// With restrict: compiler knows they don't overlap: can vectorize
void copy(uint8_t * restrict dst, const uint8_t * restrict src, size_t n);
```

The standard library's `memcpy` is declared with `restrict`. `memmove` is not, because it explicitly handles overlapping buffers. This is why `memcpy` is often faster than `memmove`: the compiler can emit wider SIMD copies.

## The Opaque Pointer Pattern: libcurl

`libcurl`'s `CURL` type is never defined in the public header. You cannot write `handle->url = "https://..."` because the struct's members are invisible to the caller:

```c
// Public header (curl.h):
typedef struct Curl_easy CURL;  // declared but not defined

CURL *handle = curl_easy_init();

// All state goes through functions:
curl_easy_setopt(handle, CURLOPT_URL, "https://example.com");
curl_easy_setopt(handle, CURLOPT_TIMEOUT, 30L);
curl_easy_perform(handle);
curl_easy_cleanup(handle);
```

This design provides several important guarantees. First, it ensures ABI stability: the library can restructure `Curl_easy` without breaking compiled applications. Second, it enforces information hiding, preventing callers from accidentally depending on internal fields. Third, it enforces a proper lifecycle, where `init` and `cleanup` are the only ways in and out.

This pattern appears throughout well-designed C APIs, including `sqlite3 *` in SQLite, `FILE *` in the standard library (where `FILE` is defined but its members are never accessed directly), `pthread_t` in pthreads, and `GObject *` in GTK.

The `curl_mock.h` in your workspace simulates this design.

## Struct Returns and RVO

Returning a struct by value appears as though it should be expensive:

```c
struct Point make_point(int x, int y) {
    struct Point p = {x, y};
    return p;
}

struct Point origin = make_point(0, 0);
```

In practice, modern compilers apply **Return Value Optimization (RVO)**: the caller allocates space for the return value and passes a hidden pointer to the function, which writes directly into that space. No copy occurs.

The ABI defines this behavior: on x86-64, structs larger than 16 bytes are returned via a hidden first argument. Smaller structs may be returned in registers.

You can verify this by comparing `-O0` and `-O2` assembly on godbolt.org. At `-O2`, the struct copy often disappears entirely.

## The Mission

Open `exercise.c`. `curl_easy_setopt` currently receives the `CURL` struct by value: changes are lost on return.

1. Add `#include "curl_mock.h"`.
2. Change the signature to take `CURL *handle` instead of `CURL handle`.
3. Use `handle->timeout` to update the timeout field through the pointer.
4. Verify the change persists after the function returns.
