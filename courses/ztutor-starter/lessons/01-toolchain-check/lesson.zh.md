---
difficulty: beginner
tags: [入门, c, hello-world]
tutorial:
  - "C 程序以 #include 开头，用于加载头文件。<stdio.h> 提供 printf 函数。"
  - "程序从 main() 开始执行。int 返回值向 shell 表示成功（0）或失败。"
  - "printf 输出格式化文本。末尾的 \\n 将光标移到下一行。"
---

# 你好，C

你的第一个 C 程序。让它准确输出：

```text
hello world
```

**代码结构说明：**

```c
#include <stdio.h>   // 加载标准输入输出库
int main(void) {     // 程序入口
    printf("...\n"); // 输出文本——\n 是换行符
    return 0;        // 0 表示成功
}
```

只需修改 `printf` 里的字符串。

## 答案

```c
#include <stdio.h>

int main(void) {
    printf("hello world\n");
    return 0;
}
```
