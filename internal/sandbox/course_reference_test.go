package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceSolutions_KeyLessons(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	tests := []struct {
		name       string
		files      map[string]string
		stdin      string
		wantOutput string
	}{
		{
			name: "03-scope-shadowing",
			files: map[string]string{
				"main.c": `#include <stdio.h>
#include <stdint.h>

struct delta_index {
    unsigned int long mem_size;
    void *src_buf;
};

struct delta_index *create_delta_index(const void *buf, unsigned long bufsize) {
    int32_t entry_index = 100;
    int32_t entry_stride = 25;
    int32_t entry_valid = 1;

    printf("[GIT] create_delta_index processing block window at offset: %d\n", entry_index);

    if (entry_valid > 0) {
        entry_index = entry_index + entry_stride;
        printf("[GIT] Linked entry offset to %d inside initialization block\n", entry_index);
    }

    printf("[GIT] Final committed delta entry index: %d\n", entry_index);
    return NULL;
}

int main(void) {
    const char dummy_data[10] = "PACKDATA";
    create_delta_index(dummy_data, 8);
    return 0;
}`,
			},
			wantOutput: "[GIT] create_delta_index processing block window at offset: 100\n[GIT] Linked entry offset to 125 inside initialization block\n[GIT] Final committed delta entry index: 125\n",
		},
		{
			name: "06-duffs-device",
			files: map[string]string{
				"main.c": `#include <stdio.h>
#include <stdint.h>

void send(volatile uint16_t *to, const uint16_t *from, int count) {
    if (count <= 0) return;
    int n = (count + 7) / 8;
    switch (count % 8) {
        case 0: do { *to = *from++;
        case 7: *to = *from++;
        case 6: *to = *from++;
        case 5: *to = *from++;
        case 4: *to = *from++;
        case 3: *to = *from++;
        case 2: *to = *from++;
        case 1: *to = *from++;
                } while (--n > 0);
    }
}

int main(void) {
    volatile uint16_t mock_hardware_port = 0;
    uint16_t pixel_stream[11] = { 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 1983 };
    send(&mock_hardware_port, pixel_stream, 11);
    printf("[NET.LANG.C] Duff's send verification success. Output register tail value: %d\n", mock_hardware_port);
    return 0;
}`,
			},
			wantOutput: "[NET.LANG.C] Duff's send verification success. Output register tail value: 1983\n",
		},
		{
			name: "08-while-eof",
			files: map[string]string{
				"main.c": `#include <stdio.h>

int main(void) {
    long linect = 0;
    long wordct = 0;
    long charct = 0;
    int token = 0;
    int c;

    while ((c = getchar()) != EOF) {
        charct++;
        if (c == '\n') linect++;
        if (c > ' ' && c < 0177) {
            if (!token) { wordct++; token = 1; }
        } else {
            token = 0;
        }
    }

    printf("%7ld %7ld %7ld\n", linect, wordct, charct);
    return 0;
}`,
			},
			stdin:      "Hello Server!\n",
			wantOutput: "      1       2      14\n",
		},
		{
			name: "09-call-stack",
			files: map[string]string{
				"main.c": `#include <stdio.h>
#include <stdint.h>

void unwind_stack(uintptr_t *stack_mem, int max_depth) {
    int depth = 0;
    while (depth < max_depth) {
        uintptr_t prev_rbp = stack_mem[depth * 2];
        uintptr_t ret_addr = stack_mem[(depth * 2) + 1];
        if (ret_addr == 0) break;
        printf("Frame %d: Return Address 0x%lx\n", depth, ret_addr);
        depth++;
    }
}

int main(void) {
    uintptr_t mock_stack[] = {
        0x7fff1234, 0x400500,
        0x7fff5678, 0x400620,
        0x00000000, 0x000000
    };
    unwind_stack(mock_stack, 3);
    return 0;
}`,
			},
			wantOutput: "Frame 0: Return Address 0x400500\nFrame 1: Return Address 0x400620\n",
		},
		{
			name: "12-conditional-compilation",
			files: map[string]string{
				"main.c": `#include <stdio.h>

#include "include/linux/spinlock.h"

void kmalloc_mock(spinlock_t *alloc_lock) {
    spin_lock(alloc_lock);
    printf("[KERNEL-ALLOC] Memory allocated successfully.\n");
}

int main(void) {
    spinlock_t mem_lock = { .locked = 0 };
    kmalloc_mock(&mem_lock);
    if (mem_lock.locked == 0) {
        printf("[DEBUG] Verification passed: UP lock was compiled out (Zero Overhead).\n");
    } else {
        printf("[DEBUG] Verification failed: SMP Lock was physically executed on a UP build.\n");
    }
    return 0;
}`,
				"include/linux/spinlock.h": `#ifndef LINUX_SPINLOCK_H
#define LINUX_SPINLOCK_H

#include "spinlock_types.h"

#ifndef CONFIG_SMP
#include "spinlock_api_up.h"
#else
#include "spinlock_api_smp.h"
#endif

#endif`,
				"include/linux/spinlock_types.h": `#ifndef LINUX_SPINLOCK_TYPES_H
#define LINUX_SPINLOCK_TYPES_H

typedef struct {
    volatile int locked;
} spinlock_t;

#endif`,
				"include/linux/spinlock_api_up.h": `#ifndef LINUX_SPINLOCK_API_UP_H
#define LINUX_SPINLOCK_API_UP_H

#define spin_lock(lock) do { (void)(lock); } while(0)

#endif`,
				"include/linux/spinlock_api_smp.h": `#ifndef LINUX_SPINLOCK_API_SMP_H
#define LINUX_SPINLOCK_API_SMP_H

#include <stdio.h>

static inline void __raw_spin_lock(spinlock_t *lock) {
    printf("[KERNEL-SMP] Hardware spinlock acquired!\n");
    lock->locked = 1;
}

#define spin_lock(lock) __raw_spin_lock(lock)

#endif`,
			},
			wantOutput: "[KERNEL-ALLOC] Memory allocated successfully.\n[DEBUG] Verification passed: UP lock was compiled out (Zero Overhead).\n",
		},
		{
			name: "13-macro-gotchas",
			files: map[string]string{
				"main.c": `#include <stdio.h>
#include "include/linux/minmax.h"

int main(void) {
    int retries = 0;
    int max_allowed = 5;
    int clamped_val = SAFE_MIN(retries++, max_allowed);

    if (retries == 1 && clamped_val == 0) {
        printf("[KERNEL] Success: Safe macro prevented double-evaluation.\n");
    } else {
        printf("[FATAL] State corruption detected! Double evaluation occurred.\n");
        printf("        Expected retries=1, clamped_val=0.\n");
        printf("        Actual   retries=%d, clamped_val=%d.\n", retries, clamped_val);
    }

    return 0;
}`,
				"include/linux/minmax.h": `#ifndef _LINUX_MINMAX_H
#define _LINUX_MINMAX_H

#define __cmp_op_min <
#define __cmp_op_max >

#define __cmp(op, x, y)    ((x) __cmp_op_##op (y) ? (x) : (y))

#define MIN(a, b) __cmp(min, a, b)

#define SAFE_MIN(x, y) ({ \
    typeof(x) _x = (x); \
    typeof(y) _y = (y); \
    _x < _y ? _x : _y; \
})

#endif`,
			},
			wantOutput: "[KERNEL] Success: Safe macro prevented double-evaluation.\n",
		},
		{
			name: "10-pass-by-value",
			files: map[string]string{
				"main.c": `#include <stdio.h>
#include <stdarg.h>
#include <string.h>
#include "curl_mock.h"

#define CURLOPT_TIMEOUT_MS 100

int curl_easy_setopt(CURL *curl, int option, ...) {
    va_list arg;
    va_start(arg, option);
    if (option == CURLOPT_TIMEOUT_MS) {
        int val = va_arg(arg, int);
        curl->timeout_ms = val;
    }
    va_end(arg);
    return 0;
}

int main(void) {
    CURL session = { .url = "https://ztutor.io", .timeout_ms = 3000 };
    printf("[CURL] Initial timeout: %dms\n", session.timeout_ms);
    curl_easy_setopt(&session, CURLOPT_TIMEOUT_MS, 5000);
    if (session.timeout_ms == 5000) {
        printf("[CURL] State mutation successful: 5000ms\n");
    } else {
        printf("[CURL] Mutation failed: Timeout remains %dms\n", session.timeout_ms);
    }
    return 0;
}`,
				"curl_mock.h": `#ifndef CURL_MOCK_H
#define CURL_MOCK_H

typedef struct CURL {
    char url[256];
    int timeout_ms;
} CURL;

int curl_easy_setopt(CURL *curl, int option, ...);

#endif`,
			},
			wantOutput: "[CURL] Initial timeout: 3000ms\n[CURL] State mutation successful: 5000ms\n",
		},
		{
			name: "15-capstone-redis",
			files: map[string]string{
				"main.c": `#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "include/server.h"

int processMultibulkBuffer(client *c) {
    char *newline = NULL;
    int pos = 0;

    if (c->multibulklen == 0) {
        if (c->querybuf[c->qb_pos] != '*') {
            printf("[REDIS-CORE] Protocol Error: Expected '*'\n");
            return C_ERR;
        }

        newline = strchr(c->querybuf + c->qb_pos, '\r');

        if (newline == NULL) {
            return C_ERR;
        }

        long long ll = strtol((c->querybuf + c->qb_pos + 1), NULL, 10);
        c->multibulklen = (int)ll;

        pos = (newline - (c->querybuf + c->qb_pos));
        c->qb_pos += pos + 2;

        printf("[REDIS-CORE] Parsed Multibulk header: %d elements.\n", c->multibulklen);
        return C_OK;
    }

    return C_OK;
}

int main(void) {
    char raw_socket_data[] = "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$7\r\nmyval\r\n";
    client c;
    c.querybuf = raw_socket_data;
    c.qb_pos = 0;
    c.reqtype = PROTO_REQ_MULTIBULK;
    c.multibulklen = 0;

    if (processMultibulkBuffer(&c) == C_OK) {
        printf("[DEBUG] Next unparsed character is: '%c'\n", c.querybuf[c.qb_pos]);
    } else {
        printf("[DEBUG] Parser failed or aborted.\n");
    }

    return 0;
}`,
				"include/server.h": `#ifndef REDIS_SERVER_H
#define REDIS_SERVER_H

#include <stddef.h>

#define C_OK 0
#define C_ERR -1
#define PROTO_REQ_MULTIBULK 2

typedef char * sds;

typedef struct client {
    sds querybuf;
    size_t qb_pos;
    int reqtype;
    int multibulklen;
    long long bulklen;
} client;

#endif`,
			},
			wantOutput: "[REDIS-CORE] Parsed Multibulk header: 3 elements.\n[DEBUG] Next unparsed character is: '$'\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Run(cLang(), tt.files, "", tt.stdin, nil, nil)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if result.Error != "" {
				t.Fatalf("result.Error = %q", result.Error)
			}
			if strings.TrimSpace(result.Output) != strings.TrimSpace(tt.wantOutput) {
				t.Errorf("output mismatch:\ngot:\n%s\nwant:\n%s", result.Output, tt.wantOutput)
			}
		})
	}
}

