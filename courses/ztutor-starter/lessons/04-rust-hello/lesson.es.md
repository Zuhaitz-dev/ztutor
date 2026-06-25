---
difficulty: beginner
language: rust
tags: [inicio, rust, hello-world]
tutorial:
  - "Rust usa fn para declarar funciones. fn main() es el punto de entrada, como main() en C y Go."
  - "println! es una macro, no una función — el ! distingue las macros de las llamadas normales."
  - "println! añade el salto de línea automáticamente. No hace falta importar nada; está integrada."
---

# Hola, Rust

El último lenguaje del conjunto.

Haz que el programa imprima exactamente:

```text
hello world
```

**El enfoque de Rust en hello world:**

```rust
fn main() {            // declaración de función (más corta que int main(void) en C)
    println!("...");   // macro: imprime una línea con salto de línea al final
}
```

El `!` es la forma de Rust de marcar las macros. Sin `#include` ni `import` — `println!` siempre está disponible.

Solo el texto dentro de `println!` necesita cambiar.

## Respuesta

```rust
fn main() {
    println!("hello world");
}
```
