package scpi

// lexState represents the state of the lexer
type lexState struct {
	buffer []byte
	pos    int
	len    int
}

// isEOS checks if we're at the end of the stream
func (l *lexState) isEOS() bool {
	return l.pos >= l.len
}

// peek returns the current character without advancing
func (l *lexState) peek() byte {
	if l.isEOS() {
		return 0
	}
	return l.buffer[l.pos]
}

// advance moves the position forward by n bytes
func (l *lexState) advance(n int) {
	l.pos += n
	if l.pos > l.len {
		l.pos = l.len
	}
}

// isWhitespace checks if a character is whitespace
func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t'
}

// isAlpha checks if a character is alphabetic
func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isDigit checks if a character is a digit
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// isHexDigit checks if a character is a hex digit
func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// lexWhitespace consumes whitespace characters
func (l *lexState) lexWhitespace() (Token, int) {
	start := l.pos
	for !l.isEOS() && isWhitespace(l.peek()) {
		l.advance(1)
	}
	length := l.pos - start
	return Token{
		Type: TokenWhitespace,
		Data: l.buffer[start:l.pos],
		Pos:  start,
	}, length
}

// lexNewLine consumes newline characters
func (l *lexState) lexNewLine() (Token, int) {
	start := l.pos
	c := l.peek()

	if c == '\n' {
		l.advance(1)
		return Token{
			Type: TokenNewLine,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, 1
	} else if c == '\r' {
		l.advance(1)
		if !l.isEOS() && l.peek() == '\n' {
			l.advance(1)
		}
		return Token{
			Type: TokenNewLine,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, l.pos - start
	}

	return Token{Type: TokenUnknown}, 0
}

// lexSemicolon consumes a semicolon
func (l *lexState) lexSemicolon() (Token, int) {
	if l.peek() == ';' {
		start := l.pos
		l.advance(1)
		return Token{
			Type: TokenSemicolon,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, 1
	}
	return Token{Type: TokenUnknown}, 0
}

// lexComma consumes a comma
func (l *lexState) lexComma() (Token, int) {
	if l.peek() == ',' {
		start := l.pos
		l.advance(1)
		return Token{
			Type: TokenComma,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, 1
	}
	return Token{Type: TokenUnknown}, 0
}

// lexColon consumes a colon
func (l *lexState) lexColon() (Token, int) {
	if l.peek() == ':' {
		start := l.pos
		l.advance(1)
		return Token{
			Type: TokenColon,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, 1
	}
	return Token{Type: TokenUnknown}, 0
}

// lexProgramHeader parses a SCPI command header
func (l *lexState) lexProgramHeader() (Token, int) {
	start := l.pos

	// Check for common command (*CMD)
	if l.peek() == '*' {
		l.advance(1)
		for !l.isEOS() && isAlpha(l.peek()) {
			l.advance(1)
		}

		// Check for query
		tokenType := TokenCommonProgramHeader
		if !l.isEOS() && l.peek() == '?' {
			l.advance(1)
		}

		if l.pos > start+1 {
			return Token{
				Type: tokenType,
				Data: l.buffer[start:l.pos],
				Pos:  start,
			}, l.pos - start
		}
		return Token{Type: TokenUnknown}, 0
	}

	// Parse compound command (CMD:CMD:CMD)
	// Start with optional leading colon
	if l.peek() == ':' {
		l.advance(1)
	}

	for {
		// Parse mnemonic part
		if !isAlpha(l.peek()) {
			break
		}

		for !l.isEOS() && (isAlpha(l.peek()) || isDigit(l.peek())) {
			l.advance(1)
		}

		// Check for optional numeric suffix (#)
		if !l.isEOS() && l.peek() == '#' {
			l.advance(1)
		}

		// Check for colon (more parts to come)
		if !l.isEOS() && l.peek() == ':' {
			l.advance(1)
			continue
		}

		break
	}

	// Check for query
	if !l.isEOS() && l.peek() == '?' {
		l.advance(1)
	}

	if l.pos > start {
		return Token{
			Type: TokenCompoundProgramHeader,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, l.pos - start
	}

	return Token{Type: TokenUnknown}, 0
}

// lexDecimalNumeric parses decimal numeric data
func (l *lexState) lexDecimalNumeric() (Token, int) {
	start := l.pos

	// Optional sign
	if l.peek() == '+' || l.peek() == '-' {
		l.advance(1)
	}

	// Integer part
	hasDigits := false
	for !l.isEOS() && isDigit(l.peek()) {
		l.advance(1)
		hasDigits = true
	}

	// Optional decimal point
	if !l.isEOS() && l.peek() == '.' {
		l.advance(1)
		// Fractional part
		for !l.isEOS() && isDigit(l.peek()) {
			l.advance(1)
			hasDigits = true
		}
	}

	// Optional exponent
	if !l.isEOS() && (l.peek() == 'e' || l.peek() == 'E') {
		l.advance(1)
		if !l.isEOS() && (l.peek() == '+' || l.peek() == '-') {
			l.advance(1)
		}
		for !l.isEOS() && isDigit(l.peek()) {
			l.advance(1)
		}
	}

	if hasDigits && l.pos > start {
		return Token{
			Type: TokenDecimalNumeric,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, l.pos - start
	}

	l.pos = start
	return Token{Type: TokenUnknown}, 0
}

// lexNondecimalNumeric parses hex, octal, or binary numeric data
func (l *lexState) lexNondecimalNumeric() (Token, int) {
	start := l.pos

	if l.peek() != '#' {
		return Token{Type: TokenUnknown}, 0
	}

	l.advance(1)
	if l.isEOS() {
		l.pos = start
		return Token{Type: TokenUnknown}, 0
	}

	base := l.peek()
	tokenType := TokenUnknown

	switch base {
	case 'H', 'h':
		tokenType = TokenHexNum
		l.advance(1)
		for !l.isEOS() && isHexDigit(l.peek()) {
			l.advance(1)
		}
	case 'Q', 'q':
		tokenType = TokenOctNum
		l.advance(1)
		for !l.isEOS() && (l.peek() >= '0' && l.peek() <= '7') {
			l.advance(1)
		}
	case 'B', 'b':
		tokenType = TokenBinNum
		l.advance(1)
		for !l.isEOS() && (l.peek() == '0' || l.peek() == '1') {
			l.advance(1)
		}
	default:
		l.pos = start
		return Token{Type: TokenUnknown}, 0
	}

	if l.pos > start+2 {
		return Token{
			Type: tokenType,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, l.pos - start
	}

	l.pos = start
	return Token{Type: TokenUnknown}, 0
}

// lexCharacterProgramData parses character/mnemonic data
func (l *lexState) lexCharacterProgramData() (Token, int) {
	start := l.pos

	if !isAlpha(l.peek()) {
		return Token{Type: TokenUnknown}, 0
	}

	for !l.isEOS() && (isAlpha(l.peek()) || isDigit(l.peek()) || l.peek() == '_') {
		l.advance(1)
	}

	if l.pos > start {
		return Token{
			Type: TokenProgramMnemonic,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, l.pos - start
	}

	return Token{Type: TokenUnknown}, 0
}

// lexStringProgramData parses quoted string data
func (l *lexState) lexStringProgramData() (Token, int) {
	start := l.pos
	quote := l.peek()

	if quote != '"' && quote != '\'' {
		return Token{Type: TokenUnknown}, 0
	}

	tokenType := TokenDoubleQuoteData
	if quote == '\'' {
		tokenType = TokenSingleQuoteData
	}

	l.advance(1)

	for !l.isEOS() {
		c := l.peek()
		l.advance(1)

		if c == quote {
			// Check for escaped quote (double quote)
			if !l.isEOS() && l.peek() == quote {
				l.advance(1)
				continue
			}
			// End of string
			return Token{
				Type: tokenType,
				Data: l.buffer[start:l.pos],
				Pos:  start,
			}, l.pos - start
		}
	}

	// Unterminated string
	l.pos = start
	return Token{Type: TokenUnknown}, 0
}

// lexArbitraryBlock parses arbitrary block data (#<length><data>)
func (l *lexState) lexArbitraryBlock() (Token, int) {
	start := l.pos

	if l.peek() != '#' {
		return Token{Type: TokenUnknown}, 0
	}

	l.advance(1)
	if l.isEOS() || !isDigit(l.peek()) {
		l.pos = start
		return Token{Type: TokenUnknown}, 0
	}

	// Get the length of the length field
	lengthDigits := int(l.peek() - '0')
	l.advance(1)

	if lengthDigits == 0 {
		// Indefinite length - read until newline
		for !l.isEOS() && l.peek() != '\n' && l.peek() != '\r' {
			l.advance(1)
		}
		return Token{
			Type: TokenArbitraryBlock,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, l.pos - start
	}

	// Parse the length value
	length := 0
	for i := 0; i < lengthDigits && !l.isEOS() && isDigit(l.peek()); i++ {
		length = length*10 + int(l.peek()-'0')
		l.advance(1)
	}

	// Read the data
	dataStart := l.pos
	if dataStart+length <= l.len {
		l.advance(length)
		return Token{
			Type: TokenArbitraryBlock,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, l.pos - start
	}

	l.pos = start
	return Token{Type: TokenUnknown}, 0
}

// lexProgramExpression parses program expressions (...)
func (l *lexState) lexProgramExpression() (Token, int) {
	start := l.pos

	if l.peek() != '(' {
		return Token{Type: TokenUnknown}, 0
	}

	depth := 0
	for !l.isEOS() {
		c := l.peek()
		l.advance(1)

		if c == '(' {
			depth++
		} else if c == ')' {
			depth--
			if depth == 0 {
				return Token{
					Type: TokenProgramExpression,
					Data: l.buffer[start:l.pos],
					Pos:  start,
				}, l.pos - start
			}
		}
	}

	// Unmatched parentheses
	l.pos = start
	return Token{Type: TokenUnknown}, 0
}

// lexSuffixProgramData parses unit suffixes
func (l *lexState) lexSuffixProgramData() (Token, int) {
	start := l.pos

	if !isAlpha(l.peek()) {
		return Token{Type: TokenUnknown}, 0
	}

	for !l.isEOS() && isAlpha(l.peek()) {
		l.advance(1)
	}

	if l.pos > start {
		return Token{
			Type: TokenSuffixProgramData,
			Data: l.buffer[start:l.pos],
			Pos:  start,
		}, l.pos - start
	}

	return Token{Type: TokenUnknown}, 0
}
