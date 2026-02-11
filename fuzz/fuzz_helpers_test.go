package fuzz

import (
	"strings"

	scpi "github.com/Nine-Fives/go-scpi-parser"
)

var testChoices = []scpi.ChoiceDef{
	{Name: "LOW", Tag: 0},
	{Name: "MEDium", Tag: 1},
	{Name: "HIGH", Tag: 2},
}

type goParserResult struct {
	output   string
	errCount int
}

func runGoParser(data []byte) goParserResult {
	var output strings.Builder
	errCount := 0

	commands := []*scpi.Command{
		{Pattern: "TEST:INT32", Callback: func(ctx *scpi.Context) scpi.Result {
			val, err := ctx.ParamInt32(true)
			if err != nil {
				return scpi.ResErr
			}
			ctx.ResultInt32(val)
			return scpi.ResOK
		}},
		{Pattern: "TEST:INT32?", Callback: func(ctx *scpi.Context) scpi.Result {
			val, err := ctx.ParamInt32(true)
			if err != nil {
				return scpi.ResErr
			}
			ctx.ResultInt32(val)
			return scpi.ResOK
		}},
		{Pattern: "TEST:DOUBle", Callback: func(ctx *scpi.Context) scpi.Result {
			val, err := ctx.ParamDouble(true)
			if err != nil {
				return scpi.ResErr
			}
			ctx.ResultDouble(val)
			return scpi.ResOK
		}},
		{Pattern: "TEST:DOUBle?", Callback: func(ctx *scpi.Context) scpi.Result {
			val, err := ctx.ParamDouble(true)
			if err != nil {
				return scpi.ResErr
			}
			ctx.ResultDouble(val)
			return scpi.ResOK
		}},
		{Pattern: "TEST:BOOL", Callback: func(ctx *scpi.Context) scpi.Result {
			val, err := ctx.ParamBool(true)
			if err != nil {
				return scpi.ResErr
			}
			ctx.ResultBool(val)
			return scpi.ResOK
		}},
		{Pattern: "TEST:BOOL?", Callback: func(ctx *scpi.Context) scpi.Result {
			val, err := ctx.ParamBool(true)
			if err != nil {
				return scpi.ResErr
			}
			ctx.ResultBool(val)
			return scpi.ResOK
		}},
		{Pattern: "TEST:TEXT", Callback: func(ctx *scpi.Context) scpi.Result {
			val, err := ctx.ParamString(true)
			if err != nil {
				return scpi.ResErr
			}
			ctx.ResultText(val)
			return scpi.ResOK
		}},
		{Pattern: "TEST:TEXT?", Callback: func(ctx *scpi.Context) scpi.Result {
			val, err := ctx.ParamString(true)
			if err != nil {
				return scpi.ResErr
			}
			ctx.ResultText(val)
			return scpi.ResOK
		}},
		{Pattern: "TEST:CHOice?", Callback: func(ctx *scpi.Context) scpi.Result {
			val, err := ctx.ParamChoice(testChoices, true)
			if err != nil {
				return scpi.ResErr
			}
			ctx.ResultInt32(val)
			return scpi.ResOK
		}},
		{Pattern: "TEST:ARBitrary?", Callback: func(ctx *scpi.Context) scpi.Result {
			data, err := ctx.ParamArbitraryBlock(true)
			if err != nil {
				return scpi.ResErr
			}
			ctx.ResultArbitraryBlock(data)
			return scpi.ResOK
		}},
		{Pattern: "TEST:NOOP", Callback: func(ctx *scpi.Context) scpi.Result {
			return scpi.ResOK
		}},
		{Pattern: "TEST:QUERy?", Callback: func(ctx *scpi.Context) scpi.Result {
			ctx.ResultInt32(42)
			ctx.ResultDouble(3.14)
			ctx.ResultText("hello")
			return scpi.ResOK
		}},
		{Pattern: "TEST#:NUMbers#", Callback: func(ctx *scpi.Context) scpi.Result {
			return scpi.ResOK
		}},
	}

	iface := &scpi.Interface{
		Write: func(data []byte) (int, error) {
			return output.Write(data)
		},
		Flush: func() error { return nil },
		OnError: func(err *scpi.Error) {
			errCount++
		},
	}

	ctx := scpi.NewContext(commands, iface, 256)
	ctx.SetIDN("FUZZ", "INST", "0", "1.0")
	ctx.Input(data)

	return goParserResult{
		output:   output.String(),
		errCount: errCount,
	}
}
