package main

import (
	"bufio"
	"fmt"
	"os"

	scpi "github.com/Nine-Fives/go-scpi-parser"
)

const (
	scpiInputBufferLength = 256
	scpiIDN1              = "MANUFACTURE"
	scpiIDN2              = "INSTR2013"
	scpiIDN3              = ""
	scpiIDN4              = "01-02"
)

// DMM command handlers

func dmmMeasureVoltageDcQ(ctx *scpi.Context) scpi.Result {
	fmt.Fprintf(os.Stderr, "meas:volt:dc\r\n")

	param1, err := ctx.ParamDouble(false)
	if err == nil {
		fmt.Fprintf(os.Stderr, "\tP1=%g\r\n", param1)
	}

	param2, err := ctx.ParamDouble(false)
	if err == nil {
		fmt.Fprintf(os.Stderr, "\tP2=%g\r\n", param2)
	}

	ctx.ResultDouble(0)
	return scpi.ResOK
}

func dmmMeasureVoltageAcQ(ctx *scpi.Context) scpi.Result {
	fmt.Fprintf(os.Stderr, "meas:volt:ac\r\n")

	param1, err := ctx.ParamDouble(false)
	if err == nil {
		fmt.Fprintf(os.Stderr, "\tP1=%g\r\n", param1)
	}

	param2, err := ctx.ParamDouble(false)
	if err == nil {
		fmt.Fprintf(os.Stderr, "\tP2=%g\r\n", param2)
	}

	ctx.ResultDouble(0)
	return scpi.ResOK
}

func dmmConfigureVoltageDc(ctx *scpi.Context) scpi.Result {
	fmt.Fprintf(os.Stderr, "conf:volt:dc\r\n")

	param1, err := ctx.ParamDouble(true)
	if err != nil {
		return scpi.ResErr
	}
	fmt.Fprintf(os.Stderr, "\tP1=%f\r\n", param1)

	param2, err := ctx.ParamDouble(false)
	if err == nil {
		fmt.Fprintf(os.Stderr, "\tP2=%f\r\n", param2)
	}

	return scpi.ResOK
}

// Test command handlers

func testBool(ctx *scpi.Context) scpi.Result {
	fmt.Fprintf(os.Stderr, "TEST:BOOL\r\n")

	param1, err := ctx.ParamBool(true)
	if err != nil {
		return scpi.ResErr
	}

	fmt.Fprintf(os.Stderr, "\tP1=%t\r\n", param1)
	return scpi.ResOK
}

var triggerSource = []scpi.ChoiceDef{
	{Name: "BUS", Tag: 5},
	{Name: "IMMediate", Tag: 6},
	{Name: "EXTernal", Tag: 7},
}

func testChoiceQ(ctx *scpi.Context) scpi.Result {
	param, err := ctx.ParamChoice(triggerSource, true)
	if err != nil {
		return scpi.ResErr
	}

	// Find the name for the chosen tag
	for _, choice := range triggerSource {
		if choice.Tag == param {
			fmt.Fprintf(os.Stderr, "\tP1=%s (%d)\r\n", choice.Name, param)
			break
		}
	}

	ctx.ResultInt32(param)
	return scpi.ResOK
}

func testNumbers(ctx *scpi.Context) scpi.Result {
	numbers := ctx.CommandNumbers(2, 1)
	fmt.Fprintf(os.Stderr, "TEST numbers %d %d\r\n", numbers[0], numbers[1])
	return scpi.ResOK
}

func testText(ctx *scpi.Context) scpi.Result {
	text, err := ctx.ParamString(false)
	if err != nil {
		text = ""
	}

	fmt.Fprintf(os.Stderr, "TEXT: ***%s***\r\n", text)
	return scpi.ResOK
}

func testArbQ(ctx *scpi.Context) scpi.Result {
	data, err := ctx.ParamArbitraryBlock(false)
	if err == nil && data != nil {
		ctx.ResultArbitraryBlock(data)
	}
	return scpi.ResOK
}

