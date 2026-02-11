package scpi

import (
	"fmt"
	"strconv"
	"strings"
)

// Parameter reads the next parameter from the command line
func (c *Context) Parameter(mandatory bool) (*Parameter, error) {
	state := &lexState{
		buffer: c.currentParams,
		pos:    c.paramsPos,
		len:    len(c.currentParams),
	}

	// Skip whitespace
	state.lexWhitespace()

	// Check if we're at the end
	if state.isEOS() {
		if mandatory {
			c.ErrorPush(&Error{Code: -109, Info: "Missing parameter"})
			return nil, fmt.Errorf("missing parameter")
		}
		return &Parameter{Type: TokenUnknown}, nil
	}

	// If not first parameter, expect comma
	if c.inputCount > 0 {
		tok, _ := state.lexComma()
		if tok.Type != TokenComma {
			c.ErrorPush(&Error{Code: -104, Info: "Invalid separator"})
			return nil, fmt.Errorf("invalid separator")
		}
		state.lexWhitespace()
	}

	c.inputCount++

	// Parse program data
	param := c.parseProgramData(state)
	c.paramsPos = state.pos

	return param, nil
}

// parseProgramData parses a single parameter value
func (c *Context) parseProgramData(state *lexState) *Parameter {
	// Try different token types

	// Try nondecimal numeric (hex, octal, binary)
	if tok, length := state.lexNondecimalNumeric(); length > 0 {
		return (*Parameter)(&tok)
	}

	// Try character/mnemonic data
	if tok, length := state.lexCharacterProgramData(); length > 0 {
		return (*Parameter)(&tok)
	}

	// Try decimal numeric (possibly with suffix)
	if tok, length := state.lexDecimalNumeric(); length > 0 {
		// Check for suffix
		wsStart := state.pos
		_, _ = state.lexWhitespace()
		_, suffixLen := state.lexSuffixProgramData()

		if suffixLen > 0 {
			// Extend token to include suffix
			tok.Type = TokenDecimalNumericWithSuffix
			tok.Data = state.buffer[tok.Pos : state.pos]
			return (*Parameter)(&tok)
		}

		// No suffix, restore position
		state.pos = wsStart
		return (*Parameter)(&tok)
	}

	// Try string data
	if tok, length := state.lexStringProgramData(); length > 0 {
		return (*Parameter)(&tok)
	}

	// Try arbitrary block
	if tok, length := state.lexArbitraryBlock(); length > 0 {
		return (*Parameter)(&tok)
	}

	// Try program expression
	if tok, length := state.lexProgramExpression(); length > 0 {
		return (*Parameter)(&tok)
	}

	// Unknown token type
	return &Parameter{Type: TokenUnknown}
}

// ParamInt32 reads a mandatory or optional int32 parameter
func (c *Context) ParamInt32(mandatory bool) (int32, error) {
	param, err := c.Parameter(mandatory)
	if err != nil {
		return 0, err
	}

	if param.Type == TokenUnknown {
		return 0, nil
	}

	return c.paramToInt32(param)
}

// ParamInt64 reads a mandatory or optional int64 parameter
func (c *Context) ParamInt64(mandatory bool) (int64, error) {
	param, err := c.Parameter(mandatory)
	if err != nil {
		return 0, err
	}

	if param.Type == TokenUnknown {
		return 0, nil
	}

	return c.paramToInt64(param)
}

// ParamFloat reads a mandatory or optional float32 parameter
func (c *Context) ParamFloat(mandatory bool) (float32, error) {
	param, err := c.Parameter(mandatory)
	if err != nil {
		return 0, err
	}

	if param.Type == TokenUnknown {
		return 0, nil
	}

	val, err := c.paramToFloat64(param)
	return float32(val), err
}

// ParamDouble reads a mandatory or optional float64 parameter
func (c *Context) ParamDouble(mandatory bool) (float64, error) {
	param, err := c.Parameter(mandatory)
	if err != nil {
		return 0, err
	}

	if param.Type == TokenUnknown {
		return 0, nil
	}

	return c.paramToFloat64(param)
}

// ParamString reads a mandatory or optional string parameter
func (c *Context) ParamString(mandatory bool) (string, error) {
	param, err := c.Parameter(mandatory)
	if err != nil {
		return "", err
	}

	if param.Type == TokenUnknown {
		return "", nil
	}

	return c.paramToString(param)
}

