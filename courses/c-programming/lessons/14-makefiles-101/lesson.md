---
difficulty: intermediate
premium: false
tags: [build-systems, make, redis, automation, compilation, cmake, ninja, ccache]
build: make
build_output: redis-server
files:
  - name: Makefile
  - name: server.c
    editable: false
  - name: zmalloc.c
    editable: false
tutorial:
  - "Make is a dependency graph resolver, not a script runner. It only recompiles files whose sources have changed since the last build."
  - "A rule has three parts: target (what to build), prerequisites (what it depends on), and recipe (how to build it, indented with a TAB not spaces)."
  - "Automatic variables: $@ = target name, $< = first prerequisite, $^ = all prerequisites. These keep patterns DRY."
  - ".PHONY declares targets that are actions (like 'clean'), not files. Without it, Make confuses the target name with an existing file."
references:
  - "Redis Source: src/Makefile (https://github.com/redis/redis/blob/unstable/src/Makefile): 2,000+ lines of real-world Make, study it to see every pattern in production"
  - "GNU Make Manual (https://www.gnu.org/software/make/manual/make.html): the complete reference, chapter 2 is the quickest tutorial"
  - "Managing Projects with GNU Make: Robert Mecklenburg: the book for serious Make users"
  - "Recursive Make Considered Harmful, Peter Miller (https://aegis.sourceforge.net/auug97.pdf): why the common recursive pattern breaks incremental builds"
  - "Practical Makefiles, John Graham-Cumming (http://nuclear.mutantstargoat.com/articles/make/): real-world patterns for non-trivial projects"
  - "CMake Tutorial (https://cmake.org/cmake/help/latest/guide/tutorial/index.html): the generator most projects use above 10 source files"
  - "Meson Build System (https://mesonbuild.com/): modern alternative to CMake, faster, cleaner syntax"
  - "Ninja Build System (https://ninja-build.org/): the low-level backend CMake/Meson generate rules for, much faster than GNU Make"
  - "ccache, Compiler Cache (https://ccache.dev/): caches object files by hash; a full rebuild becomes a cache-hit load"
  - "pkg-config manual (https://www.freedesktop.org/wiki/Software/pkg-config/): how system libraries expose their flags to build systems"
  - "compile_commands.json: clangd LSP (https://clang.llvm.org/docs/JSONCompilationDatabase.html): how editors (VSCode, vim) know your include paths"
  - "Implicit rules in GNU Make: how Make guesses how to build .o from .c without explicit rules"
  - "Bear: compilation database generator (https://github.com/rizsotto/Bear): wraps a make invocation to produce compile_commands.json"
---
# Build Automation: The Redis Dependency Graph

## Why Make Exists

Without Make, the build command for a three-file C project is:

```bash
gcc -c server.c -o server.o
gcc -c zmalloc.c -o zmalloc.o
gcc server.o zmalloc.o -o redis-server
```

This works, but if you change one line in `zmalloc.c`, you must recompile everything, including files that have not changed. For Redis's hundreds of source files, a full rebuild takes minutes.

Make tracks **file timestamps**. If `zmalloc.c` is newer than `zmalloc.o`, Make recompiles it. If `zmalloc.o` is already newer because nothing changed, Make skips it. Only the minimum necessary work happens.

## Anatomy of a Makefile Rule

```makefile
target: prerequisites
	recipe
```

The recipe must be indented with a **literal Tab character**, not spaces. This is the most notorious Makefile gotcha: spaces look identical to tabs in most editors, but Make rejects them with `*** missing separator`.

A real example:

```makefile
zmalloc.o: zmalloc.c zmalloc.h
	gcc -c zmalloc.c -o zmalloc.o
```

Read as: "to build `zmalloc.o`, I need `zmalloc.c` and `zmalloc.h`. If either is newer than `zmalloc.o`, run the recipe."

## Automatic Variables

In Redis's Makefile, you see recipes like:

```makefile
%.o: %.c
	$(REDIS_CC) -MMD -o $@ -c $<
```

The `%` is a pattern: `%.o` matches any target ending in `.o`, and `%.c` is the corresponding source. The automatic variables fill in the blanks:

| Variable | Expands to |
|----------|-----------|
| `$@` | The target name (`zmalloc.o`) |
| `$<` | The first prerequisite (`zmalloc.c`) |
| `$^` | All prerequisites (link step: all `.o` files) |
| `$*` | The matched stem (`zmalloc` from `zmalloc.o`) |

Without these, you would repeat the filename in every recipe, and a rename would require changes in three places.

## Automatic Header Dependency Tracking: `-MMD`

The previous rule includes `-MMD`. This flag instructs GCC to emit a `.d` file alongside the `.o`:

```
zmalloc.d:
zmalloc.o: zmalloc.c zmalloc.h sds.h config.h
```

You include these `.d` files at the bottom of the Makefile:

```makefile
-include $(OBJS:.o=.d)
```

Now if you change `zmalloc.h`, Make knows to rebuild every `.o` that includes it, not just `zmalloc.o`. Without `-MMD`, changing a header does not trigger recompilation of its consumers, which leads to builds that succeed but produce incorrect binaries.

The `-` before `include` suppresses errors when `.d` files do not yet exist, as is the case on the first build.

## Parallel Builds: `make -j`

By default, Make runs rules one at a time. The `-j` flag allows parallel jobs:

