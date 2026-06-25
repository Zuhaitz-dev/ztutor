---
difficulty: beginner
language: python
tags: [inicio, python, hello-world]
tutorial:
  - "Python es interpretado — lo ejecutas directamente sin paso de compilación."
  - "print() muestra una línea de texto y añade el salto de línea automáticamente."
  - "La indentación importa en Python. El código dentro de una función debe estar indentado."
---

# Hola, Python

El mismo objetivo que la lección de C, pero en otro lenguaje.

Haz que el programa imprima exactamente:

```text
hello world
```

**Comparado con C:**

| C | Python |
|---|--------|
| `#include <stdio.h>` | *(no hace falta)* |
| `printf("...\n")` | `print("...")` |
| `return 0;` | *(no hace falta)* |

Python gestiona el salto de línea y la salida por ti. Solo el texto dentro de `print` necesita cambiar.

## Respuesta

```python
def main():
    print("hello world")

if __name__ == "__main__":
    main()
```
