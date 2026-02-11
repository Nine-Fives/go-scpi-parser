package main

import (
	"fmt"
	"os"
	"strings"

	scpi "github.com/Nine-Fives/go-scpi-parser"
)

var output strings.Builder

// Example SCPI command handlers
func handleIDN(ctx *scpi.Context) scpi.Result {
	// *IDN? query returns identification
	ctx.ResultText("MyCompany")
	ctx.ResultText("MyInstrument")
	ctx.ResultText("12345")
	ctx.ResultText("1.0.0")
	return scpi.ResOK
}

func handleMeasureVoltage(ctx *scpi.Context) scpi.Result {
	// MEAS:VOLT? query
	// Return a simulated voltage measurement
	ctx.ResultDouble(3.14159)
	return scpi.ResOK
}

func handleMeasureCurrent(ctx *scpi.Context) scpi.Result {
	// MEAS:CURR? query
	ctx.ResultDouble(0.125)
	return scpi.ResOK
}

func handleSourceVoltage(ctx *scpi.Context) scpi.Result {
	// SOUR:VOLT <value>
	voltage, err := ctx.ParamDouble(true)
	if err != nil {
		return scpi.ResErr
	}

	fmt.Printf("Setting voltage to: %f V\n", voltage)
	return scpi.ResOK
}

func handleSourceCurrent(ctx *scpi.Context) scpi.Result {
	// SOUR:CURR <value>
	current, err := ctx.ParamDouble(true)
	if err != nil {
		return scpi.ResErr
	}

	fmt.Printf("Setting current to: %f A\n", current)
	return scpi.ResOK
}

func handleOutput(ctx *scpi.Context) scpi.Result {
	// OUTP <bool>
	state, err := ctx.ParamBool(true)
	if err != nil {
		return scpi.ResErr
	}

	if state {
		fmt.Println("Output: ON")
	} else {
		fmt.Println("Output: OFF")
	}
	return scpi.ResOK
}

func handleOutputQuery(ctx *scpi.Context) scpi.Result {
	// OUTP? query
	// Return simulated output state
	ctx.ResultBool(true)
	return scpi.ResOK
}

func handleReset(ctx *scpi.Context) scpi.Result {
	// *RST command
	fmt.Println("Reset instrument")
	return scpi.ResOK
}

func handleClear(ctx *scpi.Context) scpi.Result {
	// *CLS command
	fmt.Println("Clear status")
	return scpi.ResOK
}

func handleSystemError(ctx *scpi.Context) scpi.Result {
	// SYST:ERR? query
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

func main() {
	// Define SCPI commands
	commands := []*scpi.Command{
		// IEEE 488.2 Common Commands
		{Pattern: "*IDN?", Callback: handleIDN},
		{Pattern: "*RST", Callback: handleReset},
		{Pattern: "*CLS", Callback: handleClear},

		// Measurement commands
		{Pattern: "MEASure:VOLTage[:DC]?", Callback: handleMeasureVoltage},
		{Pattern: "MEASure:CURRent[:DC]?", Callback: handleMeasureCurrent},

		// Source commands
		{Pattern: "SOURce:VOLTage", Callback: handleSourceVoltage},
		{Pattern: "SOURce:CURRent", Callback: handleSourceCurrent},

		// Output control
		{Pattern: "OUTPut?", Callback: handleOutputQuery},
		{Pattern: "OUTPut", Callback: handleOutput},

		// System commands
		{Pattern: "SYSTem:ERRor?", Callback: handleSystemError},
	}

	// Create SCPI interface
	iface := &scpi.Interface{
		Write: func(data []byte) (int, error) {
			output.Write(data)
			return len(data), nil
		},
		Flush: func() error {
			// Flush is automatic with strings.Builder
			return nil
		},
		OnError: func(err *scpi.Error) {
			fmt.Fprintf(os.Stderr, "SCPI Error %d: %s\n", err.Code, err.Info)
		},
	}

	// Create parser context
	ctx := scpi.NewContext(commands, iface, 256)
	ctx.SetIDN("MyCompany", "MyInstrument", "12345", "1.0.0")

	// Example commands to process
	testCommands := []string{
		"*IDN?",
		"MEAS:VOLT?",
		"MEAS:CURR?",
		"SOUR:VOLT 5.0",
		"SOUR:CURR 0.5",
		"OUTP ON",
		"OUTP?",
		"*RST",
		"SYST:ERR?",
	}

	fmt.Println("SCPI Parser Example")
	fmt.Println("===================")
	fmt.Println()

	for _, cmd := range testCommands {
		fmt.Printf("Command: %s\n", cmd)
		output.Reset()

		err := ctx.Input([]byte(cmd + "\n"))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		result := output.String()
		if result != "" {
			fmt.Printf("Response: %s", result)
		}
		fmt.Println()
	}

	// Test compound command
	fmt.Println("Testing compound command:")
	fmt.Println("Command: SOUR:VOLT 3.3; SOUR:CURR 0.1; OUTP ON")
	output.Reset()

	err := ctx.Input([]byte("SOUR:VOLT 3.3; SOUR:CURR 0.1; OUTP ON\n"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	result := output.String()
	if result != "" {
		fmt.Printf("Response: %s", result)
	}
	fmt.Println()

	// Test short form commands
	fmt.Println("Testing short form commands:")
	fmt.Println("Command: MEAS:VOLT?")
	output.Reset()

	err = ctx.Input([]byte("MEAS:VOLT?\n"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	result = output.String()
	if result != "" {
		fmt.Printf("Response: %s", result)
	}
	fmt.Println()
}
