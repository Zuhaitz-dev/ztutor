---
difficulty: beginner
tags: [بداية, c, hello-world]
tutorial:
  - "تبدأ برامج C بـ #include لتحميل مكتبات. <stdio.h> يوفر دالة printf."
  - "يبدأ التنفيذ من main(). قيمة الإرجاع int تُخبر الـ shell بالنجاح (0) أو الفشل."
  - "printf يطبع نصاً. رمز \\n في النهاية ينقل المؤشر إلى السطر التالي."
---

# مرحباً، C

برنامجك الأول بلغة C. اجعله يطبع بالضبط:

```text
hello world
```

**ما تراه في الكود:**

```c
#include <stdio.h>   // تحميل مكتبة الإدخال/الإخراج
int main(void) {     // نقطة بداية البرنامج
    printf("...\n"); // يطبع نصاً — \n هو سطر جديد
    return 0;        // 0 يعني النجاح
}
```

فقط النص داخل `printf` يحتاج إلى تغيير.

## الجواب

```c
#include <stdio.h>

int main(void) {
    printf("hello world\n");
    return 0;
}
```
