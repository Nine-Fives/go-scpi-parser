package scpi

import (
	"strings"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"MEASure", "MEAS", true},
		{"MEASure", "MEASURE", true},
		{"MEASure", "MEASUR", true},
		{"MEASure", "MEA", false},
		{"MEASure", "MEASUREMENT", false},
		{"VOLTage", "VOLT", true},
		{"VOLTage", "VOLTAGE", true},
		{"CURRent", "CURR", true},
		{"CURRent", "CURRENT", true},
	}

	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.value)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
		}
	}
}

func TestMatchCommand(t *testing.T) {
	tests := []struct {
		pattern string
		header  string
		want    bool
	}{
		{"MEASure:VOLTage?", "MEAS:VOLT?", true},
		{"MEASure:VOLTage?", "MEASURE:VOLTAGE?", true},
		{"MEASure:VOLTage?", "MEAS:VOLT", true},
		{"SOURce:VOLTage", "SOUR:VOLT", true},
		{"SOURce:CURRent", "SOUR:CURR", true},
		{"*IDN?", "*IDN?", true},
		{"*RST", "*RST", true},
		{"OUTPut", "OUTP", true},
		{"OUTPut", "OUTPUT", true},
		{"MEASure:VOLTage?", "MEAS:CURR?", false},
	}

	for _, tt := range tests {
		got := matchCommand(tt.pattern, tt.header)
		if got != tt.want {
			t.Errorf("matchCommand(%q, %q) = %v, want %v", tt.pattern, tt.header, got, tt.want)
		}
	}
}

func TestLexDecimalNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123", "123"},
		{"-456", "-456"},
		{"+789", "+789"},
		{"3.14", "3.14"},
		{"-2.5", "-2.5"},
		{"1.23e4", "1.23e4"},
		{"5.6E-7", "5.6E-7"},
		{"-8.9e+2", "-8.9e+2"},
	}

	for _, tt := range tests {
		state := &lexState{
			buffer: []byte(tt.input),
			pos:    0,
			len:    len(tt.input),
		}

		tok, length := state.lexDecimalNumeric()
		if length == 0 {
			t.Errorf("lexDecimalNumeric(%q) failed to parse", tt.input)
			continue
		}

		got := string(tok.Data)
		if got != tt.want {
			t.Errorf("lexDecimalNumeric(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLexNondecimalNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  TokenType
	}{
		{"#HFF", TokenHexNum},
		{"#H123ABC", TokenHexNum},
		{"#Q777", TokenOctNum},
		{"#Q123", TokenOctNum},
		{"#B1010", TokenBinNum},
		{"#B11110000", TokenBinNum},
	}

	for _, tt := range tests {
		state := &lexState{
			buffer: []byte(tt.input),
			pos:    0,
			len:    len(tt.input),
		}

		tok, length := state.lexNondecimalNumeric()
		if length == 0 {
			t.Errorf("lexNondecimalNumeric(%q) failed to parse", tt.input)
			continue
		}

		if tok.Type != tt.want {
			t.Errorf("lexNondecimalNumeric(%q) type = %v, want %v", tt.input, tok.Type, tt.want)
		}
	}
}

func TestLexStringProgramData(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"hello"`, `"hello"`},
		{`'world'`, `'world'`},
		{`"test""quote"`, `"test""quote"`},
		{`'test''quote'`, `'test''quote'`},
	}

	for _, tt := range tests {
		state := &lexState{
			buffer: []byte(tt.input),
			pos:    0,
			len:    len(tt.input),
		}

		tok, length := state.lexStringProgramData()
		if length == 0 {
			t.Errorf("lexStringProgramData(%q) failed to parse", tt.input)
			continue
		}

		got := string(tok.Data)
		if got != tt.want {
			t.Errorf("lexStringProgramData(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseSimpleCommand(t *testing.T) {
	var output strings.Builder
	callCount := 0

	commands := []*Command{
		{
			Pattern: "*IDN?",
			Callback: func(ctx *Context) Result {
				callCount++
				ctx.ResultText("Test")
				return ResOK
			},
		},
	}

	iface := &Interface{
		Write: func(data []byte) (int, error) {
			output.Write(data)
			return len(data), nil
		},
	}

	ctx := NewContext(commands, iface, 256)

	err := ctx.Input([]byte("*IDN?\n"))
	if err != nil {
		t.Errorf("Parse failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Callback called %d times, want 1", callCount)
	}

	result := output.String()
	if !strings.Contains(result, "Test") {
		t.Errorf("Output %q does not contain 'Test'", result)
	}
}

func TestParseWithParameters(t *testing.T) {
	var receivedValue float64

	commands := []*Command{
		{
			Pattern: "SOURce:VOLTage",
			Callback: func(ctx *Context) Result {
				val, err := ctx.ParamDouble(true)
				if err != nil {
					return ResErr
				}
				receivedValue = val
				return ResOK
			},
		},
	}

	iface := &Interface{
		Write: func(data []byte) (int, error) {
			return len(data), nil
		},
	}

	ctx := NewContext(commands, iface, 256)

	err := ctx.Input([]byte("SOUR:VOLT 3.14\n"))
	if err != nil {
		t.Errorf("Parse failed: %v", err)
	}

	if receivedValue != 3.14 {
		t.Errorf("Received value = %f, want 3.14", receivedValue)
	}
}

func TestParseMultipleParameters(t *testing.T) {
	var values []int32

	commands := []*Command{
		{
			Pattern: "TEST:DATA",
			Callback: func(ctx *Context) Result {
				for {
					param, err := ctx.Parameter(false)
					if err != nil || param.Type == TokenUnknown {
						break
					}
					val, err := ctx.paramToInt32(param)
					if err != nil {
						break
					}
					values = append(values, val)
				}
				return ResOK
			},
		},
	}

	iface := &Interface{
		Write: func(data []byte) (int, error) {
			return len(data), nil
		},
	}

	ctx := NewContext(commands, iface, 256)

	err := ctx.Input([]byte("TEST:DATA 1,2,3,4,5\n"))
	if err != nil {
		t.Errorf("Parse failed: %v", err)
	}

	expected := []int32{1, 2, 3, 4, 5}
	if len(values) != len(expected) {
		t.Errorf("Received %d values, want %d", len(values), len(expected))
	}

	for i, v := range values {
		if v != expected[i] {
			t.Errorf("values[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestParseCompoundCommand(t *testing.T) {
	callCount := 0

	commands := []*Command{
		{
			Pattern: "CMD1",
			Callback: func(ctx *Context) Result {
				callCount++
				return ResOK
			},
		},
		{
			Pattern: "CMD2",
			Callback: func(ctx *Context) Result {
				callCount++
				return ResOK
			},
		},
	}

	iface := &Interface{
		Write: func(data []byte) (int, error) {
			return len(data), nil
		},
	}

	ctx := NewContext(commands, iface, 256)

	err := ctx.Input([]byte("CMD1; CMD2\n"))
	if err != nil {
		t.Errorf("Parse failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Callbacks called %d times, want 2", callCount)
	}
}

func TestCommandNumbers(t *testing.T) {
	tests := []struct {
		name         string
		header       string
		count        int
		defaultValue int32
		want         []int32
	}{
		{"both suffixes", "TEST1:NUMBERS2\n", 2, 1, []int32{1, 2}},
		{"no suffixes", "TEST:NUMBERS\n", 2, 1, []int32{1, 1}},
		{"first only", "TEST1:NUMBERS\n", 2, 1, []int32{1, 1}},
		{"second only", "TEST:NUMBERS2\n", 2, 1, []int32{1, 2}},
		{"large numbers", "TEST10:NUMBERS20\n", 2, 0, []int32{10, 20}},
		{"default zero", "TEST:NUMBERS\n", 2, 0, []int32{0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []int32

			commands := []*Command{
				{
					Pattern: "TEST#:NUMbers#",
					Callback: func(ctx *Context) Result {
						result = ctx.CommandNumbers(tt.count, tt.defaultValue)
						return ResOK
					},
				},
			}

			iface := &Interface{
				Write: func(data []byte) (int, error) {
					return len(data), nil
				},
			}

			ctx := NewContext(commands, iface, 256)

			err := ctx.Input([]byte(tt.header))
			if err != nil {
				t.Fatalf("Parse %q failed: %v", tt.header, err)
			}

			if len(result) != len(tt.want) {
				t.Fatalf("got %d numbers, want %d", len(result), len(tt.want))
			}

			for i, v := range result {
				if v != tt.want[i] {
					t.Errorf("result[%d] = %d, want %d", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestParamArbitraryBlock(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"4 bytes", "#14ABCD", "ABCD"},
		{"11 bytes", "#211hello world", "hello world"},
		{"empty", "#10", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []byte

			commands := []*Command{
				{
					Pattern: "TEST:ARB",
					Callback: func(ctx *Context) Result {
						data, err := ctx.ParamArbitraryBlock(true)
						if err != nil {
							t.Fatalf("ParamArbitraryBlock failed: %v", err)
						}
						result = data
						return ResOK
					},
				},
			}

			iface := &Interface{
				Write: func(data []byte) (int, error) {
					return len(data), nil
				},
			}

			ctx := NewContext(commands, iface, 256)
			err := ctx.Input([]byte("TEST:ARB " + tt.input + "\n"))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if string(result) != tt.want {
				t.Errorf("got %q, want %q", string(result), tt.want)
			}
		})
	}
}

func TestResultArbitraryBlock(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"4 bytes", "ABCD", "#14ABCD"},
		{"11 bytes", "hello world", "#211hello world"},
		{"empty", "", "#10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder

			commands := []*Command{
				{
					Pattern: "TEST:ARB?",
					Callback: func(ctx *Context) Result {
						ctx.ResultArbitraryBlock([]byte(tt.data))
						return ResOK
					},
				},
			}

			iface := &Interface{
				Write: func(data []byte) (int, error) {
					output.Write(data)
					return len(data), nil
				},
			}

			ctx := NewContext(commands, iface, 256)
			err := ctx.Input([]byte("TEST:ARB?\n"))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			got := output.String()
			if got != tt.want+"\n" {
				t.Errorf("got %q, want %q", got, tt.want+"\n")
			}
		})
	}
}

func TestArbitraryBlockRoundTrip(t *testing.T) {
	var output strings.Builder

	commands := []*Command{
		{
			Pattern: "TEST:ARB?",
			Callback: func(ctx *Context) Result {
				data, err := ctx.ParamArbitraryBlock(false)
				if err != nil || data == nil {
					return ResErr
				}
				ctx.ResultArbitraryBlock(data)
				return ResOK
			},
		},
	}

	iface := &Interface{
		Write: func(data []byte) (int, error) {
			output.Write(data)
			return len(data), nil
		},
	}

	ctx := NewContext(commands, iface, 256)
	err := ctx.Input([]byte("TEST:ARB? #15Hello\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	got := output.String()
	want := "#15Hello\n"
	if got != want {
		t.Errorf("round trip got %q, want %q", got, want)
	}
}

func TestParamChannelList(t *testing.T) {
	tests := []struct {
		name string
		input string
		want []ChannelListEntry
	}{
		{
			"single 1D",
			"(@1)",
			[]ChannelListEntry{
				{IsRange: false, From: []int32{1}, Dimensions: 1},
			},
		},
		{
			"single 2D",
			"(@1!2)",
			[]ChannelListEntry{
				{IsRange: false, From: []int32{1, 2}, Dimensions: 2},
			},
		},
		{
			"multiple 1D",
			"(@1,2,3)",
			[]ChannelListEntry{
				{IsRange: false, From: []int32{1}, Dimensions: 1},
				{IsRange: false, From: []int32{2}, Dimensions: 1},
				{IsRange: false, From: []int32{3}, Dimensions: 1},
			},
		},
		{
			"1D range",
			"(@1:3)",
			[]ChannelListEntry{
				{IsRange: true, From: []int32{1}, To: []int32{3}, Dimensions: 1},
			},
		},
		{
			"2D range",
			"(@1!1:3!2)",
			[]ChannelListEntry{
				{IsRange: true, From: []int32{1, 1}, To: []int32{3, 2}, Dimensions: 2},
			},
		},
		{
			"reverse 2D range",
			"(@3!1:1!3)",
			[]ChannelListEntry{
				{IsRange: true, From: []int32{3, 1}, To: []int32{1, 3}, Dimensions: 2},
			},
		},
		{
			"mixed entries",
			"(@1,2:4,5!1)",
			[]ChannelListEntry{
				{IsRange: false, From: []int32{1}, Dimensions: 1},
				{IsRange: true, From: []int32{2}, To: []int32{4}, Dimensions: 1},
				{IsRange: false, From: []int32{5, 1}, Dimensions: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []ChannelListEntry

			commands := []*Command{
				{
					Pattern: "TEST:CHAN",
					Callback: func(ctx *Context) Result {
						entries, err := ctx.ParamChannelList(true)
						if err != nil {
							t.Fatalf("ParamChannelList failed: %v", err)
						}
						result = entries
						return ResOK
					},
				},
			}

			iface := &Interface{
				Write: func(data []byte) (int, error) {
					return len(data), nil
				},
			}

			ctx := NewContext(commands, iface, 256)
			err := ctx.Input([]byte("TEST:CHAN " + tt.input + "\n"))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if len(result) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(result), len(tt.want))
			}

			for i, got := range result {
				want := tt.want[i]
				if got.IsRange != want.IsRange {
					t.Errorf("entry[%d].IsRange = %v, want %v", i, got.IsRange, want.IsRange)
				}
				if got.Dimensions != want.Dimensions {
					t.Errorf("entry[%d].Dimensions = %d, want %d", i, got.Dimensions, want.Dimensions)
				}
				if len(got.From) != len(want.From) {
					t.Errorf("entry[%d].From length = %d, want %d", i, len(got.From), len(want.From))
				} else {
					for j := range got.From {
						if got.From[j] != want.From[j] {
							t.Errorf("entry[%d].From[%d] = %d, want %d", i, j, got.From[j], want.From[j])
						}
					}
				}
				if want.IsRange {
					if len(got.To) != len(want.To) {
						t.Errorf("entry[%d].To length = %d, want %d", i, len(got.To), len(want.To))
					} else {
						for j := range got.To {
							if got.To[j] != want.To[j] {
								t.Errorf("entry[%d].To[%d] = %d, want %d", i, j, got.To[j], want.To[j])
							}
						}
					}
				}
			}
		})
	}
}

func TestParamBool(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ON", true},
		{"OFF", false},
		{"1", true},
		{"0", false},
	}

	for _, tt := range tests {
		var result bool

		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					val, err := ctx.ParamBool(true)
					if err != nil {
						return ResErr
					}
					result = val
					return ResOK
				},
			},
		}

		iface := &Interface{
			Write: func(data []byte) (int, error) {
				return len(data), nil
			},
		}

		ctx := NewContext(commands, iface, 256)

		err := ctx.Input([]byte("TEST " + tt.input + "\n"))
		if err != nil {
			t.Errorf("Parse 'TEST %s' failed: %v", tt.input, err)
			continue
		}

		if result != tt.want {
			t.Errorf("ParamBool(%q) = %v, want %v", tt.input, result, tt.want)
		}
	}
}

// =============================================================================
// Step 1: Context Management Functions
// =============================================================================

func TestSetIDN(t *testing.T) {
	var output strings.Builder

	commands := []*Command{
		{
			Pattern: "*IDN?",
			Callback: func(ctx *Context) Result {
				ctx.ResultText(ctx.idn[0] + "," + ctx.idn[1] + "," + ctx.idn[2] + "," + ctx.idn[3])
				return ResOK
			},
		},
	}

	iface := &Interface{
		Write: func(data []byte) (int, error) {
			output.Write(data)
			return len(data), nil
		},
	}

	ctx := NewContext(commands, iface, 256)
	ctx.SetIDN("ACME", "Model1", "SN123", "1.0")

	err := ctx.Input([]byte("*IDN?\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "ACME,Model1,SN123,1.0") {
		t.Errorf("IDN output %q does not contain expected string", result)
	}
}

func TestUserContext(t *testing.T) {
	ctx := NewContext(nil, nil, 256)

	if ctx.GetUserContext() != nil {
		t.Errorf("initial user context should be nil")
	}

	type myData struct{ Name string }
	data := &myData{Name: "test"}
	ctx.SetUserContext(data)

	got := ctx.GetUserContext()
	if got != data {
		t.Errorf("GetUserContext() returned wrong value")
	}

	gotData, ok := got.(*myData)
	if !ok || gotData.Name != "test" {
		t.Errorf("GetUserContext() = %v, want %v", got, data)
	}
}

func TestErrorPushPop(t *testing.T) {
	var lastError *Error
	iface := &Interface{
		OnError: func(err *Error) {
			lastError = err
		},
	}
	ctx := NewContext(nil, iface, 256)

	// Pop from empty queue returns nil
	if got := ctx.ErrorPop(); got != nil {
		t.Errorf("ErrorPop() on empty queue = %v, want nil", got)
	}

	// Push and pop in FIFO order
	ctx.ErrorPush(&Error{Code: -100, Info: "first"})
	ctx.ErrorPush(&Error{Code: -200, Info: "second"})

	// OnError callback should have been called with the last error
	if lastError == nil || lastError.Code != -200 {
		t.Errorf("OnError callback got %v, want code -200", lastError)
	}

	err1 := ctx.ErrorPop()
	if err1 == nil || err1.Code != -100 {
		t.Errorf("first ErrorPop() = %v, want code -100", err1)
	}

	err2 := ctx.ErrorPop()
	if err2 == nil || err2.Code != -200 {
		t.Errorf("second ErrorPop() = %v, want code -200", err2)
	}

	// Now empty again
	if got := ctx.ErrorPop(); got != nil {
		t.Errorf("ErrorPop() after drain = %v, want nil", got)
	}
}

func TestErrorPushOverflow(t *testing.T) {
	ctx := NewContext(nil, nil, 256)

	// Push 11 errors into queue with capacity 10
	for i := 0; i < 11; i++ {
		ctx.ErrorPush(&Error{Code: int16(i), Info: "err"})
	}

	// First pop should return error with code 1 (code 0 was evicted)
	err := ctx.ErrorPop()
	if err == nil || err.Code != 1 {
		t.Errorf("ErrorPop() after overflow = %v, want code 1", err)
	}
}

// =============================================================================
// Step 2: Result Formatting Functions
// =============================================================================

func TestResultInt32(t *testing.T) {
	tests := []struct {
		name  string
		value int32
		want  string
	}{
		{"positive", 42, "42"},
		{"negative", -17, "-17"},
		{"zero", 0, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder
			commands := []*Command{
				{
					Pattern: "TEST?",
					Callback: func(ctx *Context) Result {
						ctx.ResultInt32(tt.value)
						return ResOK
					},
				},
			}
			iface := &Interface{
				Write: func(data []byte) (int, error) {
					output.Write(data)
					return len(data), nil
				},
			}
			ctx := NewContext(commands, iface, 256)
			ctx.Input([]byte("TEST?\n"))

			got := output.String()
			if got != tt.want+"\n" {
				t.Errorf("ResultInt32(%d) output = %q, want %q", tt.value, got, tt.want+"\n")
			}
		})
	}
}

func TestResultInt64(t *testing.T) {
	var output strings.Builder
	commands := []*Command{
		{
			Pattern: "TEST?",
			Callback: func(ctx *Context) Result {
				ctx.ResultInt64(1234567890123)
				return ResOK
			},
		},
	}
	iface := &Interface{
		Write: func(data []byte) (int, error) {
			output.Write(data)
			return len(data), nil
		},
	}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST?\n"))

	got := output.String()
	if got != "1234567890123\n" {
		t.Errorf("ResultInt64 output = %q, want %q", got, "1234567890123\n")
	}
}

func TestResultFloat(t *testing.T) {
	var output strings.Builder
	commands := []*Command{
		{
			Pattern: "TEST?",
			Callback: func(ctx *Context) Result {
				ctx.ResultFloat(3.14)
				return ResOK
			},
		},
	}
	iface := &Interface{
		Write: func(data []byte) (int, error) {
			output.Write(data)
			return len(data), nil
		},
	}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST?\n"))

	got := output.String()
	if got != "3.14\n" {
		t.Errorf("ResultFloat output = %q, want %q", got, "3.14\n")
	}
}

func TestResultDouble(t *testing.T) {
	var output strings.Builder
	commands := []*Command{
		{
			Pattern: "TEST?",
			Callback: func(ctx *Context) Result {
				ctx.ResultDouble(2.718281828)
				return ResOK
			},
		},
	}
	iface := &Interface{
		Write: func(data []byte) (int, error) {
			output.Write(data)
			return len(data), nil
		},
	}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST?\n"))

	got := output.String()
	if got != "2.718281828\n" {
		t.Errorf("ResultDouble output = %q, want %q", got, "2.718281828\n")
	}
}

func TestResultBool(t *testing.T) {
	tests := []struct {
		value bool
		want  string
	}{
		{true, "1"},
		{false, "0"},
	}

	for _, tt := range tests {
		var output strings.Builder
		commands := []*Command{
			{
				Pattern: "TEST?",
				Callback: func(ctx *Context) Result {
					ctx.ResultBool(tt.value)
					return ResOK
				},
			},
		}
		iface := &Interface{
			Write: func(data []byte) (int, error) {
				output.Write(data)
				return len(data), nil
			},
		}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST?\n"))

		got := output.String()
		if got != tt.want+"\n" {
			t.Errorf("ResultBool(%v) output = %q, want %q", tt.value, got, tt.want+"\n")
		}
	}
}

func TestResultMnemonic(t *testing.T) {
	var output strings.Builder
	commands := []*Command{
		{
			Pattern: "TEST?",
			Callback: func(ctx *Context) Result {
				ctx.ResultMnemonic("RUNNING")
				return ResOK
			},
		},
	}
	iface := &Interface{
		Write: func(data []byte) (int, error) {
			output.Write(data)
			return len(data), nil
		},
	}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST?\n"))

	got := output.String()
	if got != "RUNNING\n" {
		t.Errorf("ResultMnemonic output = %q, want %q", got, "RUNNING\n")
	}
}

func TestResultMultipleValues(t *testing.T) {
	var output strings.Builder
	commands := []*Command{
		{
			Pattern: "TEST?",
			Callback: func(ctx *Context) Result {
				ctx.ResultInt32(1)
				ctx.ResultInt32(2)
				ctx.ResultInt32(3)
				return ResOK
			},
		},
	}
	iface := &Interface{
		Write: func(data []byte) (int, error) {
			output.Write(data)
			return len(data), nil
		},
	}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST?\n"))

	got := output.String()
	if got != "1,2,3\n" {
		t.Errorf("ResultMultipleValues output = %q, want %q", got, "1,2,3\n")
	}
}

// =============================================================================
// Step 3: Parameter Extraction Functions
// =============================================================================

func TestParamInt32(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int32
	}{
		{"decimal", "42", 42},
		{"negative", "-17", -17},
		{"zero", "0", 0},
		{"hex", "#HFF", 255},
		{"octal", "#Q77", 63},
		{"binary", "#B1010", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int32
			var gotErr error

			commands := []*Command{
				{
					Pattern: "TEST",
					Callback: func(ctx *Context) Result {
						val, err := ctx.ParamInt32(true)
						result = val
						gotErr = err
						if err != nil {
							return ResErr
						}
						return ResOK
					},
				},
			}
			iface := &Interface{
				Write: func(data []byte) (int, error) {
					return len(data), nil
				},
			}
			ctx := NewContext(commands, iface, 256)
			ctx.Input([]byte("TEST " + tt.input + "\n"))

			if gotErr != nil {
				t.Fatalf("ParamInt32 error: %v", gotErr)
			}
			if result != tt.want {
				t.Errorf("ParamInt32(%q) = %d, want %d", tt.input, result, tt.want)
			}
		})
	}
}

func TestParamInt64(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{"decimal", "123456789", 123456789},
		{"large", "9876543210", 9876543210},
		{"hex", "#HFFFFFFFF", 4294967295},
		{"octal", "#Q777", 511},
		{"binary", "#B11111111", 255},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int64
			var gotErr error

			commands := []*Command{
				{
					Pattern: "TEST",
					Callback: func(ctx *Context) Result {
						val, err := ctx.ParamInt64(true)
						result = val
						gotErr = err
						if err != nil {
							return ResErr
						}
						return ResOK
					},
				},
			}
			iface := &Interface{
				Write: func(data []byte) (int, error) {
					return len(data), nil
				},
			}
			ctx := NewContext(commands, iface, 256)
			ctx.Input([]byte("TEST " + tt.input + "\n"))

			if gotErr != nil {
				t.Fatalf("ParamInt64 error: %v", gotErr)
			}
			if result != tt.want {
				t.Errorf("ParamInt64(%q) = %d, want %d", tt.input, result, tt.want)
			}
		})
	}
}

func TestParamFloat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  float32
	}{
		{"integer", "42", 42},
		{"decimal", "3.14", 3.14},
		{"scientific", "1.5e2", 150},
		{"negative", "-2.5", -2.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result float32
			var gotErr error

			commands := []*Command{
				{
					Pattern: "TEST",
					Callback: func(ctx *Context) Result {
						val, err := ctx.ParamFloat(true)
						result = val
						gotErr = err
						if err != nil {
							return ResErr
						}
						return ResOK
					},
				},
			}
			iface := &Interface{
				Write: func(data []byte) (int, error) {
					return len(data), nil
				},
			}
			ctx := NewContext(commands, iface, 256)
			ctx.Input([]byte("TEST " + tt.input + "\n"))

			if gotErr != nil {
				t.Fatalf("ParamFloat error: %v", gotErr)
			}
			if result != tt.want {
				t.Errorf("ParamFloat(%q) = %g, want %g", tt.input, result, tt.want)
			}
		})
	}
}

func TestParamString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"double quoted", `"hello world"`, "hello world"},
		{"single quoted", "'test'", "test"},
		{"escaped double", `"say ""hi"""`, `say "hi"`},
		{"escaped single", "'it''s'", "it's"},
		{"mnemonic", "VOLT", "VOLT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			var gotErr error

			commands := []*Command{
				{
					Pattern: "TEST",
					Callback: func(ctx *Context) Result {
						val, err := ctx.ParamString(true)
						result = val
						gotErr = err
						if err != nil {
							return ResErr
						}
						return ResOK
					},
				},
			}
			iface := &Interface{
				Write: func(data []byte) (int, error) {
					return len(data), nil
				},
			}
			ctx := NewContext(commands, iface, 256)
			ctx.Input([]byte("TEST " + tt.input + "\n"))

			if gotErr != nil {
				t.Fatalf("ParamString error: %v", gotErr)
			}
			if result != tt.want {
				t.Errorf("ParamString(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

func TestParamChoice(t *testing.T) {
	choices := []ChoiceDef{
		{Name: "MINimum", Tag: 1},
		{Name: "MAXimum", Tag: 2},
		{Name: "DEFault", Tag: 3},
	}

	tests := []struct {
		name    string
		input   string
		want    int32
		wantErr bool
	}{
		{"short form", "MIN", 1, false},
		{"full form", "MAXIMUM", 2, false},
		{"default", "DEF", 3, false},
		{"invalid", "BOGUS", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int32
			var gotErr error

			commands := []*Command{
				{
					Pattern: "TEST",
					Callback: func(ctx *Context) Result {
						val, err := ctx.ParamChoice(choices, true)
						result = val
						gotErr = err
						if err != nil {
							return ResErr
						}
						return ResOK
					},
				},
			}
			iface := &Interface{
				Write: func(data []byte) (int, error) {
					return len(data), nil
				},
			}
			ctx := NewContext(commands, iface, 256)
			ctx.Input([]byte("TEST " + tt.input + "\n"))

			if tt.wantErr {
				if gotErr == nil {
					t.Errorf("ParamChoice(%q) expected error, got nil", tt.input)
				}
			} else {
				if gotErr != nil {
					t.Fatalf("ParamChoice error: %v", gotErr)
				}
				if result != tt.want {
					t.Errorf("ParamChoice(%q) = %d, want %d", tt.input, result, tt.want)
				}
			}
		})
	}
}

func TestParamOptionalMissing(t *testing.T) {
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				// All optional params with no data should return defaults
				i32, err := ctx.ParamInt32(false)
				if err != nil || i32 != 0 {
					return ResErr
				}
				return ResOK
			},
		},
	}
	iface := &Interface{
		Write: func(data []byte) (int, error) {
			return len(data), nil
		},
	}
	ctx := NewContext(commands, iface, 256)
	err := ctx.Input([]byte("TEST\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
}

func TestParamOptionalAllTypes(t *testing.T) {
	t.Run("int64", func(t *testing.T) {
		var result int64
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					val, _ := ctx.ParamInt64(false)
					result = val
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST\n"))
		if result != 0 {
			t.Errorf("optional ParamInt64 = %d, want 0", result)
		}
	})

	t.Run("float", func(t *testing.T) {
		var result float32
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					val, _ := ctx.ParamFloat(false)
					result = val
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST\n"))
		if result != 0 {
			t.Errorf("optional ParamFloat = %g, want 0", result)
		}
	})

	t.Run("double", func(t *testing.T) {
		var result float64
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					val, _ := ctx.ParamDouble(false)
					result = val
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST\n"))
		if result != 0 {
			t.Errorf("optional ParamDouble = %g, want 0", result)
		}
	})

	t.Run("string", func(t *testing.T) {
		var result string
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					val, _ := ctx.ParamString(false)
					result = val
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST\n"))
		if result != "" {
			t.Errorf("optional ParamString = %q, want empty", result)
		}
	})

	t.Run("bool", func(t *testing.T) {
		var result bool
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					val, _ := ctx.ParamBool(false)
					result = val
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST\n"))
		if result != false {
			t.Errorf("optional ParamBool = %v, want false", result)
		}
	})

	t.Run("choice", func(t *testing.T) {
		var result int32
		choices := []ChoiceDef{{Name: "ON", Tag: 1}}
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					val, _ := ctx.ParamChoice(choices, false)
					result = val
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST\n"))
		if result != 0 {
			t.Errorf("optional ParamChoice = %d, want 0", result)
		}
	})

	t.Run("arbitrary block", func(t *testing.T) {
		var result []byte
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					val, _ := ctx.ParamArbitraryBlock(false)
					result = val
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST\n"))
		if result != nil {
			t.Errorf("optional ParamArbitraryBlock = %v, want nil", result)
		}
	})

	t.Run("channel list", func(t *testing.T) {
		var result []ChannelListEntry
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					val, _ := ctx.ParamChannelList(false)
					result = val
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST\n"))
		if result != nil {
			t.Errorf("optional ParamChannelList = %v, want nil", result)
		}
	})
}

// =============================================================================
// Step 4: IsCmd and lexColon
// =============================================================================

func TestIsCmd(t *testing.T) {
	var isCmdResult bool
	var isCmdNegResult bool

	commands := []*Command{
		{
			Pattern: "MEASure:VOLTage?",
			Callback: func(ctx *Context) Result {
				isCmdResult = ctx.IsCmd("MEASure:VOLTage?")
				isCmdNegResult = ctx.IsCmd("MEASure:CURRent?")
				return ResOK
			},
		},
	}
	iface := &Interface{
		Write: func(data []byte) (int, error) {
			return len(data), nil
		},
	}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("MEAS:VOLT?\n"))

	if !isCmdResult {
		t.Errorf("IsCmd(matching pattern) = false, want true")
	}
	if isCmdNegResult {
		t.Errorf("IsCmd(non-matching pattern) = true, want false")
	}

	// IsCmd outside callback (no current command) should return false
	if ctx.IsCmd("*IDN?") {
		t.Errorf("IsCmd outside callback = true, want false")
	}
}

func TestLexColon(t *testing.T) {
	state := &lexState{buffer: []byte(":"), pos: 0, len: 1}
	tok, length := state.lexColon()
	if length != 1 || tok.Type != TokenColon {
		t.Errorf("lexColon(':') = type %v length %d, want TokenColon length 1", tok.Type, length)
	}

	state = &lexState{buffer: []byte("x"), pos: 0, len: 1}
	tok, length = state.lexColon()
	if length != 0 || tok.Type != TokenUnknown {
		t.Errorf("lexColon('x') = type %v length %d, want TokenUnknown length 0", tok.Type, length)
	}
}

// =============================================================================
// Step 5: Low-Coverage Functions
// =============================================================================

func TestInputBuffering(t *testing.T) {
	callCount := 0
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				callCount++
				return ResOK
			},
		},
	}
	iface := &Interface{
		Write: func(data []byte) (int, error) {
			return len(data), nil
		},
	}
	ctx := NewContext(commands, iface, 256)

	// Partial data - no newline, should not parse yet
	ctx.Input([]byte("TES"))
	if callCount != 0 {
		t.Errorf("callback called after partial data, want 0 calls")
	}

	// Complete the command with newline
	ctx.Input([]byte("T\n"))
	if callCount != 1 {
		t.Errorf("callback count = %d after complete line, want 1", callCount)
	}
}

func TestInputEmptyFlush(t *testing.T) {
	callCount := 0
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				callCount++
				return ResOK
			},
		},
	}
	iface := &Interface{
		Write: func(data []byte) (int, error) {
			return len(data), nil
		},
	}
	ctx := NewContext(commands, iface, 256)

	// Buffer partial data
	ctx.Input([]byte("TEST"))
	if callCount != 0 {
		t.Errorf("callback called before flush")
	}

	// Empty input should flush the buffer
	ctx.Input([]byte{})
	if callCount != 1 {
		t.Errorf("callback count = %d after empty flush, want 1", callCount)
	}

	// Empty input with empty buffer should be a no-op
	err := ctx.Input([]byte{})
	if err != nil {
		t.Errorf("empty Input on empty buffer returned error: %v", err)
	}
}

func TestInputBufferOverflow(t *testing.T) {
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				return ResOK
			},
		},
	}
	iface := &Interface{
		Write: func(data []byte) (int, error) {
			return len(data), nil
		},
	}
	// Very small buffer
	ctx := NewContext(commands, iface, 5)

	err := ctx.Input([]byte("TOOLONGCOMMAND\n"))
	if err == nil {
		t.Errorf("expected buffer overflow error, got nil")
	}
}

func TestMatchCommandOptionalParts(t *testing.T) {
	tests := []struct {
		pattern string
		header  string
		want    bool
	}{
		// Pattern with optional part
		{"VOLTage[:DC]", "VOLT", true},
		{"VOLTage[:DC]", "VOLTAGE", true},
		{"VOLTage[:DC]", "VOLT:DC", true},
		{"VOLTage[:DC]", "VOLTAGE:DC", true},
		{"VOLTage[:DC]", "VOLT:AC", false},
		// Leading colon in header
		{":MEASure:VOLTage", ":MEAS:VOLT", true},
	}

	for _, tt := range tests {
		got := matchCommand(tt.pattern, tt.header)
		if got != tt.want {
			t.Errorf("matchCommand(%q, %q) = %v, want %v", tt.pattern, tt.header, got, tt.want)
		}
	}
}

func TestLexNewLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"LF", "\n", 1},
		{"CR", "\r", 1},
		{"CRLF", "\r\n", 2},
		{"not newline", "x", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &lexState{buffer: []byte(tt.input), pos: 0, len: len(tt.input)}
			tok, length := state.lexNewLine()
			if length != tt.want {
				t.Errorf("lexNewLine(%q) length = %d, want %d", tt.input, length, tt.want)
			}
			if tt.want > 0 && tok.Type != TokenNewLine {
				t.Errorf("lexNewLine(%q) type = %v, want TokenNewLine", tt.input, tok.Type)
			}
			if tt.want == 0 && tok.Type != TokenUnknown {
				t.Errorf("lexNewLine(%q) type = %v, want TokenUnknown", tt.input, tok.Type)
			}
		})
	}
}

func TestLexSuffixProgramData(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		length int
	}{
		{"mV", "mV", 2},
		{"Hz", "Hz", 2},
		{"123", "", 0},
		{"", "", 0},
	}

	for _, tt := range tests {
		state := &lexState{buffer: []byte(tt.input), pos: 0, len: len(tt.input)}
		tok, length := state.lexSuffixProgramData()
		if length != tt.length {
			t.Errorf("lexSuffixProgramData(%q) length = %d, want %d", tt.input, length, tt.length)
		}
		if length > 0 && string(tok.Data) != tt.want {
			t.Errorf("lexSuffixProgramData(%q) data = %q, want %q", tt.input, string(tok.Data), tt.want)
		}
	}
}

func TestLexArbitraryBlockIndefinite(t *testing.T) {
	input := "#0Hello World\n"
	state := &lexState{buffer: []byte(input), pos: 0, len: len(input)}
	tok, length := state.lexArbitraryBlock()
	if length == 0 {
		t.Fatalf("lexArbitraryBlock indefinite failed to parse")
	}
	if tok.Type != TokenArbitraryBlock {
		t.Errorf("type = %v, want TokenArbitraryBlock", tok.Type)
	}
	// Should contain #0Hello World (up to but not including \n)
	data := string(tok.Data)
	if data != "#0Hello World" {
		t.Errorf("data = %q, want %q", data, "#0Hello World")
	}
}

func TestLexArbitraryBlockEdgeCases(t *testing.T) {
	// # not followed by digit
	state := &lexState{buffer: []byte("#X"), pos: 0, len: 2}
	_, length := state.lexArbitraryBlock()
	if length != 0 {
		t.Errorf("lexArbitraryBlock('#X') should fail")
	}

	// # at end of stream
	state = &lexState{buffer: []byte("#"), pos: 0, len: 1}
	_, length = state.lexArbitraryBlock()
	if length != 0 {
		t.Errorf("lexArbitraryBlock('#') should fail")
	}

	// Data shorter than declared length
	state = &lexState{buffer: []byte("#15AB"), pos: 0, len: 4}
	_, length = state.lexArbitraryBlock()
	if length != 0 {
		t.Errorf("lexArbitraryBlock with insufficient data should fail")
	}
}

func TestLexProgramExpression(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		length int
	}{
		{"simple", "(@1)", "(@1)", 4},
		{"nested", "((a))", "((a))", 5},
		{"not paren", "x", "", 0},
		{"unclosed", "(abc", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &lexState{buffer: []byte(tt.input), pos: 0, len: len(tt.input)}
			tok, length := state.lexProgramExpression()
			if length != tt.length {
				t.Errorf("lexProgramExpression(%q) length = %d, want %d", tt.input, length, tt.length)
			}
			if length > 0 && string(tok.Data) != tt.want {
				t.Errorf("lexProgramExpression(%q) data = %q, want %q", tt.input, string(tok.Data), tt.want)
			}
		})
	}
}

func TestLexProgramHeader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType TokenType
		wantData string
	}{
		{"common cmd", "*RST", TokenCommonProgramHeader, "*RST"},
		{"common query", "*IDN?", TokenCommonProgramHeader, "*IDN?"},
		{"compound", "SOUR:VOLT", TokenCompoundProgramHeader, "SOUR:VOLT"},
		{"compound query", "MEAS:VOLT?", TokenCompoundProgramHeader, "MEAS:VOLT?"},
		{"leading colon", ":SOUR:VOLT", TokenCompoundProgramHeader, ":SOUR:VOLT"},
		{"simple", "VOLT", TokenCompoundProgramHeader, "VOLT"},
		{"with digits", "VOLT1", TokenCompoundProgramHeader, "VOLT1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &lexState{buffer: []byte(tt.input), pos: 0, len: len(tt.input)}
			tok, length := state.lexProgramHeader()
			if length == 0 {
				t.Fatalf("lexProgramHeader(%q) failed to parse", tt.input)
			}
			if tok.Type != tt.wantType {
				t.Errorf("lexProgramHeader(%q) type = %v, want %v", tt.input, tok.Type, tt.wantType)
			}
			if string(tok.Data) != tt.wantData {
				t.Errorf("lexProgramHeader(%q) data = %q, want %q", tt.input, string(tok.Data), tt.wantData)
			}
		})
	}

	// Invalid header (starts with digit)
	state := &lexState{buffer: []byte("123"), pos: 0, len: 3}
	_, length := state.lexProgramHeader()
	if length != 0 {
		t.Errorf("lexProgramHeader('123') should return 0 length")
	}

	// Just * with nothing after
	state = &lexState{buffer: []byte("*"), pos: 0, len: 1}
	_, length = state.lexProgramHeader()
	if length != 0 {
		t.Errorf("lexProgramHeader('*') should return 0 length")
	}
}

func TestParamToFloat64FromNondecimal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  float64
	}{
		{"hex", "#HFF", 255},
		{"octal", "#Q77", 63},
		{"binary", "#B1010", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result float64
			var gotErr error

			commands := []*Command{
				{
					Pattern: "TEST",
					Callback: func(ctx *Context) Result {
						val, err := ctx.ParamDouble(true)
						result = val
						gotErr = err
						if err != nil {
							return ResErr
						}
						return ResOK
					},
				},
			}
			iface := &Interface{
				Write: func(data []byte) (int, error) {
					return len(data), nil
				},
			}
			ctx := NewContext(commands, iface, 256)
			ctx.Input([]byte("TEST " + tt.input + "\n"))

			if gotErr != nil {
				t.Fatalf("ParamDouble error: %v", gotErr)
			}
			if result != tt.want {
				t.Errorf("ParamDouble(%q) = %g, want %g", tt.input, result, tt.want)
			}
		})
	}
}

func TestParamBoolErrors(t *testing.T) {
	t.Run("invalid mnemonic", func(t *testing.T) {
		var gotErr error
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					_, err := ctx.ParamBool(true)
					gotErr = err
					if err != nil {
						return ResErr
					}
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST BOGUS\n"))

		if gotErr == nil {
			t.Errorf("ParamBool(BOGUS) expected error, got nil")
		}
	})

	t.Run("invalid type (string)", func(t *testing.T) {
		var gotErr error
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					_, err := ctx.ParamBool(true)
					gotErr = err
					if err != nil {
						return ResErr
					}
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST \"hello\"\n"))

		if gotErr == nil {
			t.Errorf("ParamBool(string) expected error, got nil")
		}
	})
}

func TestParamMandatoryMissing(t *testing.T) {
	var gotErr error
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				_, err := ctx.ParamInt32(true)
				gotErr = err
				if err != nil {
					return ResErr
				}
				return ResOK
			},
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST\n"))

	if gotErr == nil {
		t.Errorf("mandatory param with no data should return error")
	}
}

func TestParamChannelListErrors(t *testing.T) {
	t.Run("non-expression type", func(t *testing.T) {
		var gotErr error
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					_, err := ctx.ParamChannelList(true)
					gotErr = err
					if err != nil {
						return ResErr
					}
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST 123\n"))

		if gotErr == nil {
			t.Errorf("ParamChannelList with number should return error")
		}
	})

	t.Run("missing @ prefix", func(t *testing.T) {
		var gotErr error
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					_, err := ctx.ParamChannelList(true)
					gotErr = err
					if err != nil {
						return ResErr
					}
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST (1,2,3)\n"))

		if gotErr == nil {
			t.Errorf("ParamChannelList without @ should return error")
		}
	})
}

func TestParamArbitraryBlockErrors(t *testing.T) {
	t.Run("wrong type", func(t *testing.T) {
		var gotErr error
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					_, err := ctx.ParamArbitraryBlock(true)
					gotErr = err
					if err != nil {
						return ResErr
					}
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST 123\n"))

		if gotErr == nil {
			t.Errorf("ParamArbitraryBlock with number should return error")
		}
	})

	t.Run("indefinite length", func(t *testing.T) {
		var result []byte
		commands := []*Command{
			{
				Pattern: "TEST",
				Callback: func(ctx *Context) Result {
					data, err := ctx.ParamArbitraryBlock(true)
					if err != nil {
						return ResErr
					}
					result = data
					return ResOK
				},
			},
		}
		iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
		ctx := NewContext(commands, iface, 256)
		ctx.Input([]byte("TEST #0ABCDEF\n"))

		if string(result) != "ABCDEF" {
			t.Errorf("ParamArbitraryBlock indefinite = %q, want %q", string(result), "ABCDEF")
		}
	})
}

func TestParseInvalidHeader(t *testing.T) {
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result { return ResOK },
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)

	// A line starting with a digit is an invalid header
	err := ctx.Parse([]byte("123\n"))
	if err == nil {
		t.Errorf("expected error for invalid header, got nil")
	}
}

func TestParseUnknownCommand(t *testing.T) {
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result { return ResOK },
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)

	err := ctx.Parse([]byte("BOGUS\n"))
	if err == nil {
		t.Errorf("expected error for unknown command, got nil")
	}
}

func TestParseCallbackError(t *testing.T) {
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result { return ResErr },
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)

	ctx.Parse([]byte("TEST\n"))

	// Should have pushed -200 error
	err := ctx.ErrorPop()
	if err == nil || err.Code != -200 {
		t.Errorf("ErrorPop after ResErr callback = %v, want code -200", err)
	}
}

// =============================================================================
// Step 6: Remaining Gap-Fillers
// =============================================================================

func TestParseMultipleCommandsNewline(t *testing.T) {
	callCount := 0
	commands := []*Command{
		{
			Pattern: "CMD1",
			Callback: func(ctx *Context) Result {
				callCount++
				return ResOK
			},
		},
		{
			Pattern: "CMD2",
			Callback: func(ctx *Context) Result {
				callCount++
				return ResOK
			},
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)

	// Two separate lines via Input
	ctx.Input([]byte("CMD1\n"))
	ctx.Input([]byte("CMD2\n"))

	if callCount != 2 {
		t.Errorf("callback count = %d, want 2", callCount)
	}
}

func TestWriteDataNoInterface(t *testing.T) {
	ctx := NewContext(nil, nil, 256)
	n, err := ctx.writeData([]byte("test"))
	if n != 0 || err != nil {
		t.Errorf("writeData with nil interface = (%d, %v), want (0, nil)", n, err)
	}
}

func TestWriteNewLineWithFlush(t *testing.T) {
	flushed := false
	iface := &Interface{
		Write: func(data []byte) (int, error) { return len(data), nil },
		Flush: func() error {
			flushed = true
			return nil
		},
	}
	ctx := NewContext(nil, iface, 256)
	ctx.writeNewLine()

	if !flushed {
		t.Errorf("writeNewLine did not call Flush")
	}
}

func TestLexNondecimalNumericEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		length int
	}{
		{"not hash", "123", 0},
		{"hash at end", "#", 0},
		{"unknown base", "#X5", 0},
		{"lowercase hex", "#hFF", 3},
		{"lowercase octal", "#q77", 3},
		{"lowercase binary", "#b10", 3},
		{"hex no digits", "#H", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &lexState{buffer: []byte(tt.input), pos: 0, len: len(tt.input)}
			_, length := state.lexNondecimalNumeric()
			wantNonZero := tt.length > 0
			gotNonZero := length > 0
			if gotNonZero != wantNonZero {
				t.Errorf("lexNondecimalNumeric(%q) length = %d, want nonzero=%v", tt.input, length, wantNonZero)
			}
		})
	}
}

func TestLexDecimalNumericEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		length int
	}{
		{"just sign", "+", 0},
		{"just dot", ".", 0},
		{"dot with digits", ".5", 2},
		{"alpha", "abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &lexState{buffer: []byte(tt.input), pos: 0, len: len(tt.input)}
			_, length := state.lexDecimalNumeric()
			if length != tt.length {
				t.Errorf("lexDecimalNumeric(%q) length = %d, want %d", tt.input, length, tt.length)
			}
		})
	}
}

func TestLexStringProgramDataEdgeCases(t *testing.T) {
	// Unterminated string
	state := &lexState{buffer: []byte(`"hello`), pos: 0, len: 6}
	_, length := state.lexStringProgramData()
	if length != 0 {
		t.Errorf("unterminated string should return 0")
	}

	// Not a quote
	state = &lexState{buffer: []byte("x"), pos: 0, len: 1}
	_, length = state.lexStringProgramData()
	if length != 0 {
		t.Errorf("non-quote should return 0")
	}
}

func TestLexCharacterProgramData(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		length int
	}{
		{"simple", "VOLT", "VOLT", 4},
		{"with digits", "CH1", "CH1", 3},
		{"with underscore", "MY_VAR", "MY_VAR", 6},
		{"starts with digit", "1ABC", "", 0},
		{"empty", "", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &lexState{buffer: []byte(tt.input), pos: 0, len: len(tt.input)}
			tok, length := state.lexCharacterProgramData()
			if length != tt.length {
				t.Errorf("lexCharacterProgramData(%q) length = %d, want %d", tt.input, length, tt.length)
			}
			if length > 0 && string(tok.Data) != tt.want {
				t.Errorf("lexCharacterProgramData(%q) data = %q, want %q", tt.input, string(tok.Data), tt.want)
			}
		})
	}
}

