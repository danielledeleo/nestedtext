package parse

import "fmt"

// Token is a type for communicating between the line-level scanner and the parser.
// The scanner will read lines and wrap the content into parser tags, i.e., tokens for the
// parser to perform its operations on.
type Token struct {
	LineNo, ColNo int       // start of the tag within the input source
	TokenType     TokenType // type of token
	Indent        int       // amount of indent of this line
	Content       []string  // UTF-8 content of the line (without indent and item tag)
	Error         error     // error condition, if any
}

//go:generate stringer -type=TokenType
type TokenType int8

const (
	Undefined TokenType = iota
	EOF
	EmptyDocument
	DocRoot
	ListItem
	ListItemMultiline
	StringMultiline
	DictKeyMultiline
	InlineList
	InlineDict
	InlineDictKeyValue
	InlineDictKey
)

// NewToken creates a parser token initialized with line and column index.
func NewToken(line, col int) *Token {
	return &Token{
		LineNo:  line,
		ColNo:   col,
		Content: []string{},
	}
}

func (token *Token) String() string {
	return fmt.Sprintf("token[at(%d,%d) ind=%d type=%d %#v]", token.LineNo, token.ColNo, token.Indent,
		token.TokenType, token.Content)
}

// InlineTokenType represents token types for inline parsing
type InlineTokenType int8

const (
	Character InlineTokenType = iota
	Whitespace
	Newline
	Comma
	Colon
	ListOpen
	ListClose
	DictOpen
	DictClose
)
