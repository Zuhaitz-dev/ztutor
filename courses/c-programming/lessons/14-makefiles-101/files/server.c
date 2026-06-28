#include <stdio.h>

extern void zmalloc_init(void);

int main(void) {
    zmalloc_init();
    printf("[REDIS-MOCK] Server initialized and listening...\n");
    return 0;
}
