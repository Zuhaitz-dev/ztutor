/* SPDX-License-Identifier: GPL-2.0 */
#ifndef _LINUX_MINMAX_H
#define _LINUX_MINMAX_H

/*
 * min()/max()/clamp() macros must accomplish several things:
 *
 * - Avoid multiple evaluations of the arguments (so side-effects like
 * "x++" happen only once) when non-constant.
 */

/* Token pasting to dynamically generate < or > */
#define __cmp_op_min <
#define __cmp_op_max >

#define __cmp(op, x, y)    ((x) __cmp_op_##op (y) ? (x) : (y))

/*
 * Use these carefully: no type checking, and uses the arguments
 * multiple times. Use for obvious constants only.
 */
#define MIN(a, b) __cmp(min, a, b)

/*
 * 1. TODO: Implement SAFE_MIN modeled after the kernel's __cmp_once_unique.
 * Use the GCC Statement Expression ({ ... }) and typeof() to evaluate x and y 
 * exactly once into local variables _x and _y, then pass them to __cmp(min, _x, _y).
 */
#define SAFE_MIN(x, y) 

#endif /* _LINUX_MINMAX_H */
