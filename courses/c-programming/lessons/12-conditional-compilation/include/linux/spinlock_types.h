#ifndef LINUX_SPINLOCK_TYPES_H
#define LINUX_SPINLOCK_TYPES_H

typedef struct {
    volatile int locked;
} spinlock_t;

#endif /* LINUX_SPINLOCK_TYPES_H */