func TestReferenceSolutions_CopiesHeadersFromCourse(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	// Load actual course lesson 02 (modern integers) to verify we can
	// include the custom sqliteInt.h header and compile.
	courseDir := filepath.Join("..", "..", "courses", "c-programming", "lessons", "02-modern-integers")
	headerPath := filepath.Join(courseDir, "sqliteInt.h")

	headerData, err := os.ReadFile(headerPath)
	if err != nil {
		t.Skipf("course lesson 02 not found: %v", err)
	}

	files := map[string]string{
		"main.c": `#include <stdio.h>
#include <stdint.h>

// sqlite_int64 and sqlite_uint64 are normally from sqlite3.h.
typedef int64_t sqlite_int64;
typedef uint64_t sqlite_uint64;

#include "sqliteInt.h"

void parse_sqlite_header(void) {
    unsigned char raw_bytes[100] = {0};
    raw_bytes[18] = 2;
    raw_bytes[20] = 0x00;
    raw_bytes[21] = 0x10;
    raw_bytes[24] = 0x00;
    raw_bytes[27] = 0x2A;

    u8 write_version = raw_bytes[18];
    u16 reserved_space = (raw_bytes[20] << 8) | raw_bytes[21];
    u32 file_change_counter = raw_bytes[27];

    printf("Write Version: %d (Size: %zu byte)\n", write_version, sizeof(write_version));
    printf("Reserved Space: %d (Size: %zu bytes)\n", reserved_space, sizeof(reserved_space));
    printf("Change Counter: %d (Size: %zu bytes)\n", file_change_counter, sizeof(file_change_counter));
}

int main(void) {
    parse_sqlite_header();
    return 0;
}`,
		"sqliteInt.h": string(headerData),
	}

	result, err := Run(cLang(), files, "", "", nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}

	want := "Write Version: 2 (Size: 1 byte)\nReserved Space: 16 (Size: 2 bytes)\nChange Counter: 42 (Size: 4 bytes)\n"
	if strings.TrimSpace(result.Output) != strings.TrimSpace(want) {
		t.Errorf("output mismatch:\ngot:\n%s\nwant:\n%s", result.Output, want)
	}
}

