package parse

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// Scanner is a type for a line-level scanner.
//
// Our line-level scanner will operate by calling scanning steps in a chain, iteratively.
// Each step function tests for valid lookahead and then possibly branches out to a
// subsequent step function. Step functions may consume input characters ("Match(…)").
type Scanner struct {
	Buf       *LineBuffer // line buffer abstracts away properties of input readers
	Step      ScannerStep // the next scanner step to execute in a chain
	LastError error       // last error, if any

	// Error creation function - set by the main package
	MakeParsingError func(token *Token, code int, msg string) error
	ErrCodeFormat    int
}

// We're building up a scanner from chains of scanner step functions.
// Tokens may be modified by a step function.
// A scanner step will return the next step in the chain, or nil to stop/accept.
type ScannerStep func(*Token) (*Token, ScannerStep)

// NewScanner creates a scanner for an input reader.
func NewScanner(inputReader io.Reader, makeFormatError func(string) error, wrapIOError func(string, error) error, makeParsingError func(*Token, int, string) error, errCodeFormat int, errCodeNoInput int) (*Scanner, error) {
	if inputReader == nil {
		return nil, makeParsingError(nil, errCodeNoInput, "no input present")
	}
	buf := NewLineBuffer(inputReader, makeFormatError, wrapIOError)
	sc := &Scanner{
		Buf:              buf,
		MakeParsingError: makeParsingError,
		ErrCodeFormat:    errCodeFormat,
	}
	sc.Step = sc.ScanFileStart
	return sc, nil
}

// NextToken will be called by the parser to receive the next line-level token. A token
// subsumes the properties of a line of NestedText input (excluding inline-items such
// as "{ key:val, key:val }" ).
//
// NextToken usually will iterate over a chain of step functions until it reaches an
// accepting state. Acceptance is signalled by getting a nil-step return value from a
// step function, meaning there is no further step applicable in this chain.
//
// If a step function returns an error-signalling token, the chaining stops as well.
func (sc *Scanner) NextToken() *Token {
	token := NewToken(sc.Buf.CurrentLine, int(sc.Buf.Cursor))
	if sc.Buf.IsEof() {
		token.TokenType = EOF
		return token
	}
	// Check for errors from previous operations (e.g., UTF-8 validation)
	if sc.Buf.LastError != nil {
		token.Error = sc.Buf.LastError
		return token
	}
	if sc.Step == nil {
		sc.Step = sc.ScanItem
	}
	for sc.Step != nil {
		token, sc.Step = sc.Step(token)
		if token.Error != nil {
			sc.LastError = token.Error
			sc.Buf.AdvanceLine()
			break
		}
		if sc.Buf.Line.Size() == 0 {
			break
		}
	}
	return token
}

// ScanFileStart matches a valid start of a NestedText document input. This is always the
// first step function to call.
//
//	file start:
//	  -> EOF:   EmptyDocument
//	  -> other: DocRoot
func (sc *Scanner) ScanFileStart(token *Token) (*Token, ScannerStep) {
	token.TokenType = EmptyDocument
	if sc.Buf == nil {
		token.Error = sc.MakeParsingError(token, sc.ErrCodeFormat, "no valid input document")
		return token, nil
	}
	if sc.Buf.IsEof() {
		return token, nil
	}
	token.TokenType = DocRoot
	token.Indent = 0
	if sc.Buf.Lookahead == ' ' {
		// From the spec: There is no indentation on the top-level object.
		token.Error = sc.MakeParsingError(token, sc.ErrCodeFormat+1, "top-level item must not be indented")
	}
	return token, nil
}

// ScanItem is a step function to start recognizing a line-level item.
// Checks for invalid whitespace (tabs, Unicode whitespace) at the start of a line.
func (sc *Scanner) ScanItem(token *Token) (*Token, ScannerStep) {
	if sc.Buf.Lookahead == ' ' {
		return token, sc.ScanIndentation
	}
	// Check for invalid whitespace at start of line (tabs or Unicode whitespace)
	if sc.Buf.Lookahead == '\t' {
		token.Error = sc.MakeParsingError(token, sc.ErrCodeFormat,
			"invalid character in indentation: tab")
		return token, nil
	}
	if unicode.IsSpace(sc.Buf.Lookahead) && sc.Buf.Lookahead != '\n' {
		token.Error = sc.MakeParsingError(token, sc.ErrCodeFormat,
			fmt.Sprintf("invalid character in indentation: %#U", sc.Buf.Lookahead))
		return token, nil
	}
	return token, sc.ScanItemBody
}

// ScanIndentation is a step function to recognize the indentation part of an item.
// Only ASCII spaces are allowed in indentation (no tabs or Unicode whitespace).
func (sc *Scanner) ScanIndentation(token *Token) (*Token, ScannerStep) {
	if sc.Buf.Lookahead == ' ' {
		sc.Buf.Match(SingleRune(' '))
		token.Indent++
		return token, sc.ScanIndentation
	}
	// Check for invalid indentation characters (tabs or Unicode whitespace)
	if sc.Buf.Lookahead == '\t' {
		token.Error = sc.MakeParsingError(token, sc.ErrCodeFormat,
			"invalid character in indentation: tab")
		return token, nil
	}
	if unicode.IsSpace(sc.Buf.Lookahead) && sc.Buf.Lookahead != '\n' {
		token.Error = sc.MakeParsingError(token, sc.ErrCodeFormat,
			fmt.Sprintf("invalid character in indentation: %#U", sc.Buf.Lookahead))
		return token, nil
	}
	return token, sc.ScanItemBody
}