func testChanlst(ctx *scpi.Context) scpi.Result {
	type channelValue struct {
		row, col int32
	}

	entries, err := ctx.ParamChannelList(true)
	if err != nil {
		return scpi.ResErr
	}

	var array []channelValue

	for _, entry := range entries {
		if !entry.IsRange {
			cv := channelValue{row: entry.From[0]}
			if entry.Dimensions >= 2 {
				cv.col = entry.From[1]
			}
			array = append(array, cv)
		} else {
			// Determine row direction
			dirRow := int32(1)
			if entry.From[0] > entry.To[0] {
				dirRow = -1
			}

			for n := entry.From[0]; ; n += dirRow {
				if entry.Dimensions >= 2 {
					// 2D range: iterate columns
					dirCol := int32(1)
					if entry.From[1] > entry.To[1] {
						dirCol = -1
					}
					for m := entry.From[1]; ; m += dirCol {
						array = append(array, channelValue{row: n, col: m})
						if m == entry.To[1] {
							break
						}
					}
				} else {
					// 1D range
					array = append(array, channelValue{row: n, col: 0})
				}
				if n == entry.To[0] {
					break
				}
			}
		}
	}

	fmt.Fprintf(os.Stderr, "TEST_Chanlst: ")
	for _, cv := range array {
		fmt.Fprintf(os.Stderr, "%d!%d, ", cv.row, cv.col)
	}
	fmt.Fprintf(os.Stderr, "\r\n")
	return scpi.ResOK
}

// IEEE 488.2 command handlers

func coreCls(ctx *scpi.Context) scpi.Result {
	fmt.Fprintf(os.Stderr, "**CLS\r\n")
	return scpi.ResOK
}

func coreEse(ctx *scpi.Context) scpi.Result {
	// Stub: register operations not supported
	return scpi.ResOK
}

func coreEseQ(ctx *scpi.Context) scpi.Result {
	ctx.ResultInt32(0)
	return scpi.ResOK
}

func coreEsrQ(ctx *scpi.Context) scpi.Result {
	ctx.ResultInt32(0)
	return scpi.ResOK
}

func coreIdnQ(ctx *scpi.Context) scpi.Result {
	ctx.ResultText(scpiIDN1)
	ctx.ResultText(scpiIDN2)
	ctx.ResultText(scpiIDN3)
	ctx.ResultText(scpiIDN4)
	return scpi.ResOK
}

func coreOpc(ctx *scpi.Context) scpi.Result {
	return scpi.ResOK
}

func coreOpcQ(ctx *scpi.Context) scpi.Result {
	ctx.ResultInt32(1)
	return scpi.ResOK
}

func coreRst(ctx *scpi.Context) scpi.Result {
	fmt.Fprintf(os.Stderr, "**Reset\r\n")
	return scpi.ResOK
}

func coreSre(ctx *scpi.Context) scpi.Result {
	return scpi.ResOK
}

func coreSreQ(ctx *scpi.Context) scpi.Result {
	ctx.ResultInt32(0)
	return scpi.ResOK
}

func coreStbQ(ctx *scpi.Context) scpi.Result {
	ctx.ResultInt32(0)
	return scpi.ResOK
}

func coreTstQ(ctx *scpi.Context) scpi.Result {
	ctx.ResultInt32(0)
	return scpi.ResOK
}

func coreWai(ctx *scpi.Context) scpi.Result {
	return scpi.ResOK
}

// Required SCPI command handlers

func systemErrorNextQ(ctx *scpi.Context) scpi.Result {
	err := ctx.ErrorPop()
	if err == nil {
		ctx.ResultInt32(0)
		ctx.ResultText("No error")
	} else {
		ctx.ResultInt32(int32(err.Code))
		ctx.ResultText(err.Info)
	}
	return scpi.ResOK
}

func systemErrorCountQ(ctx *scpi.Context) scpi.Result {
	// Note: error queue count not exposed by Go library; return 0
	ctx.ResultInt32(0)
	return scpi.ResOK
}

func systemVersionQ(ctx *scpi.Context) scpi.Result {
	ctx.ResultText("1999.0")
	return scpi.ResOK
}

// Status command handlers (stubs)

func statusQuestionableEventQ(ctx *scpi.Context) scpi.Result {
	ctx.ResultInt32(0)
	return scpi.ResOK
}

func statusQuestionableEnable(ctx *scpi.Context) scpi.Result {
	return scpi.ResOK
}

func statusQuestionableEnableQ(ctx *scpi.Context) scpi.Result {
	ctx.ResultInt32(0)
	return scpi.ResOK
}

