---
difficulty: beginner
language: python
tags: [入门, python, hello-world]
tutorial:
  - "Python 是解释型语言——直接运行，不需要编译步骤。"
  - "print() 输出一行文本，自动添加换行符。"
  - "Python 中缩进非常重要。函数内的代码必须缩进。"
---

# 你好，Python

和 C 课的目标一样，只是换了一种语言。

让程序准确输出：

```text
hello world
```

**与 C 的对比：**

| C | Python |
|---|--------|
| `#include <stdio.h>` | *(不需要)* |
| `printf("...\n")` | `print("...")` |
| `return 0;` | *(不需要)* |

Python 自动处理换行和退出。只需修改 `print` 里的字符串。

## 答案

```python
def main():
    print("hello world")

if __name__ == "__main__":
    main()
```