// ParamBool reads a mandatory or optional boolean parameter (0/1, ON/OFF)
func (c *Context) ParamBool(mandatory bool) (bool, error) {
	param, err := c.Parameter(mandatory)
	if err != nil {
		return false, err
	}

	if param.Type == TokenUnknown {
		return false, nil
	}

	// Try as integer
	if param.Type == TokenDecimalNumeric {
		val, err := c.paramToInt32(param)
		if err != nil {
			return false, err
		}
		return val != 0, nil
	}

	// Try as mnemonic (ON/OFF)
	if param.Type == TokenProgramMnemonic {
		str := strings.ToUpper(string(param.Data))
		switch str {
		case "ON", "1":
			return true, nil
		case "OFF", "0":
			return false, nil
		default:
			c.ErrorPush(&Error{Code: -108, Info: "Invalid parameter value"})
			return false, fmt.Errorf("invalid boolean value: %s", str)
		}
	}

	c.ErrorPush(&Error{Code: -104, Info: "Data type error"})
	return false, fmt.Errorf("invalid data type for boolean")
}

// ParamArbitraryBlock reads a mandatory or optional arbitrary block parameter.
// Returns the raw data bytes from a definite-length block (#<n><length><data>).
func (c *Context) ParamArbitraryBlock(mandatory bool) ([]byte, error) {
	param, err := c.Parameter(mandatory)
	if err != nil {
		return nil, err
	}

	if param.Type == TokenUnknown {
		return nil, nil
	}

	if param.Type != TokenArbitraryBlock {
		c.ErrorPush(&Error{Code: -104, Info: "Data type error"})
		return nil, fmt.Errorf("expected arbitrary block data")
	}

	data := param.Data
	if len(data) < 2 || data[0] != '#' {
		c.ErrorPush(&Error{Code: -104, Info: "Invalid arbitrary block"})
		return nil, fmt.Errorf("invalid arbitrary block format")
	}

	n := int(data[1] - '0')
	if n == 0 {
		// Indefinite length: data is everything after #0
		return data[2:], nil
	}

	// Definite length: skip #, n digit, and n length digits
	headerLen := 2 + n
	if len(data) < headerLen {
		c.ErrorPush(&Error{Code: -104, Info: "Invalid arbitrary block"})
		return nil, fmt.Errorf("invalid arbitrary block format")
	}

	return data[headerLen:], nil
}

