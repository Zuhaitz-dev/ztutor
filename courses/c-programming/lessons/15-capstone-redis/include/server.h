#ifndef REDIS_SERVER_H
#define REDIS_SERVER_H

#include <stddef.h>

/*
** Authentic Redis connection and protocol structures.
** Source: src/server.h
*/

#define C_OK 0
#define C_ERR -1

/* Protocol states */
#define PROTO_REQ_MULTIBULK 2

typedef char * sds; /* Redis uses 'Simple Dynamic Strings'. We mock it as char* here. */

/* * The authentic Redis client state structure. 
 * This allows the parser to pause and resume if a TCP packet is fragmented.
 */
typedef struct client {
    sds querybuf;           /* Buffer we use to accumulate client queries. */
    size_t qb_pos;          /* The position we have read in querybuf. */
    int reqtype;            /* Protocol type being parsed. */
    int multibulklen;       /* Number of multi-bulk arguments left to read. */
    long long bulklen;      /* Length of the current bulk argument. */
} client;

#endif /* REDIS_SERVER_H */