func statusPreset(ctx *scpi.Context) scpi.Result {
	return scpi.ResOK
}

// Stub query handler for unimplemented measurement commands
func stubQ(ctx *scpi.Context) scpi.Result {
	return scpi.ResOK
}

func systemCommTcpipControlQ(ctx *scpi.Context) scpi.Result {
	return scpi.ResErr
}

var scpiCommands = []*scpi.Command{
	// IEEE Mandated Commands (SCPI std V1999.0 4.1.1)
	{Pattern: "*CLS", Callback: coreCls},
	{Pattern: "*ESE", Callback: coreEse},
	{Pattern: "*ESE?", Callback: coreEseQ},
	{Pattern: "*ESR?", Callback: coreEsrQ},
	{Pattern: "*IDN?", Callback: coreIdnQ},
	{Pattern: "*OPC", Callback: coreOpc},
	{Pattern: "*OPC?", Callback: coreOpcQ},
	{Pattern: "*RST", Callback: coreRst},
	{Pattern: "*SRE", Callback: coreSre},
	{Pattern: "*SRE?", Callback: coreSreQ},
	{Pattern: "*STB?", Callback: coreStbQ},
	{Pattern: "*TST?", Callback: coreTstQ},
	{Pattern: "*WAI", Callback: coreWai},

	// Required SCPI commands (SCPI std V1999.0 4.2.1)
	{Pattern: "SYSTem:ERRor[:NEXT]?", Callback: systemErrorNextQ},
	{Pattern: "SYSTem:ERRor:COUNt?", Callback: systemErrorCountQ},
	{Pattern: "SYSTem:VERSion?", Callback: systemVersionQ},

	// Status commands
	{Pattern: "STATus:QUEStionable[:EVENt]?", Callback: statusQuestionableEventQ},
	{Pattern: "STATus:QUEStionable:ENABle", Callback: statusQuestionableEnable},
	{Pattern: "STATus:QUEStionable:ENABle?", Callback: statusQuestionableEnableQ},
	{Pattern: "STATus:PRESet", Callback: statusPreset},

	// DMM
	{Pattern: "MEASure:VOLTage:DC?", Callback: dmmMeasureVoltageDcQ},
	{Pattern: "CONFigure:VOLTage:DC", Callback: dmmConfigureVoltageDc},
	{Pattern: "MEASure:VOLTage:DC:RATio?", Callback: stubQ},
	{Pattern: "MEASure:VOLTage:AC?", Callback: dmmMeasureVoltageAcQ},
	{Pattern: "MEASure:CURRent:DC?", Callback: stubQ},
	{Pattern: "MEASure:CURRent:AC?", Callback: stubQ},
	{Pattern: "MEASure:RESistance?", Callback: stubQ},
	{Pattern: "MEASure:FRESistance?", Callback: stubQ},
	{Pattern: "MEASure:FREQuency?", Callback: stubQ},
	{Pattern: "MEASure:PERiod?", Callback: stubQ},

	{Pattern: "SYSTem:COMMunication:TCPIP:CONTROL?", Callback: systemCommTcpipControlQ},

	// Test commands
	{Pattern: "TEST:BOOL", Callback: testBool},
	{Pattern: "TEST:CHOice?", Callback: testChoiceQ},
	{Pattern: "TEST#:NUMbers#", Callback: testNumbers},
	{Pattern: "TEST:TEXT", Callback: testText},
	{Pattern: "TEST:ARBitrary?", Callback: testArbQ},
	{Pattern: "TEST:CHANnellist", Callback: testChanlst},
}

func main() {
	iface := &scpi.Interface{
		Write: func(data []byte) (int, error) {
			return os.Stdout.Write(data)
		},
		Flush: func() error {
			return nil
		},
		OnError: func(err *scpi.Error) {
			fmt.Fprintf(os.Stderr, "**ERROR: %d, \"%s\"\r\n", err.Code, err.Info)
		},
		Reset: func() error {
			fmt.Fprintf(os.Stderr, "**Reset\r\n")
			return nil
		},
	}

	ctx := scpi.NewContext(scpiCommands, iface, scpiInputBufferLength)
	ctx.SetIDN(scpiIDN1, scpiIDN2, scpiIDN3, scpiIDN4)

	fmt.Printf("SCPI Interactive demo\r\n")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		ctx.Input([]byte(line))
	}
}
