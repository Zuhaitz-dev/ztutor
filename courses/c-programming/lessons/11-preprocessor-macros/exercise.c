#include <stdio.h>
#include <stdlib.h>
#include "internal/cryptlib.h"

/**
 * process_key - Simulates a sensitive cryptographic routine.
 * Demonstrates the use of internal preprocessor security macros.
 */
void process_key(int key_strength) {
    // This assertion protects production from weak keys.
    // It should log a file/line diagnostic only in DEBUG_MODE.
    OSS_ASSERT(key_strength >= 128, "Key strength is too low for production");
    
    printf("[CRYPT] Key processing initiated with strength: %d\n", key_strength);
}

int main(void) {
    // 1. Production code would compile with -DNDEBUG
    // 2. Debug code would compile without it.
    process_key(64); 

    // The dangling else trap check
    if (1)
        OSS_ASSERT(1, "Success");
    else
        printf("Macro is broken: Else-trap triggered!\n");

    return 0;
}
