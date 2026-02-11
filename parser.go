package scpi

import (
	"fmt"
	"strings"
)

// NewContext creates a new SCPI parser context
func NewContext(commands []*Command, iface *Interface, bufferSize int) *Context {
	ctx := &Context{
		commands:    commands,
		iface:       iface,
		inputBuffer: make([]byte, bufferSize),
		bufferPos:   0,
		errorQueue:  make([]*Error, 0, 10),
		firstOutput: true,
	}
	return ctx
}

// SetIDN sets the identification strings
func (c *Context) SetIDN(manufacturer, model, serial, version string) {
	c.idn[0] = manufacturer
	c.idn[1] = model
	c.idn[2] = serial
	c.idn[3] = version
}

// SetUserContext sets user-defined context data
func (c *Context) SetUserContext(ctx interface{}) {
	c.userContext = ctx
}

// GetUserContext retrieves user-defined context data
func (c *Context) GetUserContext() interface{} {
	return c.userContext
}

// ErrorPush adds an error to the error queue
func (c *Context) ErrorPush(err *Error) {
	if len(c.errorQueue) < cap(c.errorQueue) {
		c.errorQueue = append(c.errorQueue, err)
	} else {
		// Queue full, remove oldest
		c.errorQueue = append(c.errorQueue[1:], err)
	}
	c.cmdError = true

	if c.iface != nil && c.iface.OnError != nil {
		c.iface.OnError(err)
	}
}

// ErrorPop removes and returns the oldest error
func (c *Context) ErrorPop() *Error {
	if len(c.errorQueue) == 0 {
		return nil
	}
	err := c.errorQueue[0]
	c.errorQueue = c.errorQueue[1:]
	return err
}

// matchPattern checks if a value matches a SCPI pattern
// Pattern supports short/long form like "MEASure" matches "MEAS" or "MEASURE"
func matchPattern(pattern, value string) bool {
	value = strings.ToUpper(value)

	pIdx := 0
	vIdx := 0

	// Try to match
	for pIdx < len(pattern) && vIdx < len(value) {
		pChar := pattern[pIdx]
		vChar := value[vIdx]

		// Convert pattern character to uppercase for comparison
		pCharUpper := pChar
		if pChar >= 'a' && pChar <= 'z' {
			pCharUpper = pChar - 32
		}

		if pCharUpper == vChar {
			pIdx++
			vIdx++
			continue
		}

		// Skip lowercase letters in pattern (optional long form)
		if pChar >= 'a' && pChar <= 'z' {
			pIdx++
			continue
		}

		// Mismatch
		return false
	}

	// Check if we matched all of value
	if vIdx == len(value) {
		// Skip remaining lowercase letters in pattern
		for pIdx < len(pattern) && pattern[pIdx] >= 'a' && pattern[pIdx] <= 'z' {
			pIdx++
		}
		return pIdx == len(pattern)
	}

	return false
}

// matchCommand checks if a command header matches a pattern
func matchCommand(pattern, header string) bool {
	// Check if pattern and header query status match
	patternIsQuery := strings.HasSuffix(pattern, "?")
	headerIsQuery := strings.HasSuffix(header, "?")

	// Query patterns should only match query headers, and vice versa
	if patternIsQuery != headerIsQuery {
		return false
	}

	// Remove trailing ? from both pattern and header for comparison
	pattern = strings.TrimSuffix(pattern, "?")
	header = strings.TrimSuffix(header, "?")

	// Remove optional parts from pattern (parts in brackets)
	// For example, "VOLTage[:DC]" becomes "VOLTage" when matching without optional part
	// We'll try matching with and without optional parts
	patternWithoutOptional := pattern
	if strings.Contains(pattern, "[") && strings.Contains(pattern, "]") {
		beforeIdx := strings.Index(pattern, "[")
		afterIdx := strings.Index(pattern, "]")
		patternWithoutOptional = pattern[:beforeIdx] + pattern[afterIdx+1:]
	}

	// Try matching without optional part first
	if matchCommandParts(patternWithoutOptional, header) {
		return true
	}

	// Try matching with optional part included
	if strings.Contains(pattern, "[") && strings.Contains(pattern, "]") {
		// Remove brackets but keep the content
		patternWithOptional := strings.ReplaceAll(pattern, "[", "")
		patternWithOptional = strings.ReplaceAll(patternWithOptional, "]", "")
		if matchCommandParts(patternWithOptional, header) {
			return true
		}
	}

	return false
}

// matchCommandParts matches command pattern parts against header parts
func matchCommandParts(pattern, header string) bool {
	// Split both pattern and header by colons
	patternParts := strings.Split(pattern, ":")
	headerParts := strings.Split(header, ":")

	// Remove leading empty strings from absolute paths
	if len(patternParts) > 0 && patternParts[0] == "" {
		patternParts = patternParts[1:]
	}
	if len(headerParts) > 0 && headerParts[0] == "" {
		headerParts = headerParts[1:]
	}

	// Must have same number of parts
	if len(patternParts) != len(headerParts) {
		return false
	}

	// Match each part
	for i := 0; i < len(headerParts); i++ {
		part := patternParts[i]
		hdr := headerParts[i]

		// Handle numeric suffix (#) - only strip digits from header if pattern has #
		if strings.Contains(part, "#") {
			part = strings.Replace(part, "#", "", -1)
			hdr = strings.TrimRight(headerParts[i], "0123456789")
		}

		if !matchPattern(part, hdr) {
			return false
		}
	}

	return true
}

// findCommand finds a command that matches the given header
func (c *Context) findCommand(header string) *Command {
	for _, cmd := range c.commands {
		if matchCommand(cmd.Pattern, header) {
			return cmd
		}
	}
	return nil
}

