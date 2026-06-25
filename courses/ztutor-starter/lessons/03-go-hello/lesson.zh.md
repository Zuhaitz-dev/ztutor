---
difficulty: beginner
language: go
tags: [入门, go, hello-world]
tutorial:
  - "每个 Go 文件都属于一个包。可执行程序始终使用 package main。"
  - "Go 需要显式导入包——fmt 提供打印和格式化函数。"
  - "fmt.Println 自动添加换行，就像 Python 的 print 一样。"
---

# 你好，Go

第三种语言，同样的目标。

让程序准确输出：

```text
hello world
```

**Go 的代码结构：**

```go
package main           // 这个文件是可执行程序
import "fmt"           // 加载格式化/打印包
func main() {          // 程序入口
    fmt.Println("...") // 打印一行，末尾自动换行
}
```

Go 像 C 一样编译，但读起来更像 Python。只需修改 `fmt.Println` 里的字符串。

## 答案

```go
package main

import "fmt"

func main() {
    fmt.Println("hello world")
}
```
