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