func TestReferenceSolutions_Lesson14MultiFileMake(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	if !hasMake() {
		t.Skip("make not available")
	}

	// Complete Makefile (all TODOs filled in) plus server.c and zmalloc.c.
	files := map[string]string{
		"server.c": `#include <stdio.h>
extern void zmalloc_init(void);
int main(void) {
    zmalloc_init();
    printf("[REDIS-MOCK] Server initialized and listening...\n");
    return 0;
}`,
		"zmalloc.c": `#include <stdio.h>
void zmalloc_init(void) {
    printf("[REDIS-MOCK] Memory allocator (zmalloc) booted.\n");
}`,
		"Makefile": `STD=-std=c11 -pedantic
WARN=-Wall -W -Wno-missing-field-initializers
OPT=-O3
FINAL_CFLAGS=$(STD) $(WARN) $(OPT)
REDIS_CC=$(CC) $(FINAL_CFLAGS)
REDIS_LD=$(CC) $(FINAL_CFLAGS)
REDIS_SERVER_NAME=redis-server
REDIS_SERVER_OBJ=server.o zmalloc.o

all: $(REDIS_SERVER_NAME)

$(REDIS_SERVER_NAME): $(REDIS_SERVER_OBJ)
	$(REDIS_LD) -o $@ $^

%.o: %.c
	$(REDIS_CC) -MMD -o $@ -c $<

clean:
	rm -rf $(REDIS_SERVER_NAME) *.o *.d

.PHONY: all clean
`,
	}

	result, err := Run(cLang(), files, "make", "", nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if !strings.Contains(result.Stdout, "[REDIS-MOCK] Memory allocator (zmalloc) booted.") {
		t.Errorf("stdout missing zmalloc boot, got: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "[REDIS-MOCK] Server initialized and listening...") {
		t.Errorf("stdout missing server init, got: %s", result.Stdout)
	}
}
