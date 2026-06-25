---
difficulty: beginner
language: go
tags: [بداية, go, hello-world]
tutorial:
  - "كل ملف Go ينتمي إلى حزمة. البرامج القابلة للتنفيذ تستخدم دائماً package main."
  - "تستورد Go الحزم صراحةً — fmt توفر دوال الطباعة والتنسيق."
  - "fmt.Println يضيف سطراً جديداً تلقائياً، تماماً كـ print في بايثون."
---

# مرحباً، Go

اللغة الثالثة، نفس الهدف.

اجعل البرنامج يطبع بالضبط:

```text
hello world
```

**كيف تُقارَن Go:**

```go
package main           // هذا الملف برنامج قابل للتنفيذ
import "fmt"           // تحميل حزمة التنسيق/الطباعة
func main() {          // نقطة الدخول
    fmt.Println("...") // يطبع سطراً مع سطر جديد في النهاية
}
```

تُترجَم Go مثل C لكن تُقرأ أقرب إلى بايثون. فقط النص داخل `fmt.Println` يحتاج إلى تغيير.

## الجواب

```go
package main

import "fmt"

func main() {
    fmt.Println("hello world")
}
```