// ParamChannelList reads a channel list parameter and returns all parsed entries.
// Channel lists use the SCPI format (@<entries>) where entries are comma-separated.
// Each entry is a single value (e.g. "1" or "1!2") or a range (e.g. "1:3" or "1!1:3!2").
func (c *Context) ParamChannelList(mandatory bool) ([]ChannelListEntry, error) {
	param, err := c.Parameter(mandatory)
	if err != nil {
		return nil, err
	}

	if param.Type == TokenUnknown {
		return nil, nil
	}

	if param.Type != TokenProgramExpression {
		c.ErrorPush(&Error{Code: -104, Info: "Data type error"})
		return nil, fmt.Errorf("expected channel list expression")
	}

	data := string(param.Data)

	// Validate channel list format: (@...)
	if len(data) < 3 || data[0] != '(' || data[1] != '@' || data[len(data)-1] != ')' {
		c.ErrorPush(&Error{Code: -104, Info: "Invalid channel list"})
		return nil, fmt.Errorf("invalid channel list format")
	}

	inner := strings.TrimSpace(data[2 : len(data)-1])
	if inner == "" {
		return []ChannelListEntry{}, nil
	}

	parts := strings.Split(inner, ",")
	entries := make([]ChannelListEntry, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		entry, parseErr := parseChannelListEntry(part)
		if parseErr != nil {
			c.ErrorPush(&Error{Code: -104, Info: "Invalid channel list entry"})
			return nil, parseErr
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func parseChannelListEntry(s string) (ChannelListEntry, error) {
	if idx := strings.Index(s, ":"); idx >= 0 {
		from, err := parseDimensionValues(s[:idx])
		if err != nil {
			return ChannelListEntry{}, err
		}

		to, err := parseDimensionValues(s[idx+1:])
		if err != nil {
			return ChannelListEntry{}, err
		}

		dims := len(from)
		if len(to) > dims {
			dims = len(to)
		}

		return ChannelListEntry{
			IsRange:    true,
			From:       from,
			To:         to,
			Dimensions: dims,
		}, nil
	}

	from, err := parseDimensionValues(s)
	if err != nil {
		return ChannelListEntry{}, err
	}

	return ChannelListEntry{
		IsRange:    false,
		From:       from,
		Dimensions: len(from),
	}, nil
}

func parseDimensionValues(s string) ([]int32, error) {
	parts := strings.Split(s, "!")
	values := make([]int32, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		val, err := strconv.ParseInt(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid channel list value: %s", p)
		}
		values = append(values, int32(val))
	}

	return values, nil
}

// ParamChoice reads a choice parameter from a list of options
func (c *Context) ParamChoice(choices []ChoiceDef, mandatory bool) (int32, error) {
	param, err := c.Parameter(mandatory)
	if err != nil {
		return 0, err
	}

	if param.Type == TokenUnknown {
		return 0, nil
	}

	if param.Type != TokenProgramMnemonic {
		c.ErrorPush(&Error{Code: -104, Info: "Data type error"})
		return 0, fmt.Errorf("expected mnemonic for choice")
	}

	value := string(param.Data)
	for _, choice := range choices {
		if matchPattern(choice.Name, value) {
			return choice.Tag, nil
		}
	}

	c.ErrorPush(&Error{Code: -108, Info: "Invalid parameter value"})
	return 0, fmt.Errorf("invalid choice: %s", value)
}

// paramToInt32 converts a parameter to int32
func (c *Context) paramToInt32(param *Parameter) (int32, error) {
	switch param.Type {
	case TokenHexNum:
		// Skip #H prefix
		val, err := strconv.ParseInt(string(param.Data[2:]), 16, 32)
		return int32(val), err

	case TokenOctNum:
		// Skip #Q prefix
		val, err := strconv.ParseInt(string(param.Data[2:]), 8, 32)
		return int32(val), err

	case TokenBinNum:
		// Skip #B prefix
		val, err := strconv.ParseInt(string(param.Data[2:]), 2, 32)
		return int32(val), err

	case TokenDecimalNumeric, TokenDecimalNumericWithSuffix:
		// Extract numeric part (before any suffix)
		numStr := string(param.Data)
		if param.Type == TokenDecimalNumericWithSuffix {
			// Find where suffix starts
			for i, c := range numStr {
				if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
					numStr = numStr[:i]
					break
				}
			}
		}
		numStr = strings.TrimSpace(numStr)
		// Use integer parse for values without decimal point or exponent
		// to avoid float32 precision loss (e.g. INT32_MAX rounds in float32)
		if !strings.Contains(numStr, ".") && !strings.ContainsAny(numStr, "eE") {
			val, err := strconv.ParseInt(numStr, 10, 32)
			return int32(val), err
		}
		val, err := strconv.ParseFloat(numStr, 64)
		return int32(val), err

	default:
		c.ErrorPush(&Error{Code: -104, Info: "Data type error"})
		return 0, fmt.Errorf("cannot convert to int32")
	}
}

// paramToInt64 converts a parameter to int64
func (c *Context) paramToInt64(param *Parameter) (int64, error) {
	switch param.Type {
	case TokenHexNum:
		return strconv.ParseInt(string(param.Data[2:]), 16, 64)

	case TokenOctNum:
		return strconv.ParseInt(string(param.Data[2:]), 8, 64)

	case TokenBinNum:
		return strconv.ParseInt(string(param.Data[2:]), 2, 64)

	case TokenDecimalNumeric, TokenDecimalNumericWithSuffix:
		numStr := string(param.Data)
		if param.Type == TokenDecimalNumericWithSuffix {
			for i, c := range numStr {
				if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
					numStr = numStr[:i]
					break
				}
			}
		}
		numStr = strings.TrimSpace(numStr)
		// Use integer parse for values without decimal point or exponent
		if !strings.Contains(numStr, ".") && !strings.ContainsAny(numStr, "eE") {
			return strconv.ParseInt(numStr, 10, 64)
		}
		val, err := strconv.ParseFloat(numStr, 64)
		return int64(val), err

	default:
		c.ErrorPush(&Error{Code: -104, Info: "Data type error"})
		return 0, fmt.Errorf("cannot convert to int64")
	}
}

// paramToFloat64 converts a parameter to float64
func (c *Context) paramToFloat64(param *Parameter) (float64, error) {
	switch param.Type {
	case TokenHexNum, TokenOctNum, TokenBinNum:
		// Convert to int first
		val, err := c.paramToInt64(param)
		return float64(val), err

	case TokenDecimalNumeric, TokenDecimalNumericWithSuffix:
		numStr := string(param.Data)
		if param.Type == TokenDecimalNumericWithSuffix {
			for i, c := range numStr {
				if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
					numStr = numStr[:i]
					break
				}
			}
		}
		numStr = strings.TrimSpace(numStr)
		return strconv.ParseFloat(numStr, 64)

	default:
		c.ErrorPush(&Error{Code: -104, Info: "Data type error"})
		return 0, fmt.Errorf("cannot convert to float64")
	}
}

// paramToString converts a parameter to string
func (c *Context) paramToString(param *Parameter) (string, error) {
	switch param.Type {
	case TokenSingleQuoteData, TokenDoubleQuoteData:
		// Remove quotes and unescape
		str := string(param.Data[1 : len(param.Data)-1])
		quote := string(param.Data[0])
		str = strings.ReplaceAll(str, quote+quote, quote)
		return str, nil

	case TokenProgramMnemonic:
		return string(param.Data), nil

	default:
		return string(param.Data), nil
	}
}