// ScanItemBody is a step function to recognize the main part of an item, starting at
// the item's tag (e.g., ':', '>', etc.). The only exception are inline keys and inline key-value-pairs,
// which start with the key's string.
func (sc *Scanner) ScanItemBody(token *Token) (*Token, ScannerStep) {
	switch sc.Buf.Lookahead {
	case '-': // list value, either single-line or multi-line. From the spec:
		// If the first non-space character on a line is a dash followed immediately by a space (-␣) or
		// a line break, the line is a list item.
		sc.Buf.Match(SingleRune('-'))
		switch sc.Buf.Lookahead {
		case ' ', '\n': // yes, this is a valid list tag
			return sc.recognizeItemTag('-', ListItem, ListItemMultiline, token), nil
		default: // rare case: '-' as start of a dict key
			return token, sc.ScanInlineKey
		}
	case '>': // multi-line string. From the spec:
		// If the first non-space character on a line is a greater-than symbol followed immediately by
		// a space (>␣) or a line break, the line is a string item.
		sc.Buf.Match(SingleRune('>'))
		switch sc.Buf.Lookahead {
		case ' ', '\n': // yes, this is a valid string tag
			return sc.recognizeItemTag('>', StringMultiline, StringMultiline, token), nil
		default: // rare case: '>' as start of a dict key
			return token, sc.ScanInlineKey
		}
	case ':': // multi-line key. From the spec:
		// If the first non-space character on a line is a colon followed immediately by a space (:␣) or
		// a line break, the line is a key item.
		sc.Buf.Match(SingleRune(':'))
		switch sc.Buf.Lookahead {
		case ' ', '\n': // yes, this is a valid dict-key tag
			return sc.recognizeItemTag(':', DictKeyMultiline, DictKeyMultiline, token), nil
		default: // rare case: ':' as start of a dict-key
			return token, sc.ScanInlineKey
		}
	case '[': // single-line list
		return sc.recognizeInlineItem(InlineList, token), nil
	case '{': // single-line dictionary
		return sc.recognizeInlineItem(InlineDict, token), nil
	default: // should be dictionary key
	}
	return token, sc.ScanInlineKey // 'epsilon-transition' to inline-key-value rules
}

// ScanInlineKey is a step function to recognize an inline key, optionally followed by an inline
// value.
func (sc *Scanner) ScanInlineKey(token *Token) (*Token, ScannerStep) {
	switch sc.Buf.Lookahead { // consume characters; stop on ': ', ':\n' or EOL
	case ':':
		sc.Buf.Match(SingleRune(':'))
		switch sc.Buf.Lookahead {
		case ' ': // yes, this is a valid dict-key tag
			// remove trailing whitespace from key (=> Content[0])
			key := sc.Buf.Text[token.Indent : sc.Buf.ByteCursor-2]
			token.Content = append(token.Content, strings.TrimSpace(key))
			token = sc.recognizeItemTag(':', InlineDictKeyValue, InlineDictKey, token)
		case EOLMarker: // yes, this is a valid dict-key tag
			// remove trailing whitespace from key (=> Content[0])
			key := sc.Buf.Text[token.Indent : sc.Buf.ByteCursor-1]
			token.Content = append(token.Content, strings.TrimSpace(key))
			token = sc.recognizeItemTag(':', InlineDictKeyValue, InlineDictKey, token)
		default: // rare case: ':' inside a dict key
			return token, sc.ScanInlineKey
		}
	case EOLMarker: // Error: premature end of line
		key := sc.Buf.Text[token.Indent : sc.Buf.ByteCursor-1]
		token.Error = sc.MakeParsingError(token, sc.ErrCodeFormat+3,
			fmt.Sprintf("dict key item %q not properly terminated by ':'", key))
	default: // recognize everything as either part of the key or trailing whitespace
		sc.Buf.Match(Anything())
		return token, sc.ScanInlineKey
	}
	return token, nil
}

// recognizeItemTag continues after a valid item tag has been discovered. It will
// match the second character of the tag (either a space or a newline) and,
// depending on this character, select the continuation call.
func (sc *Scanner) recognizeItemTag(tag rune, single, multi TokenType, token *Token) *Token {
	if sc.Buf.Lookahead != ' ' && sc.Buf.Lookahead != EOLMarker {
		token.Error = sc.MakeParsingError(token, sc.ErrCodeFormat+3,
			fmt.Sprintf("item tag %q followed by illegal character %#U", tag, sc.Buf.Lookahead))
		return token
	}
	if sc.Buf.Lookahead == ' ' {
		sc.Buf.Match(SingleRune(' '))
		token.TokenType = single
		token.Content = append(token.Content, sc.Buf.ReadLineRemainder())
		return token
	}
	sc.Buf.Match(SingleRune(EOLMarker))
	token.TokenType = multi
	return token
}

func (sc *Scanner) recognizeInlineItem(toktype TokenType, token *Token) *Token {
	trimmed := strings.TrimSpace(sc.Buf.Text)
	closing := trimmed[len(trimmed)-1]
	if !isMatchingBracket(sc.Buf.Lookahead, rune(closing)) {
		token.Error = sc.MakeParsingError(token, sc.ErrCodeFormat+3,
			fmt.Sprintf("inline-item does not match opening tag: %#U vs %#U",
				sc.Buf.Lookahead, rune(closing)))
	}
	token.TokenType = toktype
	token.Content = append(token.Content, sc.Buf.ReadLineRemainder())
	return token
}

func isMatchingBracket(open, close rune) bool {
	if open == '[' {
		return close == ']'
	}
	if open == '{' {
		return close == '}'
	}
	return false
}
