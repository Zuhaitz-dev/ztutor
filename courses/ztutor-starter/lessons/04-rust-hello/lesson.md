---
difficulty: beginner
language: rust
tags: [starter, rust, hello-world]
tutorial:
  - "Rust uses fn to declare functions. fn main() is the entry point, like main() in C and Go."
  - "println! is a macro, not a function — the ! distinguishes macros from regular calls."
  - "println! adds a newline automatically. No imports needed; it is built into the language."
---

# Hello, Rust

Last language in the set.

Make the program print exactly:

```text
hello world
```

**Rust's take on hello world:**

```rust
fn main() {            // function declaration (shorter than C's int main(void))
    println!("...");   // macro: prints a line with a newline at the end
}
```

The `!` is Rust's way of marking macros. No `#include`, no `import` — `println!` is always available.

Only the string inside `println!` needs to change.

## Answer

```rust
fn main() {
    println!("hello world");
}
```
