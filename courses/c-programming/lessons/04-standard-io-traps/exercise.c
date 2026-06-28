#include <stdio.h>
#include <string.h>

#define MAX_BUFFER_SIZE 16

/**
 * parse_irc_stream - Implements an isolated protocol command framing slice.
 * Reflects the architectural separation seen in ngIRCd's processing engine:
 * https://github.com/ngircd/ngircd/blob/master/src/ngircd/conn.c
 */
void parse_irc_stream(void) {
    char command_buffer[MAX_BUFFER_SIZE];

    // 1. TODO: Use fgets to securely read up to MAX_BUFFER_SIZE bytes from stdin into command_buffer
    

    // 2. TODO: Detect if the input line was truncated. If a newline ('\n') is missing 
    //    and the string has exhausted the buffer limits, handle the overlong state.
    //    Format: "ERROR: IRC line too long\n"
    

    // 3. TODO: Sanitize the string bounds by stripping a trailing '\n', 
    //    along with an adjacent trailing carriage return '\r' if present.
    

    // 4. TODO: Evaluate the input. Ensure it matches exactly the command "JOIN #ztutor".
    //    Outputs:
    //    - If valid: "OK: joined channel\n"
    //    - If invalid: "ERROR: unknown IRC command\n"
    
}

int main(void) {
    parse_irc_stream();
    return 0;
}
