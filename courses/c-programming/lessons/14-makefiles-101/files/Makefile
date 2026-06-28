# Authentic Redis-style compiler configurations
# Extracted directly from: redis/src/Makefile

STD=-std=c11 -pedantic
WARN=-Wall -W -Wno-missing-field-initializers
OPT=-O3
FINAL_CFLAGS=$(STD) $(WARN) $(OPT)

REDIS_CC=$(CC) $(FINAL_CFLAGS)
REDIS_LD=$(CC) $(FINAL_CFLAGS)

REDIS_SERVER_NAME=redis-server
REDIS_SERVER_OBJ=server.o zmalloc.o

all: $(REDIS_SERVER_NAME)
	@echo ""
	@echo "Hint: It's a good idea to run 'make test' ;)"
	@echo ""

# 1. TODO: Write the linking recipe using Automatic Variables.
# The Redis Makefile uses: $(REDIS_LD) -o $@ $^
$(REDIS_SERVER_NAME): $(REDIS_SERVER_OBJ)
	

# 2. TODO: Write the object compilation pattern rule.
# The Redis Makefile uses: $(REDIS_CC) -MMD -o $@ -c $<
%.o: %.c
	

# Clean up build artifacts
clean:
	rm -rf $(REDIS_SERVER_NAME) *.o *.d

# 3. TODO: Declare 'clean' as a PHONY target.
# Syntax: .PHONY: clean
