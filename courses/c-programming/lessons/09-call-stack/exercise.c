#include <stdio.h>
#include <stdint.h>

/**
 * unwind_stack - Walks the stack frames to identify caller history.
 * Modeled after the unwinding logic in GDB (gdb/frame.c):
 * https://sourceware.org/git/gdb.git
 */
void unwind_stack(uintptr_t *stack_mem, int max_depth) {
    // Current register snapshot: RBP points to the current frame start
    // The frame structure is: [prev_rbp][return_addr]
    
    // 1. TODO: Implement the traversal. 
    // Start at stack_mem[0].
    // Pointer math: frame[0] is the pointer to the previous frame.
    // Pointer math: frame[1] is the saved return address.
    
    int depth = 0;
    while (depth < max_depth) {
        uintptr_t prev_rbp = stack_mem[depth * 2];
        uintptr_t ret_addr = stack_mem[(depth * 2) + 1];

        if (ret_addr == 0) break; // End of chain

        printf("Frame %d: Return Address 0x%lx\n", depth, ret_addr);
        
        depth++;
    }
}

int main(void) {
    // Mock memory dump of 3 frames: [prev_rbp, ret_addr]
    // Final frame points to 0 to signal end of stack
    uintptr_t mock_stack[] = {
        0x7fff1234, 0x400500, // Frame 0
        0x7fff5678, 0x400620, // Frame 1
        0x00000000, 0x000000  // End of chain
    };

    unwind_stack(mock_stack, 3);
    return 0;
}
