---
difficulty: beginner
language: rust
tags: [入门, rust, hello-world]
tutorial:
  - "Rust 用 fn 声明函数。fn main() 是程序入口，和 C、Go 的 main() 一样。"
  - "println! 是宏，不是函数——! 用于区分宏和普通函数调用。"
  - "println! 自动添加换行符，不需要任何导入，它是语言内置的。"
---

# 你好，Rust

最后一种语言。

让程序准确输出：

```text
hello world
```

**Rust 的 hello world：**

```rust
fn main() {            // 函数声明（比 C 的 int main(void) 更简洁）
    println!("...");   // 宏：打印一行并自动换行
}
```

`!` 是 Rust 标记宏的方式。不需要 `#include` 或 `import`——`println!` 始终可用。

只需修改 `println!` 里的字符串。

## 答案

```rust
fn main() {
    println!("hello world");
}
```
