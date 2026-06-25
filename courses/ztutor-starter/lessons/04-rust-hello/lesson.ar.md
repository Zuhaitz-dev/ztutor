---
difficulty: beginner
language: rust
tags: [بداية, rust, hello-world]
tutorial:
  - "تستخدم Rust fn للإعلان عن الدوال. fn main() هي نقطة الدخول، كما في C وGo."
  - "println! هو ماكرو وليس دالة — علامة ! تُميّز الماكروهات عن الاستدعاءات العادية."
  - "println! يضيف سطراً جديداً تلقائياً. لا حاجة لاستيراد أي شيء؛ إنه مدمج في اللغة."
---

# مرحباً، Rust

آخر لغة في المجموعة.

اجعل البرنامج يطبع بالضبط:

```text
hello world
```

**طريقة Rust في hello world:**

```rust
fn main() {            // إعلان دالة (أقصر من int main(void) في C)
    println!("...");   // ماكرو: يطبع سطراً مع سطر جديد في النهاية
}
```

`!` هي طريقة Rust لتمييز الماكروهات. لا `#include` ولا `import` — `println!` متاح دائماً.

فقط النص داخل `println!` يحتاج إلى تغيير.

## الجواب

```rust
fn main() {
    println!("hello world");
}
```
