#ifndef INTERNAL_CRYPTLIB_H
#define INTERNAL_CRYPTLIB_H

/*
** Veridic slice of OpenSSL internal diagnostic macros.
** Source: crypto/include/internal/cryptlib.h
** Context: Enforcing secure assertions that vanish in production.
*/

#ifndef NDEBUG
# define OSS_ASSERT(cond, msg) \
    do { \
        if (!(cond)) { \
            fprintf(stderr, "[OSS_ASSERT] Failed at %s:%d: %s\n", \
                    __FILE__, __LINE__, msg); \
            exit(1); \
        } \
    } while (0)
#else
# define OSS_ASSERT(cond, msg) ((void)0)
#endif

#endif /* INTERNAL_CRYPTLIB_H */