// Parse parses a complete SCPI command line
func (c *Context) Parse(data []byte) error {
	c.outputCount = 0
	c.firstOutput = true

	state := &lexState{
		buffer: data,
		pos:    0,
		len:    len(data),
	}

	for !state.isEOS() {
		// Skip whitespace
		state.lexWhitespace()

		if state.isEOS() {
			break
		}

		// Parse program header (command)
		header, length := state.lexProgramHeader()
		if length == 0 || header.Type == TokenUnknown {
			// Invalid command
			c.ErrorPush(&Error{Code: -100, Info: "Invalid command"})
			return fmt.Errorf("invalid command at position %d", state.pos)
		}

		// Find matching command
		headerStr := string(header.Data)
		cmd := c.findCommand(headerStr)
		if cmd == nil {
			c.ErrorPush(&Error{Code: -113, Info: fmt.Sprintf("Undefined header: %s", headerStr)})
			return fmt.Errorf("undefined header: %s", headerStr)
		}

		// Set current command
		c.currentCmd = cmd
		c.cmdError = false
		c.inputCount = 0

		// Skip whitespace before parameters
		state.lexWhitespace()

		// Store parameter data position
		paramStart := state.pos

		// Skip to end of command (semicolon or newline)
		for !state.isEOS() {
			c := state.peek()
			if c == ';' || c == '\n' || c == '\r' {
				break
			}
			state.advance(1)
		}

		paramEnd := state.pos
		c.currentParams = data[paramStart:paramEnd]
		c.paramsPos = 0

		// Execute command callback
		if cmd.Callback != nil {
			result := cmd.Callback(c)
			if result != ResOK {
				if !c.cmdError {
					c.ErrorPush(&Error{Code: -200, Info: "Execution error"})
				}
			}
		}

		// Skip terminator
		if !state.isEOS() {
			tok, _ := state.lexSemicolon()
			if tok.Type == TokenUnknown {
				state.lexNewLine()
			}
		}

		// Write output newline if needed
		if !c.firstOutput {
			c.writeNewLine()
		}
	}

	return nil
}

// Input processes incoming data and parses complete command lines
func (c *Context) Input(data []byte) error {
	if len(data) == 0 {
		// Parse what we have in buffer
		if c.bufferPos > 0 {
			err := c.Parse(c.inputBuffer[:c.bufferPos])
			c.bufferPos = 0
			return err
		}
		return nil
	}

	// Add data to buffer
	for _, b := range data {
		if c.bufferPos >= len(c.inputBuffer) {
			c.ErrorPush(&Error{Code: -350, Info: "Input buffer overflow"})
			c.bufferPos = 0
			return fmt.Errorf("input buffer overflow")
		}

		c.inputBuffer[c.bufferPos] = b
		c.bufferPos++

		// Check for line terminator
		if b == '\n' {
			// Parse complete line
			err := c.Parse(c.inputBuffer[:c.bufferPos])
			c.bufferPos = 0
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// IsCmd checks if the current command matches the given pattern
func (c *Context) IsCmd(pattern string) bool {
	if c.currentCmd == nil {
		return false
	}
	return matchCommand(pattern, c.currentCmd.Pattern)
}

// writeData writes data to output
func (c *Context) writeData(data []byte) (int, error) {
	if c.iface != nil && c.iface.Write != nil {
		return c.iface.Write(data)
	}
	return 0, nil
}

// writeNewLine writes a newline to output
func (c *Context) writeNewLine() error {
	c.writeData([]byte("\n"))
	if c.iface != nil && c.iface.Flush != nil {
		return c.iface.Flush()
	}
	return nil
}

// writeDelimiter writes a comma delimiter if needed
func (c *Context) writeDelimiter() {
	if c.outputCount > 0 {
		c.writeData([]byte(","))
	}
}

// ResultText writes a quoted string result
func (c *Context) ResultText(text string) error {
	c.writeDelimiter()
	c.writeData([]byte("\""))
	// Escape quotes in text
	escaped := strings.ReplaceAll(text, "\"", "\"\"")
	c.writeData([]byte(escaped))
	c.writeData([]byte("\""))
	c.outputCount++
	c.firstOutput = false
	return nil
}

// ResultInt32 writes a 32-bit integer result
func (c *Context) ResultInt32(value int32) error {
	c.writeDelimiter()
	c.writeData([]byte(fmt.Sprintf("%d", value)))
	c.outputCount++
	c.firstOutput = false
	return nil
}

// ResultInt64 writes a 64-bit integer result
func (c *Context) ResultInt64(value int64) error {
	c.writeDelimiter()
	c.writeData([]byte(fmt.Sprintf("%d", value)))
	c.outputCount++
	c.firstOutput = false
	return nil
}

// ResultFloat writes a float32 result
func (c *Context) ResultFloat(value float32) error {
	c.writeDelimiter()
	c.writeData([]byte(fmt.Sprintf("%g", value)))
	c.outputCount++
	c.firstOutput = false
	return nil
}

// ResultDouble writes a float64 result
func (c *Context) ResultDouble(value float64) error {
	c.writeDelimiter()
	c.writeData([]byte(fmt.Sprintf("%g", value)))
	c.outputCount++
	c.firstOutput = false
	return nil
}

// ResultBool writes a boolean result (0 or 1)
func (c *Context) ResultBool(value bool) error {
	if value {
		return c.ResultInt32(1)
	}
	return c.ResultInt32(0)
}

// ResultMnemonic writes a character data result
func (c *Context) ResultMnemonic(data string) error {
	c.writeDelimiter()
	c.writeData([]byte(data))
	c.outputCount++
	c.firstOutput = false
	return nil
}
