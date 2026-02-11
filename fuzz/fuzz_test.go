package fuzz

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
)

var initOnce sync.Once

// cMu serializes access to the C parser (uses static globals).
var cMu sync.Mutex

func ensureCInit() {
	initOnce.Do(func() {
		CParserInit()
	})
}

// normalizeOutput handles known formatting differences between C and Go.
func normalizeOutput(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n ")
	return s
}

// outputsEquivalent compares parser outputs, allowing for numeric
// formatting differences (C uses "%.15lg" for doubles, Go uses "%g").
func outputsEquivalent(cOut, goOut string) bool {
	if cOut == goOut {
		return true
	}

	// Try comma-separated multi-result comparison
	cParts := strings.Split(cOut, ",")
	goParts := strings.Split(goOut, ",")
	if len(cParts) != len(goParts) {
		return false
	}
	if len(cParts) > 1 {
		for i := range cParts {
			if !outputsEquivalent(strings.TrimSpace(cParts[i]), strings.TrimSpace(goParts[i])) {
				return false
			}
		}
		return true
	}

	// Try numeric comparison for floating-point formatting differences
	cVal, cErr := strconv.ParseFloat(cOut, 64)
	goVal, goErr := strconv.ParseFloat(goOut, 64)
	if cErr == nil && goErr == nil {
		if cVal == goVal {
			return true
		}
		if cVal != 0 && math.Abs((cVal-goVal)/cVal) < 1e-14 {
			return true
		}
	}

	return false
}

// runBothParsers feeds input to both C and Go parsers, returning normalized outputs.
func runBothParsers(data []byte) (cOut string, cErrors int, goOut string, goErrors int) {
	cMu.Lock()
	CParserReset()
	cRaw, cErrs := CParserInput(data)
	cMu.Unlock()

	goResult := runGoParser(data)

	return normalizeOutput(cRaw), cErrs, normalizeOutput(goResult.output), goResult.errCount
}

// TestFormerDivergences verifies that previously known behavioral differences
// between the Go and C parsers have been resolved.
func TestFormerDivergences(t *testing.T) {
	ensureCInit()

	tests := []struct {
		name  string
		input string
		desc  string
	}{
		{
			name:  "choice short form",
			input: "TEST:CHOI? LOW\n",
			desc:  "Both parsers should reject partial short form CHOI for CHOice",
		},
		{
			name:  "int32 max value",
			input: "TEST:INT32 2147483647\n",
			desc:  "Both parsers should correctly output 2147483647",
		},
		{
			name:  "compound command without colon",
			input: "TEST:INT32 1;TEST:INT32 2\n",
			desc:  "Both parsers should require ;: for unrelated compound commands",
		},
		{
			name:  "empty newline",
			input: "\n",
			desc:  "Both parsers should silently ignore bare newlines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cOut, cErrors, goOut, goErrors := runBothParsers([]byte(tt.input))
			t.Logf("Description: %s", tt.desc)
			t.Logf("Input: %q", tt.input)
			t.Logf("C output:  %q (errors: %d)", cOut, cErrors)
			t.Logf("Go output: %q (errors: %d)", goOut, goErrors)

			if !outputsEquivalent(cOut, goOut) {
				t.Errorf("output mismatch: C=%q Go=%q", cOut, goOut)
			}

			cHadError := cErrors > 0
			goHadError := goErrors > 0
			if cHadError != goHadError {
				t.Errorf("error agreement mismatch: C errors=%d, Go errors=%d", cErrors, goErrors)
			}
		})
	}
}

