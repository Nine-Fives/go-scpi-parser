// Unity build file: includes all libscpi source files so cgo compiles them
// as a single translation unit without needing a Makefile.
#include "scpi-parser/libscpi/src/error.c"
#include "scpi-parser/libscpi/src/expression.c"
#include "scpi-parser/libscpi/src/fifo.c"
#include "scpi-parser/libscpi/src/ieee488.c"
#include "scpi-parser/libscpi/src/lexer.c"
#include "scpi-parser/libscpi/src/minimal.c"
#include "scpi-parser/libscpi/src/parser.c"
#include "scpi-parser/libscpi/src/units.c"
#include "scpi-parser/libscpi/src/utils.c"
