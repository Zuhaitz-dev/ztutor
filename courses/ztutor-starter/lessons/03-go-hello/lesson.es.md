---
difficulty: beginner
language: go
tags: [inicio, go, hello-world]
tutorial:
  - "Cada archivo Go pertenece a un paquete. Los programas ejecutables siempre usan package main."
  - "Go importa paquetes explícitamente — fmt proporciona funciones de impresión y formato."
  - "fmt.Println añade el salto de línea automáticamente, igual que print de Python."
---

# Hola, Go

Tercer lenguaje, mismo objetivo.

Haz que el programa imprima exactamente:

```text
hello world
```

**Cómo se compara Go:**

```go
package main           // este archivo es un programa ejecutable
import "fmt"           // carga el paquete de formato/impresión
func main() {          // punto de entrada
    fmt.Println("...") // imprime una línea con salto de línea al final
}
```

Go se compila como C pero se lee más como Python. Solo el texto dentro de `fmt.Println` necesita cambiar.

## Respuesta

```go
package main

import "fmt"

func main() {
    fmt.Println("hello world")
}
```
