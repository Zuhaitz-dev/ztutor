---
difficulty: advanced
premium: false
tags: [capstone, parsing, networking, redis, pointers, state-machine, sds, event-loop, resp3, security]
tutorial:
  - "Network sockets deliver data in chunks, not complete messages. A parser must handle partial data arriving across multiple reads."
  - "An incremental state machine tracks its parse position in the buffer. Each call processes what's available and returns early if data is incomplete."
  - "RESP (Redis Serialization Protocol) encodes arrays as *N\\r\\n, followed by N bulk strings each as $len\\r\\ndata\\r\\n."
  - "The state machine pattern: receive bytes, advance a position pointer, update state, return on partial input: is used in HTTP servers, database drivers, and every network protocol parser."
references:
  - "RESP Protocol Specification (https://redis.io/docs/latest/develop/reference/protocol-spec/): RESP2 and RESP3 encoding rules"
  - "RESP3 Specification: antirez (https://github.com/antirez/RESP3/blob/master/spec.md): the new protocol with typed maps, sets, and errors"
  - "Redis Source: src/networking.c: processMultibulkBuffer (https://github.com/redis/redis/blob/unstable/src/networking.c): the actual function you're implementing"
  - "Redis Source: src/sds.c, Simple Dynamic Strings (https://github.com/redis/redis/blob/unstable/src/sds.c): the string library Redis uses internally"
  - "Redis Source: src/ae.c: the event loop (https://github.com/redis/redis/blob/unstable/src/ae.c): the aeEventLoop that drives all I/O"
  - "The Architecture of Open Source Applications, Volume 2, Redis chapter (https://aosabook.org/en/v2/redis.html): event loop architecture, client state machine, memory management"
  - "antirez's blog: http://antirez.com/latest/0: the Redis author's notes on design decisions"
  - "Beej's Guide to Network Programming (https://beej.us/guide/bgnet/): TCP socket mechanics needed before writing network parsers"
  - "HTTP/1.1 parsing in nginx: src/http/ngx_http_request.c: the same incremental pattern at larger scale"
  - "CWE-190: Integer Overflow in Length Parsing (https://cwe.mitre.org/data/definitions/190.html): why atoi() on protocol data is dangerous"
  - "Redis Security Guide (https://redis.io/docs/latest/operate/oss_and_stack/management/security/): bind, requirepass, protected-mode, and why you should not expose Redis to the internet"
  - "Writing Network Drivers in C: LWN.net: background on how kernel networking works below this level"
---
# Capstone: The Redis Multi-Bulk State Machine

You have reached the end of Module 1. Over the previous 14 lessons you learned stdout vs stderr, fixed-width types, variable scope, safe I/O, booleans, switch fallthrough, cache locality, stream EOF, the call stack, pass-by-value, macros and the preprocessor, conditional compilation, macro gotchas, and build automation.

Now you use all of it together. You are implementing a fragment of the actual Redis command parser.

## How Redis Receives Commands

When a client runs `redis-cli SET mykey myval`, it sends this over TCP:

```
*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$7\r\nmyval\r\n
```

This is the Redis Serialization Protocol (RESP), version 2. Breaking it down:

| Token | Meaning |
|-------|---------|
| `*3\r\n` | Array with 3 elements |
| `$3\r\n` | Bulk string, 3 bytes long |
| `SET\r\n` | The 3-byte string "SET" |
| `$5\r\n` | Bulk string, 5 bytes long |
| `mykey\r\n` | The 5-byte string "mykey" |
| `$7\r\n` | Bulk string, 7 bytes long |
| `myval\r\n` | The 7-byte string "myval" |

The array prefix `*3\r\n` is the **Multi-Bulk Header**. Every Redis command starts with it.

## The Incremental Parsing Problem

TCP does not deliver data in protocol-sized chunks. A 30-byte command might arrive as two separate packets: the first carrying `*3\r\n$3\r\nSE` and the second carrying the remainder `T\r\n$5\r\nmykey\r\n$7\r\nmyval\r\n`.