```bash
make -j4            # 4 parallel jobs
make -j$(nproc)     # one job per CPU: maximum parallelism
```

For Redis's roughly 200 source files, `-j$(nproc)` on an 8-core machine reduces build time from approximately 90 seconds to approximately 15 seconds. The dependency graph ensures correctness: Make never starts a job before its prerequisites are complete. `$(nproc)` is a shell command substitution that expands to the number of available CPUs, and it is the standard idiom in CI scripts and developer aliases.

## The `.PHONY` Trap

```makefile
clean:
	rm -f *.o redis-server
```

This looks correct. If a file named `clean` exists in the directory, however, Make sees the target `clean` and checks whether it is newer than its prerequisites. There are no prerequisites, so `clean` is always considered up to date and `rm` never runs.

`.PHONY` tells Make that a target is an action, not a filename:

```makefile
.PHONY: clean all test install

clean:
	rm -f *.o redis-server

all: redis-server  # the default target: run first with no args
```

Any target that does not produce a file should be declared `.PHONY`. Redis declares `clean`, `distclean`, `test`, `install`, and others as phony.

## ccache: Compiler-Level Caching

When you run `make clean && make`, every file recompiles from scratch even if the source has not changed since the previous build. `ccache` addresses this:

```bash
# Prepend ccache to the compiler:
CC="ccache gcc" make -j$(nproc)

# Or set globally:
export CC="ccache gcc"
```

ccache hashes the preprocessed source and compiler flags. If the hash matches a cached object file, it copies the cache entry instead of running the compiler. A clean rebuild with a warm cache is nearly instant: cache hits take roughly 5 ms, while full compilation takes 200 to 500 ms per file.

`sccache` (Shared ccache) extends this to distributed caches, allowing teams to share a single S3 or Redis-backed cache so that each developer benefits from other developers' recent compilations.

## The Modern Build Stack

Make is 1976 technology. Modern projects layer on top of it:

```
Source Code
    ↓
CMake / Meson    ← Generate build system rules
    ↓
Ninja / GNU Make ← Execute the rules in dependency order
    ↓
Object Files → Linker → Binary
```

**CMake** generates Makefiles or Ninja files from `CMakeLists.txt`. It handles feature detection (`find_package(OpenSSL)`), cross-compilation, and IDE project files. Almost every serious open-source C project above a few thousand lines uses it.

**Meson** is a newer alternative to CMake with cleaner syntax and faster configuration. It is used by GLib, Systemd, and many GNOME projects.

**Ninja** is a build executor optimized for speed. It has no built-in rules; everything is generated by CMake or Meson. On large projects, Ninja starts builds noticeably faster than GNU Make because it has a simpler input format and less overhead.

## pkg-config: Finding System Libraries

When you write code that uses OpenSSL, how do you determine where its headers are installed?

```bash
$ pkg-config --cflags openssl
-I/usr/include/openssl

$ pkg-config --libs openssl
-lssl -lcrypto

# In a Makefile:
CFLAGS  += $(shell pkg-config --cflags openssl)
LDFLAGS += $(shell pkg-config --libs openssl)
```

`pkg-config` reads `.pc` files installed by libraries (for example, `/usr/lib/pkgconfig/openssl.pc`) and emits the correct compiler and linker flags. Hardcoding `/usr/include` paths breaks on different distributions and architectures. `pkg-config` is the standard solution.

## compile_commands.json: Editor Intelligence

Editors and language servers (clangd, the LSP backend for C/C++ in VSCode, Vim, and Emacs) need to know what flags were used to compile each file, including include paths, macros, and language standard. Without this information, your editor cannot resolve `#include "sds.h"` or autocomplete struct members.

CMake generates this automatically:

```bash
cmake -DCMAKE_EXPORT_COMPILE_COMMANDS=ON .
```

For Make-based projects, `bear` intercepts compiler calls during a build:

```bash
bear -- make -j$(nproc)
# writes compile_commands.json in the current directory
```

Once `compile_commands.json` exists, clangd reads it and provides accurate go-to-definition, find-references, and diagnostics, even in deeply nested header chains.

## Implicit Rules

GNU Make has built-in knowledge of common patterns. Even without a `%.o: %.c` rule in your Makefile, `make foo.o` will attempt to compile `foo.c` using the built-in `COMPILE.c` rule:

```makefile
# Built-in (you don't write this):
%.o: %.c
	$(CC) $(CPPFLAGS) $(CFLAGS) -c $< -o $@
```

This is why `make foo` on a directory containing `foo.c` often works with no Makefile at all. Understanding implicit rules explains confusing behavior when your explicit rules conflict with them. You can list all implicit rules with `make -p | grep '%'`.

## The Mission

Open `Makefile`. Three things are incomplete:

1. The `clean` target exists but is not declared `.PHONY`. Add the `.PHONY: clean` declaration.
2. The pattern rule `%.o: %.c` has an empty recipe. Complete it: use `$(REDIS_CC)` to compile `$<` into `$@` with `-c` and `-MMD` flags.
3. The final link rule `$(REDIS_SERVER_NAME):` has an empty recipe. Complete it: use `$(REDIS_LD)` to link `$^` into `$@`.

Bonus: After completing the Makefile, count how many `.d` files were generated. Inspect one with a text editor: you will see all the headers that file depends on, automatically discovered.