func TestLexAdvanceOverflow(t *testing.T) {
	state := &lexState{buffer: []byte("ab"), pos: 0, len: 2}
	state.advance(100) // Advance way past end
	if state.pos != 2 {
		t.Errorf("advance past end: pos = %d, want 2", state.pos)
	}
}

func TestParamDecimalWithSuffix(t *testing.T) {
	var result float64
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				val, err := ctx.ParamDouble(true)
				if err != nil {
					return ResErr
				}
				result = val
				return ResOK
			},
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST 3.14 V\n"))

	if result != 3.14 {
		t.Errorf("ParamDouble with suffix = %g, want 3.14", result)
	}
}

func TestParamInt32WithSuffix(t *testing.T) {
	var result int32
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				val, err := ctx.ParamInt32(true)
				if err != nil {
					return ResErr
				}
				result = val
				return ResOK
			},
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST 100 mV\n"))

	if result != 100 {
		t.Errorf("ParamInt32 with suffix = %d, want 100", result)
	}
}

func TestParamChoiceNonMnemonic(t *testing.T) {
	var gotErr error
	choices := []ChoiceDef{{Name: "ON", Tag: 1}}
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				_, err := ctx.ParamChoice(choices, true)
				gotErr = err
				if err != nil {
					return ResErr
				}
				return ResOK
			},
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST 123\n"))

	if gotErr == nil {
		t.Errorf("ParamChoice with numeric should return error")
	}
}