A parser that expects a complete message will fail on the first packet. A production server must handle this gracefully: process what arrived, save state, and wait for more data.

Redis uses a **client struct** (`client *c` in `src/networking.c`) to track incremental parse state:

```c
typedef struct client {
    char  *querybuf;     // raw bytes received so far
    int    qb_pos;       // offset of first unparsed byte
    int    multibulklen; // how many array elements remain to parse
    // ...
} client;
```

The function `processMultibulkBuffer` is called every time new bytes arrive. It checks whether `querybuf + qb_pos` starts with `*`, then looks for `\r` to locate the end of the integer. If `\r` is not yet present, the function returns `C_ERR` to signal that more data is needed. Otherwise it parses the integer between `*` and `\r`, stores the result in `c->multibulklen`, advances `c->qb_pos` past the `\r\n`, and returns `C_OK` to indicate readiness for the next parse phase.

This is the incremental state machine. Each invocation advances as far as the available data allows, then yields. The event loop calls it again when the next TCP packet arrives.

## After the Multi-Bulk Header: Parsing Bulk Strings

Once `multibulklen` is set, Redis enters a second loop to parse each bulk string element:

```
$3\r\n       ← bulk string header: 3 bytes follow
SET\r\n      ← the 3-byte string, then CRLF
$5\r\n
mykey\r\n
$7\r\n
myval\r\n
```

Each iteration reads the `$<len>` header, allocates an SDS string for the argument, reads `<len>` bytes into it, and decrements `multibulklen`. When `multibulklen` reaches 0, the command is complete and is dispatched to the command table.

The complete call path is: `readQueryFromClient()` calls `processInputBuffer()`, which calls `processMultibulkBuffer()` for the header phase and then the argument phase, which in turn calls `processCommand()`, then `lookupCommand()`, and finally the actual command handler.

## Redis's Event Loop: ae.c

Redis is single-threaded for command execution. Everything runs inside `ae.c`'s event loop:

```
aeMain()
  └── aeProcessEvents()
        ├── epoll_wait() / kqueue()  ← wait for I/O events
        ├── readQueryFromClient()    ← called when a client socket is readable
        ├── sendReplyToClient()      ← called when a client socket is writable
        └── processTimeEvents()      ← run expired timers (e.g., key expiry)
```

`ae.c` is 600 lines and abstracts `epoll` (Linux), `kqueue` (macOS/BSD), and `select` (fallback). The core loop waits for I/O readiness events, calls the registered handlers, and returns. There are no threads and no locks; simplicity is the point.

This is the **reactor pattern**: a single thread dispatches all events in a loop. The trade-off is that any blocking call, such as disk I/O or a slow `fork()` for snapshotting, blocks all clients. Redis avoids blocking calls or offloads them to background threads (since Redis 6.0 for disk I/O).

## SDS: Simple Dynamic Strings

Redis does not use C's `char *` internally. It uses **SDS (Simple Dynamic Strings)**, defined in `src/sds.h`:

```c
typedef char *sds;

struct sdshdr {
    unsigned int len;    // used bytes
    unsigned int alloc;  // allocated capacity
    unsigned char flags; // header type
    char buf[];          // the actual string data (flexible array member)
};
```

The `sds` typedef is simply `char *`, pointing to `buf`. The header lives immediately before `buf` in memory. To obtain the header from an `sds`, you subtract the header size: `(struct sdshdr *)((char *)s - sizeof(struct sdshdr))`.

This design means SDS strings are compatible with `strlen`, `printf`, and all C string functions because they are null-terminated. `sdslen(s)` is O(1) because it reads the stored `len` rather than scanning. The strings are binary-safe because `len` tracks actual bytes, so strings can contain `\0` bytes, unlike C strings. Growing a string is amortized O(1) because SDS doubles capacity when needed.

SDS is one of the most influential small libraries in C infrastructure. Its ideas appear in similar "fat string" designs across many projects.

## RESP3: The Next Generation Protocol

RESP2 has one fundamental limitation: the client must know the expected type of each response. `GET` returns a Bulk String; `HGETALL` returns a flat Array of alternating keys and values, and the client must know to pair them up.

