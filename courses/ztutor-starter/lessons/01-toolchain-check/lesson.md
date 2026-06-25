---
difficulty: beginner
tags: [starter, c, hello-world]
tutorial:
  - "C programs start with #include to load library headers. <stdio.h> gives you printf."
  - "Execution begins at main(). The int return value signals success (0) or failure to the shell."
  - "printf prints formatted text. The \\n at the end moves the cursor to the next line."
---

# Hello, C

Your first C program. Make it print exactly:

```text
hello world
```

**What you are looking at:**

```c
#include <stdio.h>   // load the standard I/O library
int main(void) {     // program entry point
    printf("...\n"); // print text — \n is a newline
    return 0;        // 0 means success
}
```

Only the string inside `printf` needs to change.

## Answer

```c
#include <stdio.h>

int main(void) {
    printf("hello world\n");
    return 0;
}
```