// FuzzRawInput feeds arbitrary byte sequences to both parsers and
// compares their text output. This is the broadest fuzz target.
func FuzzRawInput(f *testing.F) {
	ensureCInit()

	seeds := []string{
		"TEST:INT32 42\n",
		"TEST:INT32 -100\n",
		"TEST:INT32 #HFF\n",
		"TEST:INT32 #B1010\n",
		"TEST:INT32 #Q77\n",
		"TEST:DOUB 3.14\n",
		"TEST:DOUB -1.5e2\n",
		"TEST:DOUB 0.0\n",
		"TEST:BOOL ON\n",
		"TEST:BOOL OFF\n",
		"TEST:BOOL 1\n",
		"TEST:BOOL 0\n",
		"TEST:TEXT 'hello world'\n",
		"TEST:TEXT \"quoted\"\n",
		"TEST:CHOICE? LOW\n",
		"TEST:CHOICE? MED\n",
		"TEST:CHOICE? HIGH\n",
		"TEST:ARB? #14abcd\n",
		"TEST:NOOP\n",
		"TEST:QUER?\n",
		"TEST1:NUM2\n",
		// Edge cases
		"TEST:INT32 0\n",
		"TEST:INT32 -2147483648\n",
		"TEST:DOUB 1e308\n",
		"TEST:DOUB -1e308\n",
		// Leading colon
		":TEST:INT32 99\n",
		// Short and long forms
		"TEST:DOUBLE 1.0\n",
		"TEST:DOUB 1.0\n",
		// Case variations
		"test:int32 5\n",
		"Test:Int32 5\n",
		// Invalid commands
		"INVALID:CMD\n",
		// Whitespace variations
		"TEST:INT32  42\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		if len(data) > 512 {
			data = data[:512]
		}
		// Ensure newline termination
		if data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}

		cOut, cErrors, goOut, goErrors := runBothParsers(data)

		if !outputsEquivalent(cOut, goOut) {
			t.Errorf("output mismatch for input %q\nC:  %q\nGo: %q",
				string(data), cOut, goOut)
		}

		// Compare error counts: both should agree on error vs success
		cHadError := cErrors > 0
		goHadError := goErrors > 0
		if cHadError != goHadError {
			t.Errorf("error agreement mismatch for input %q\nC errors: %d, Go errors: %d",
				string(data), cErrors, goErrors)
		}
	})
}

// FuzzInt32Param focuses on integer parameter parsing with structured input.
func FuzzInt32Param(f *testing.F) {
	ensureCInit()

	f.Add(int32(0))
	f.Add(int32(1))
	f.Add(int32(-1))
	f.Add(int32(-2147483648))
	f.Add(int32(255))
	f.Add(int32(42))
	f.Add(int32(-42))
	f.Add(int32(1000000))

	f.Fuzz(func(t *testing.T, val int32) {
		input := []byte(fmt.Sprintf("TEST:INT32 %d\n", val))

		cOut, _, goOut, _ := runBothParsers(input)

		if cOut != goOut {
			t.Errorf("int32 mismatch for val=%d\nC:  %q\nGo: %q",
				val, cOut, goOut)
		}
	})
}

// FuzzDoubleParam focuses on floating-point parameter parsing.
func FuzzDoubleParam(f *testing.F) {
	ensureCInit()

	f.Add(0.0)
	f.Add(1.0)
	f.Add(-1.0)
	f.Add(3.14159)
	f.Add(1e-10)
	f.Add(1e10)
	f.Add(-0.0)
	f.Add(0.001)
	f.Add(123456789.0)

	f.Fuzz(func(t *testing.T, val float64) {
		// Skip NaN and Inf
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return
		}
		// Skip values too large/small for reliable string round-trip
		if math.Abs(val) > 1e300 || (val != 0 && math.Abs(val) < 1e-300) {
			return
		}

		input := []byte(fmt.Sprintf("TEST:DOUB %g\n", val))

		cOut, _, goOut, _ := runBothParsers(input)

		if !outputsEquivalent(cOut, goOut) {
			t.Errorf("double mismatch for val=%v (input %q)\nC:  %q\nGo: %q",
				val, string(input), cOut, goOut)
		}
	})
}

// FuzzBoolParam focuses on boolean parameter parsing with raw string input.
func FuzzBoolParam(f *testing.F) {
	ensureCInit()

	seeds := []string{
		"ON", "OFF", "1", "0",
		"on", "off", "On", "Off",
		"TRUE", "FALSE", "true", "false",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, val string) {
		if len(val) > 64 {
			return
		}
		input := []byte(fmt.Sprintf("TEST:BOOL %s\n", val))

		cOut, cErrors, goOut, goErrors := runBothParsers(input)

		if !outputsEquivalent(cOut, goOut) {
			t.Errorf("bool mismatch for val=%q\nC:  %q\nGo: %q",
				val, cOut, goOut)
		}

		cHadError := cErrors > 0
		goHadError := goErrors > 0
		if cHadError != goHadError {
			t.Errorf("bool error mismatch for val=%q\nC errors: %d, Go errors: %d",
				val, cErrors, goErrors)
		}
	})
}
