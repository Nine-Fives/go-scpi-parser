#ifndef C_WRAPPER_H
#define C_WRAPPER_H

#include <stddef.h>

// Initialize the C parser context. Call once.
void c_scpi_init(void);

// Feed data to the C parser and return the captured output.
// The returned pointer is to a static buffer valid until the next call.
// out_len receives the length of the output.
// err_count receives the number of errors that occurred.
const char* c_scpi_input(const char* data, int len, int* out_len, int* err_count);

// Reset the C parser state between fuzz iterations.
void c_scpi_reset(void);

#endif
