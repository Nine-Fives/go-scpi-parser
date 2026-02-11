package scpi

// Result represents the result of SCPI command execution
type Result int

const (
	ResOK  Result = 1
	ResErr Result = -1
)

// TokenType represents the type of token parsed
type TokenType int

const (
	TokenComma TokenType = iota
	TokenSemicolon
	TokenColon
	TokenQuestion
	TokenNewLine
	TokenHexNum
	TokenOctNum
	TokenBinNum
	TokenProgramMnemonic
	TokenDecimalNumeric
	TokenDecimalNumericWithSuffix
	TokenSuffixProgramData
	TokenArbitraryBlock
	TokenSingleQuoteData
	TokenDoubleQuoteData
	TokenProgramExpression
	TokenCompoundProgramHeader
	TokenCommonProgramHeader
	TokenWhitespace
	TokenInvalid
	TokenUnknown
)

// Token represents a parsed token
type Token struct {
	Type TokenType
	Data []byte
	Pos  int
}

// MessageTermination represents how a message was terminated
type MessageTermination int

const (
	TerminationNone MessageTermination = iota
	TerminationNewLine
	TerminationSemicolon
)

// Command represents a SCPI command definition
type Command struct {
	Pattern  string
	Callback func(*Context) Result
	Tag      int32 // Optional command tag
}

// Error represents a SCPI error
type Error struct {
	Code int16
	Info string // Device-dependent info
}

// Interface defines the callbacks for SCPI I/O operations
type Interface struct {
	Write   func(data []byte) (int, error)
	Flush   func() error
	Reset   func() error
	OnError func(err *Error)
}

// Context represents the SCPI parser context
type Context struct {
	commands      []*Command
	iface         *Interface
	inputBuffer   []byte
	bufferPos     int
	outputCount   int
	inputCount    int
	firstOutput   bool
	cmdError      bool
	errorQueue    []*Error
	currentCmd    *Command
	currentParams []byte
	paramsPos     int
	userContext   interface{}
	idn           [4]string
}

// ArrayFormat represents the format for array data
type ArrayFormat int

const (
	FormatASCII       ArrayFormat = 0
	FormatBigEndian   ArrayFormat = 1
	FormatLittleEndian ArrayFormat = 2
)

// Unit represents SCPI units
type Unit int

const (
	UnitNone Unit = iota
	UnitVolt
	UnitAmper
	UnitOhm
	UnitHertz
	UnitCelsius
	UnitSecond
	UnitMeter
	UnitFarad
	UnitWatt
	UnitDecibel
	// Add more units as needed
)

// UnitDef defines a unit with its multiplier
type UnitDef struct {
	Name string
	Unit Unit
	Mult float64
}

// ChoiceDef defines a choice option
type ChoiceDef struct {
	Name string
	Tag  int32
}

// SpecialNumber represents special numeric values
type SpecialNumber int

const (
	NumNumber SpecialNumber = iota
	NumMin
	NumMax
	NumDef
	NumUp
	NumDown
	NumNaN
	NumInf
	NumNInf
	NumAuto
)

// Number represents a numeric parameter with optional unit
type Number struct {
	Special bool
	Value   float64
	Tag     int32
	Unit    Unit
	Base    int8
}

// Parameter is an alias for Token
type Parameter Token
