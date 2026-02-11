# go-scpi-parser

Go port of [j123b567/scpi-parser](https://github.com/j123b567/scpi-parser), rewritten from C to Go.

## Build, Vet, and Test

```sh
go build ./...
go vet ./...
go test -v ./...
```

## Differential Fuzzing

The `fuzz/` directory contains differential fuzz tests that compare this Go parser against the original C [j123b567/scpi-parser](https://github.com/j123b567/scpi-parser) via cgo. The C library is included as a git submodule at `fuzz/scpi-parser/`.

```sh
# After cloning, init the submodule
git submodule update --init

# Run seed corpus (quick check, requires C compiler)
go test -v -run='Fuzz.*' ./fuzz/

# Run the fuzzer (e.g. 5 minutes)
go test -fuzz=FuzzRawInput -fuzztime=5m ./fuzz/

# Other fuzz targets
go test -fuzz=FuzzInt32Param -fuzztime=2m ./fuzz/
go test -fuzz=FuzzDoubleParam -fuzztime=2m ./fuzz/
go test -fuzz=FuzzBoolParam -fuzztime=2m ./fuzz/
```

## Guidelines

- New features must include corresponding tests in `parser_test.go`.
- The `fuzz/` package requires a C compiler (cgo). The main package does not.
