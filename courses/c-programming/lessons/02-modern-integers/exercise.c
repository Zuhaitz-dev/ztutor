#include <stdio.h>

// 1. TODO: Include the relative SQLite header file below


/**
 * parse_sqlite_header - Parses physical byte regions out of a raw database page block.
 * Modeled after the official SQLite disk layout specifications:
 * https://www.sqlite.org/fileformat.html
 */
void parse_sqlite_header(void) {
    // Simulated raw 100-byte database master header page array
    unsigned char raw_bytes[100] = {0};
    raw_bytes[18] = 2;          // File format write version
    raw_bytes[20] = 0x00;       // Page tail reserved space (High byte)
    raw_bytes[21] = 0x10;       // Page tail reserved space (Low byte) -> 16 bytes
    raw_bytes[24] = 0x00;       // File change counter bytes...
    raw_bytes[27] = 0x2A;       // 42 changes total

    // 2. TODO: Refactor these primitive types to their correct SQLite aliases (u8, u16, u32)
    unsigned int write_version = raw_bytes[18];
    unsigned int reserved_space = (raw_bytes[20] << 8) | raw_bytes[21];
    unsigned long file_change_counter = raw_bytes[27];

    // Structural validation trackers. Do not alter or reposition these outputs!
    printf("Write Version: %d (Size: %zu byte)\n", write_version, sizeof(write_version));
    printf("Reserved Space: %d (Size: %zu bytes)\n", reserved_space, sizeof(reserved_space));
    printf("Change Counter: %ld (Size: %zu bytes)\n", file_change_counter, sizeof(file_change_counter));
}

int main(void) {
    parse_sqlite_header();
    return 0;
}
