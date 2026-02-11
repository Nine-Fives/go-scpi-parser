# go-scpi-parser

Go port of [j123b567/scpi-parser](https://github.com/j123b567/scpi-parser), rewritten from C to Go.

## Build, Vet, and Test

```sh
go build ./...
go vet ./...
go test -v ./...
```

## Guidelines

- New features must include corresponding tests in `parser_test.go`.
