#include <stdio.h>
#include <stdarg.h>
#include <string.h>
#include "curl_mock.h"

#define CURLOPT_TIMEOUT_MS 100

/**
 * curl_easy_setopt - Production-style setter for session state.
 * Modeled after: https://github.com/curl/curl/blob/master/lib/setopt.c
 */
int curl_easy_setopt(CURL *curl, int option, ...) {
    va_list arg;
    va_start(arg, option);

    if (option == CURLOPT_TIMEOUT_MS) {
        // 1. TODO: Extract the integer from the variadic list
        // 2. TODO: Mutate the pointer 'curl' to update timeout_ms
        int val = va_arg(arg, int);
        // ... mutation logic here ...
    }

    va_end(arg);
    return 0;
}

int main(void) {
    CURL session = { .url = "https://ztutor.io", .timeout_ms = 3000 };

    printf("[CURL] Initial timeout: %dms\n", session.timeout_ms);
    
    // 3. TODO: Pass the pointer to the session, not the struct itself
    curl_easy_setopt(&session, CURLOPT_TIMEOUT_MS, 5000);

    if (session.timeout_ms == 5000) {
        printf("[CURL] State mutation successful: 5000ms\n");
    } else {
        printf("[CURL] Mutation failed: Timeout remains %dms\n", session.timeout_ms);
    }

    return 0;
}
