---
difficulty: beginner
tags: [intro, foundations, resources, meta]
references:
  - "The C Programming Language, 2nd ed., Kernighan & Ritchie (K&R): The source. Read chapter by chapter alongside this module."
  - "Beej's Guide to C Programming (https://beej.us/guide/bgc/): Free, modern, and written for people who actually want to understand things."
  - "cdecl.org: Translate cryptic C declarations to English in real time."
  - "godbolt.org (Compiler Explorer): Paste any C snippet and watch the assembly it produces. Use it constantly."
  - "SEI CERT C Coding Standard (https://wiki.sei.cmu.edu/confluence/display/c/SEI+CERT+C+Coding+Standard): How production engineers write defensive C."
  - "POSIX.1-2017 man pages (https://pubs.opengroup.org/onlinepubs/9699919799/): The authoritative reference for every standard library function."
  - "Ulrich Drepper, What Every Programmer Should Know About Memory (https://people.freedesktop.org/~ajax/nm-commit-rant.html): The definitive essay on cache, NUMA, and memory hierarchy. Read it when you hit Module 7."
  - "Julia Evans, Memory Zines (https://wizardzines.com/): Visual, approachable breakdowns of pointer mechanics and memory layout."
  - "Linus Torvalds on good C taste (https://github.com/mkirchner/linked-list-good-taste): A short GitHub repo demonstrating Linus's famous 'good taste' example for linked list pointer handling."
  - "The Lost Art of C Structure Packing, Eric Raymond (http://www.catb.org/esr/structure-packing/): Essential reading before Module 4 (Structs & Data Layout)."
  - "Computer Systems: A Programmer's Perspective, Bryant & O'Hallaron (CS:APP): The textbook behind CMU's legendary 15-213 course. Chapters 1–3 pair directly with this module."
  - "John Regehr's blog (https://blog.regehr.org/): Undefined behavior, compiler internals, and what C actually guarantees. Essential reading for the sharp edges."
  - "Hacker News search: 'C undefined behavior': The comment sections on these threads contain more practical wisdom than most textbooks."
---
# Module 1: Modern Foundations

Welcome to the C Programming course.

This module covers the building blocks of every C program, but it does not treat them as trivial. Each lesson is grounded in a real codebase: Git, SQLite, Redis, the Linux kernel, ngIRCd, FFmpeg. By the end of the 15 lessons, you will parse a live Redis protocol frame from scratch.

## What this module covers

| Lesson | Topic | Codebase anchor |
|--------|-------|----------------|
| 01 | stdout vs stderr: why the split exists | Git |
| 02 | Fixed-width integers: `<stdint.h>` | SQLite |
| 03 | Scope and variable shadowing | Game loop pattern |
| 04 | Safe I/O with `fgets`: buffer traps | ngIRCd |
| 05 | Booleans in C: `<stdbool.h>` | Linux type history |
| 06 | Switch fallthrough: Duff's Device | Lucasfilm, 1983 |
| 07 | For-loops and cache locality | FFmpeg |
| 08 | While-loops and EOF: reading `wc` | Unix V7 source |
| 09 | Functions and the call stack | GDB frame unwinding |
| 10 | Pass-by-value: why `swap()` fails | curl internals |
| 11 | The C preprocessor: macros | cryptlib |
| 12 | Conditional compilation: `#ifdef` | Linux kernel |
| 13 | Macro gotchas: `MAX(a,b)` dangers | Linux kernel |
| 14 | Makefiles 101 | Redis build system |
| 15 | Capstone: parse a Redis RESP frame | Redis networking |

## Prerequisites

This module assumes you can already open a terminal and run commands, write a basic program in any language, and understand what a variable, a loop, and a function are. You do **not** need prior C experience. You **will** be reading and modifying real C code from lesson one.

## A note on difficulty

C is not hard because the concepts are complex. It is hard because the language makes no assumptions and offers no safety net. When something goes wrong, it goes wrong silently: the wrong number prints, or nothing prints at all, or the program crashes with no useful message.

The discipline you build here, reading compiler output carefully, checking every format string, and thinking about what is in memory before you use it, is the same discipline that makes engineers effective in every other language as well.

## How to use this course

Each lesson has three layers. The first is the lesson text, which provides context, history, and the concept explained through a real codebase. The second is the exercise, a real function to implement that is validated against expected output. The third is trivia and hints, offering deeper dives and escape hatches when you get stuck.

Read the lesson text before touching the editor. The mission instructions are precise: pay attention to exact format strings, capitalization, and newlines. The validator matches output byte-for-byte.

Use the GDB debugger (`Ctrl+G`) freely. Use the assembly view (`Ctrl+A`) to see what the compiler actually generates. These are not advanced tools; they are standard equipment.

## The resources list

The references below are not homework. They are a map of the field. Some you will use immediately (K&R, Beej's Guide, godbolt). Others will make more sense later (Drepper's memory paper, CS:APP). Bookmark them and return as topics come up.

The more specialized entries, Linus's linked list example, John Regehr's blog, and the structure packing essay, are where the real understanding lives. Textbooks teach you what the language does. These teach you what experienced engineers think about when they write it.
