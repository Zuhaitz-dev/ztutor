#include <stdio.h>

// Simulating a Uniprocessor (UP) embedded environment build.
// #define CONFIG_SMP 1  <-- Intentionally omitted!

// Include the main kernel router
#include "include/linux/spinlock.h"

/**
 * kmalloc_mock - Simulates memory allocation in the kernel.
 */
void kmalloc_mock(spinlock_t *alloc_lock) {
    // On an SMP build, this locks the hardware.
    // On a UP build, your macro should make this compile into nothing.
    spin_lock(alloc_lock);
    
    printf("[KERNEL-ALLOC] Memory allocated successfully.\n");
}

int main(void) {
    spinlock_t mem_lock = { .locked = 0 };

    kmalloc_mock(&mem_lock);

    // Architectural verification check for the ztutor sandbox
    if (mem_lock.locked == 0) {
        printf("[DEBUG] Verification passed: UP lock was compiled out (Zero Overhead).\n");
    } else {
        printf("[DEBUG] Verification failed: SMP Lock was physically executed on a UP build.\n");
    }

    return 0;
}
