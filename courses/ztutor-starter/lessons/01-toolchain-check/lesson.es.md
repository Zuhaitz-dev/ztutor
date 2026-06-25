---
difficulty: beginner
tags: [inicio, c, hello-world]
tutorial:
  - "Los programas C empiezan con #include para cargar cabeceras. <stdio.h> da acceso a printf."
  - "La ejecución comienza en main(). El valor de retorno int señala éxito (0) o fallo al shell."
  - "printf imprime texto formateado. El \\n al final mueve el cursor a la siguiente línea."
---

# Hola, C

Tu primer programa C. Haz que imprima exactamente:

```text
hello world
```

**Lo que estás viendo:**

```c
#include <stdio.h>   // carga la biblioteca estándar de E/S
int main(void) {     // punto de entrada del programa
    printf("...\n"); // imprime texto — \n es un salto de línea
    return 0;        // 0 significa éxito
}
```

Solo el texto dentro de `printf` necesita cambiar.

## Respuesta

```c
#include <stdio.h>

int main(void) {
    printf("hello world\n");
    return 0;
}
```
