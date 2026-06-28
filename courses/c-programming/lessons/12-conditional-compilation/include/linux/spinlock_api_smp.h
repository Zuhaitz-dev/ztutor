#ifndef LINUX_SPINLOCK_API_SMP_H
#define LINUX_SPINLOCK_API_SMP_H

#include <stdio.h>

static inline void __raw_spin_lock(spinlock_t *lock) {
    printf("[KERNEL-SMP] Hardware spinlock acquired!\n");
    lock->locked = 1;
}

#define spin_lock(lock) __raw_spin_lock(lock)

#endif /* LINUX_SPINLOCK_API_SMP_H */
