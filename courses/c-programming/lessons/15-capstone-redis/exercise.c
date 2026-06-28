#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "include/server.h"

/**
 * processMultibulkBuffer - Parses the RESP array header.
 * Modeled strictly after the Redis source: src/networking.c
 */
int processMultibulkBuffer(client *c) {
    char *newline = NULL;
    int pos = 0;

    // The unread portion of the buffer starts at c->querybuf + c->qb_pos
    if (c->multibulklen == 0) {
        // 1. TODO: Verify the first unread byte is the '*' array marker
        if (/* ... */) {
            printf("[REDIS-CORE] Protocol Error: Expected '*'\n");
            return C_ERR;
        }

        // 2. TODO: Find the carriage return '\r' in the unread buffer using strchr
        newline = strchr(/* ... */, '\r');
        
        if (newline == NULL) {
            // Buffer doesn't contain a full line yet. Wait for more network data.
            return C_ERR;
        }

        // 3. TODO: Calculate the integer value between '*' and '\r'.
        // You can use strtol on the memory address right after the '*'.
        long long ll = strtol((c->querybuf + c->qb_pos + 1), NULL, 10);
        
        c->multibulklen = (int)ll;

        // 4. TODO: Advance qb_pos past the integer and the \r\n (which is 2 bytes)
        // Hint: newline is a pointer to the \r. Subtracting the base pointer gives the length!
        pos = (newline - (c->querybuf + c->qb_pos));
        c->qb_pos += /* ... */;

        printf("[REDIS-CORE] Parsed Multibulk header: %d elements.\n", c->multibulklen);
        return C_OK;
    }

    return C_OK;
}

int main(void) {
    // Simulated raw TCP socket read for: "SET mykey myval"
    char raw_socket_data[] = "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$7\r\nmyval\r\n";

    // Initialize the authentic client struct state
    client c;
    c.querybuf = raw_socket_data;
    c.qb_pos = 0;
    c.reqtype = PROTO_REQ_MULTIBULK;
    c.multibulklen = 0;

    // Run the parser state machine
    if (processMultibulkBuffer(&c) == C_OK) {
        printf("[DEBUG] Next unparsed character is: '%c'\n", c.querybuf[c.qb_pos]);
    } else {
        printf("[DEBUG] Parser failed or aborted.\n");
    }

    return 0;
}
