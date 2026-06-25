---
difficulty: beginner
language: python
tags: [starter, python, hello-world]
tutorial:
  - "Python is interpreted — you run it directly without a compile step."
  - "print() outputs a line of text and adds a newline automatically."
  - "Indentation matters in Python. Code inside a function must be indented."
---

# Hello, Python

Same goal as the C lesson, different language.

Make the program print exactly:

```text
hello world
```

**Compared to C:**

| C | Python |
|---|--------|
| `#include <stdio.h>` | *(not needed)* |
| `printf("...\n")` | `print("...")` |
| `return 0;` | *(not needed)* |

Python handles the newline and exit for you. Only the string inside `print` needs to change.

## Answer

```python
def main():
    print("hello world")

if __name__ == "__main__":
    main()
```
