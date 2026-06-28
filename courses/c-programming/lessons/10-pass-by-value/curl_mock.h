#ifndef CURL_MOCK_H
#define CURL_MOCK_H

/* * Opaque handle structure. Real libcurl hides this in its internal .c files. 
 */
typedef struct CURL {
    char url[256];
    int timeout_ms;
} CURL;

/* Mocking the variadic prototype */
int curl_easy_setopt(CURL *curl, int option, ...);

#endif
