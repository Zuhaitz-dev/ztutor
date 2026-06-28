#include <stdio.h>

int main(void) {
    long linect = 0;
    long wordct = 0;
    long charct = 0;
    int token = 0;
    int c;

    /*
     * Rebuild the core stdin loop of V7 wc.
     *
     * Historical rule for this lesson:
     * a word character is any byte where ' ' < c && c < 0177.
     *
     * The variable 'token' means "currently inside a word".
     */
    // TODO: Read stdin until EOF and compute linect, wordct, and charct.

    printf("%7ld %7ld %7ld\n", linect, wordct, charct);
    return 0;
}
