---
difficulty: beginner
language: go
tags: [starter, go, hello-world]
tutorial:
  - "Every Go file belongs to a package. Executable programs always use package main."
  - "Go imports packages explicitly — fmt provides print and format functions."
  - "fmt.Println adds a newline automatically, just like Python's print."
---

# Hello, Go

Third language, same goal.

Make the program print exactly:

```text
hello world
```

**How Go compares:**

```go
package main      // this file is an executable program
import "fmt"      // load the format/print package
func main() {     // entry point
    fmt.Println("...") // print a line with a newline at the end
}
```

Go is compiled like C but reads more like Python. Only the string inside `fmt.Println` needs to change.

## Answer

```go
package main

import "fmt"

func main() {
    fmt.Println("hello world")
}
```