func TestParamToInt32ConversionError(t *testing.T) {
	var gotErr error
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				_, err := ctx.ParamInt32(true)
				gotErr = err
				if err != nil {
					return ResErr
				}
				return ResOK
			},
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)
	// String data cannot be converted to int32
	ctx.Input([]byte("TEST \"hello\"\n"))

	if gotErr == nil {
		t.Errorf("paramToInt32 with string should return error")
	}
}

func TestParamToInt64ConversionError(t *testing.T) {
	var gotErr error
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				_, err := ctx.ParamInt64(true)
				gotErr = err
				if err != nil {
					return ResErr
				}
				return ResOK
			},
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST \"hello\"\n"))

	if gotErr == nil {
		t.Errorf("paramToInt64 with string should return error")
	}
}

func TestParamToFloat64ConversionError(t *testing.T) {
	var gotErr error
	commands := []*Command{
		{
			Pattern: "TEST",
			Callback: func(ctx *Context) Result {
				_, err := ctx.ParamDouble(true)
				gotErr = err
				if err != nil {
					return ResErr
				}
				return ResOK
			},
		},
	}
	iface := &Interface{Write: func(data []byte) (int, error) { return len(data), nil }}
	ctx := NewContext(commands, iface, 256)
	ctx.Input([]byte("TEST \"hello\"\n"))

	if gotErr == nil {
		t.Errorf("paramToFloat64 with string should return error")
	}
}

func TestCommandNumbersNoCommand(t *testing.T) {
	ctx := NewContext(nil, nil, 256)
	result := ctx.CommandNumbers(3, 5)
	if len(result) != 3 {
		t.Fatalf("CommandNumbers length = %d, want 3", len(result))
	}
	for i, v := range result {
		if v != 5 {
			t.Errorf("result[%d] = %d, want 5", i, v)
		}
	}
}
