#include <stdio.h>
#include "include/linux/minmax.h"

/**
 * process_network_retry - Simulates a kernel network buffer retry loop.
 */
int main(void) {
    int retries = 0;
    int max_allowed = 5;

    // The Bug: If we use MIN(retries++, max_allowed), retries increments TWICE!
    // 1. TODO: Change MIN to your newly implemented SAFE_MIN macro.
    int clamped_val = MIN(retries++, max_allowed);

    // Architectural verification check
    if (retries == 1 && clamped_val == 0) {
        printf("[KERNEL] Success: Safe macro prevented double-evaluation.\n");
    } else {
        printf("[FATAL] State corruption detected! Double evaluation occurred.\n");
        printf("        Expected retries=1, clamped_val=0.\n");
        printf("        Actual   retries=%d, clamped_val=%d.\n", retries, clamped_val);
    }

    return 0;
}
