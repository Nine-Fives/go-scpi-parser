package fuzz

/*
#cgo CFLAGS: -I${SRCDIR}/scpi-parser/libscpi/inc
#cgo CFLAGS: -I${SRCDIR}/scpi-parser/libscpi/src
#cgo CFLAGS: -I${SRCDIR}
#cgo LDFLAGS: -lm
#include "c_wrapper.h"
#include <stdlib.h>
*/
import "C"

// CParserInit initializes the C SCPI parser. Must be called once.
func CParserInit() {
	C.c_scpi_init()
}

// CParserInput feeds data to the C parser and returns the output string
// and the number of errors that occurred.
func CParserInput(data []byte) (string, int) {
	var outLen C.int
	var errCount C.int
	cData := C.CBytes(data)
	defer C.free(cData)

	result := C.c_scpi_input((*C.char)(cData), C.int(len(data)), &outLen, &errCount)
	return C.GoStringN(result, outLen), int(errCount)
}

// CParserReset resets the C parser state.
func CParserReset() {
	C.c_scpi_reset()
}
