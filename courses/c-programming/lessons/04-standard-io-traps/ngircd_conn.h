#ifndef NGIRCD_CONN_H
#define NGIRCD_CONN_H

#define COMMAND_LEN 16

/*
** Connection structure block replicated from the official ngIRCd engine.
** Core Subsystem: Network Stream Buffer & Ingestion Interface
** Source Reference: https://github.com/ngircd/ngircd/blob/master/src/ngircd/conn.c
*/
typedef struct _CONNECTION {
    int sock_fd;                /* Low-level network socket file descriptor */
    char read_buffer[COMMAND_LEN]; /* Local raw data assembly buffer */
    unsigned int bytes_handled;  /* Metric counter for validation tracking */
} CONNECTION;

#endif /* NGIRCD_CONN_H */
