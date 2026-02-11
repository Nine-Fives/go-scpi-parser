# go-scpi-parser

![Build status](https://github.com/Nine-Fives/go-scpi-parser/actions/workflows/main.yml/badge.svg) [![Coverage Status](https://coveralls.io/repos/github/Nine-Fives/go-scpi-parser/badge.svg?branch=master)](https://coveralls.io/github/Nine-Fives/go-scpi-parser?branch=master)

A Go implementation of a SCPI (Standard Commands for Programmable Instruments) parser library, rewritten from Jan Breuer's original C library [j123b567/scpi-parser](https://github.com/j123b567/scpi-parser).

## Why not just use CGO?

A key feature of Go is the ability to cross-compile for different platforms in a single command. For example, you can cross-compile this module for Linux on ARMv7 as simply as:

```sh
GOOS=linux GOARCH=arm GOARM=7 go build
```

Without this Go rewrite, compiling Jan's original C scpi-parser would require a C cross-compiler for the target platform. For quick iteration on your test equipment, a pure Go implementation is superior.


## Documentation

Documentation is available at [ninefives.github.io/go-scpi-parser](https://ninefives.github.io/go-scpi-parser).

## Installation

```sh
go get github.com/Nine-Fives/go-scpi-parser
```

## Usage

```go
package main

import (
	"strings"

	scpi "github.com/Nine-Fives/go-scpi-parser"
)

func main() {
	var output strings.Builder

	commands := []*scpi.Command{
		{Pattern: "*IDN?", Callback: func(ctx *scpi.Context) scpi.Result {
			ctx.ResultText("MyCompany")
			ctx.ResultText("MyInstrument")
			ctx.ResultText("12345")
			ctx.ResultText("1.0.0")
			return scpi.ResOK
		}},
		{Pattern: "MEASure:VOLTage[:DC]?", Callback: func(ctx *scpi.Context) scpi.Result {
			ctx.ResultDouble(3.14159)
			return scpi.ResOK
		}},
	}

	iface := &scpi.Interface{
		Write: func(data []byte) (int, error) {
			return output.Write(data)
		},
	}

	ctx := scpi.NewContext(commands, iface, 256)
	ctx.Input([]byte("*IDN?\n"))
	// output.String() => "MyCompany","MyInstrument","12345","1.0.0"
}
```

See [examples/main.go](example/main.go) for a more complete example. To run it:

```sh
go run examples/main.go
```

## About

[SCPI](http://en.wikipedia.org/wiki/Standard_Commands_for_Programmable_Instruments) Parser library provides parsing ability of SCPI commands on the **instrument side**. Commands are defined by patterns, e.g. `"STATus:QUEStionable:EVENt?"`.

Source code is published under the open source BSD 2-Clause License.

This library is based on the following standards:

* [SCPI-99](https://www.ivifoundation.org/downloads/SCPI/scpi-99.pdf)
* [IEEE 488.2-2004](http://dx.doi.org/10.1109/IEEESTD.2004.95390)

**SCPI version compliance**
<table>
<tr><td>SCPI version<td>v1999.0</tr>
</table>

**Supported command patterns**
<table>
<tr><th>Feature<th>Pattern example</tr>
<tr><td>Short and long form<td><code>MEASure</code> means <code>MEAS</code> or <code>MEASURE</code> command</tr>
<tr><td>Common command<td><code>*CLS</code></td>
<tr><td>Compound command<td><code>CONFigure:VOLTage</code><tr>
<tr><td>Query command<td><code>MEASure:VOLTage?</code>, <code>*IDN?</code></tr>
<tr><td>Optional keywords<td><code>MEASure:VOLTage[:DC]?</code></tr>
<tr><td>Numeric keyword suffix<br>Multiple identical capabilities<td><code>OUTput#:FREQuency</code></tr>
</table>

**Supported parameter types**
<table>
<tr><th>Type<th>Example</tr>
<tr><td>Decimal<td><code>10</code>, <code>10.5</code></tr>
<tr><td>Decimal with suffix<td><code>-5.5 V</code>, <code>1.5 KOHM</code></tr>
<tr><td>Hexadecimal<td><code>#HFF</code></tr>
<tr><td>Octal<td><code>#Q77</code></tr>
<tr><td>Binary<td><code>#B11</code></tr>
<tr><td>String<td><code>"text"</code>, <code>'text'</code></tr>
<tr><td>Arbitrary block<td><code>#12AB</code></tr>
<tr><td>Program expression<td><code>(1)</code></tr>
<tr><td>Numeric list<td><code>(1,2:50,80)</code></tr>
<tr><td>Channel list<td><code>(@1!2:3!4,5!6)</code></tr>
<tr><td>Character data<td><code>MINimum</code>, <code>DEFault</code>, <code>INFinity</code></tr>
</table>

## License

BSD 2-Clause — see [LICENSE](LICENSE) for details.