RESP3 (Redis 7.0+) adds typed responses:

```
%2\r\n          ← Map with 2 entries
+key1\r\n
:100\r\n
+key2\r\n
,3.14\r\n       ← Double (new in RESP3)
```

New types include Map (`%`), Set (`~`), Double (`,`), Boolean (`#`), Null (`_`), Big Number (`(`), and Blob Error (`!`). The client no longer needs to know which commands return which types because the protocol is self-describing. RESP3 is negotiated at connection time via the `HELLO 3` command. RESP2 clients are not affected.

## Security: Never Trust Protocol Data

The multi-bulk header contains an integer provided by an external client. Parsing it with `atoi()` is dangerous:

```c
// Unsafe:
c->multibulklen = atoi(p + 1);

// Problems:
// 1. atoi returns 0 on failure: no way to distinguish "0" from parse error
// 2. atoi does not detect overflow: INT_MAX+1 wraps to a negative number
// 3. A negative multibulklen causes logic errors downstream
```

Redis's actual code uses `string2ll()`, a custom function that returns a boolean success or failure, detects overflow explicitly, and rejects values outside a valid range (`[1, 1024*1024]` for `multibulklen`).

CWE-190 (Integer Overflow) in length fields is a frequent source of heap buffer overflows. The correct pattern for any length parsed from external input:

```c
long long count;
if (string2ll(p+1, newlinepos-(p+1), &count) == 0 ||
    count > server.proto_max_bulk_len) {
    // reject the connection
    return C_ERR;
}
c->multibulklen = count;
```

Redis must never be exposed to the public internet without authentication (`requirepass`) and binding to localhost (`bind 127.0.0.1`). Redis's default configuration assumes a trusted network. An unauthenticated Redis instance on a public IP is a common initial access vector in cloud breaches: attackers can write SSH keys to `/root/.ssh/authorized_keys` via Redis's `CONFIG SET dir` / `CONFIG SET dbfilename` combined with `SAVE`.

## What You Need From Previous Lessons

- **Lesson 01 (stderr):** Diagnostic output goes to stderr; the result status to stdout: the same separation applies here.
- **Lesson 02 (fixed-width integers):** `int qb_pos` is a precise 32-bit signed offset into the buffer.
- **Lesson 07 (cache):** The querybuf is processed sequentially: the access pattern is cache-friendly.
- **Lesson 08 (EOF / streams):** The same discipline of checking sentinel values applies here, with `\r` as the delimiter.
- **Lesson 09 (call stack):** `c` is a pointer to a heap-allocated struct: changes to `c->qb_pos` persist after return.
- **Lesson 10 (pass-by-value):** `client *c` lets the function mutate the caller's state.
- **Lesson 13 (macro gotchas):** `C_OK` and `C_ERR` are simple `#define` constants, not enums and not functions.

## The Mission

Open `exercise.c`. You are implementing the first stage of `processMultibulkBuffer`.

The `include/server.h` file defines the `client` struct and the `C_OK`/`C_ERR` return codes.

1. Add `#include "include/server.h"` at the top.
2. Implement `processMultibulkBuffer(client *c)`:
   - Find the start of unparsed data: `c->querybuf + c->qb_pos`
   - Confirm the first byte is `*`. If not, return `C_ERR`.
   - Use `strchr` to find `\r`. If not found, return `C_ERR` (incomplete data: more TCP bytes needed).
   - Calculate how many characters are between `*` and `\r`.
   - Parse that substring as an integer and store it in `c->multibulklen`.
   - Advance `c->qb_pos` past the `\r\n` (advance by the full length of `*N\r\n`).
   - Return `C_OK`.
3. Do not modify the test harness in `main`.

When this works, you have implemented the first phase of what Redis actually does for every command it ever receives. The struct field updates, the position tracking, the early-return on partial input: this is real production logic used at millions of requests per second in production systems worldwide.

**Module 1 complete.** Module 2 covers memory allocation, heap management, valgrind, and the full allocator path from `malloc` to the OS kernel.
