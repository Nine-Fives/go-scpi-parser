# Contributing to go-scpi-parser

## Build and Test

```sh
go build ./...
go vet ./...
go test -v ./...
```

New features must include corresponding tests in `parser_test.go`.

## Differential Fuzzing

The `fuzz/` directory implements differential fuzzing that compares this Go parser against the original C [j123b567/scpi-parser](https://github.com/j123b567/scpi-parser). Both parsers receive the same SCPI input, and their outputs are compared to find behavioral divergences.

### Prerequisites

- A C compiler (gcc or clang) for cgo
- The git submodule must be initialized:

```sh
git submodule update --init
```

### How It Works

The fuzzer uses Go's `testing/fuzz` framework with cgo to wrap the C parser:

- **`fuzz/scpi-parser/`** -- Git submodule containing the original C library.
- **`fuzz/compile_libscpi.c`** -- Unity build file that `#include`s all C library source files, allowing cgo to compile them without a Makefile.
- **`fuzz/c_wrapper.h` / `fuzz/c_wrapper.c`** -- C shim that initializes the C parser with a fixed command set, feeds input via `SCPI_Input()`, and captures output to a buffer. Includes echo-style callbacks (e.g., read an int32 parameter, write it back as a result).
- **`fuzz/c_wrapper.go`** -- cgo bridge exposing `CParserInit()`, `CParserInput()`, and `CParserReset()` to Go.
- **`fuzz/fuzz_helpers_test.go`** -- Sets up the Go parser with an identical command set and callback logic.
- **`fuzz/fuzz_test.go`** -- Fuzz targets and a `TestKnownDivergences` test documenting known behavioral differences.

Both parsers register the same commands with echo-style callbacks (read a parameter, write it back as a result). This tests the full round-trip: lexing, command matching, parameter extraction, and result formatting.

### Fuzz Targets

| Target | Input | What it tests |
|--------|-------|---------------|
| `FuzzRawInput` | Arbitrary bytes | Broadest target: command matching, error handling, edge cases |
| `FuzzInt32Param` | `int32` values | Integer parameter parsing and formatting |
| `FuzzDoubleParam` | `float64` values | Floating-point parsing with numeric tolerance comparison |
| `FuzzBoolParam` | Arbitrary strings | Boolean parameter acceptance/rejection |

### Running the Fuzzer

```sh
# Run all seed corpora (quick verification)
go test -v -run='Fuzz.*' ./fuzz/

# Run a specific fuzz target
go test -fuzz=FuzzRawInput -fuzztime=5m ./fuzz/

# Run with AddressSanitizer for C memory safety
CGO_CFLAGS="-fsanitize=address" CGO_LDFLAGS="-fsanitize=address" \
  go test -fuzz=FuzzRawInput -fuzztime=5m ./fuzz/

# View known divergences between Go and C parsers
go test -v -run=TestKnownDivergences ./fuzz/
```

### Output Comparison

The comparison logic handles known formatting differences:

- **Line endings**: The C library defaults to `\r\n`; Go uses `\n`. Outputs are normalized before comparison.
- **Float formatting**: C uses `%.15lg` for doubles; Go uses `%g`. Outputs are compared numerically with a relative tolerance of `1e-14`.
- **Multi-value results**: Comma-separated outputs are compared element-by-element.

### Adding New Fuzz Commands

To extend the fuzzer with new command types:

1. Add the C callback in `fuzz/c_wrapper.c` and register it in the `commands[]` table.
2. Add the matching Go callback in `fuzz/fuzz_helpers_test.go` inside `runGoParser()`.
3. Add seed corpus entries in the relevant fuzz target in `fuzz/fuzz_test.go`.

Both sides must use identical command patterns and parameter/result types.
