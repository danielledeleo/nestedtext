package nestedtext

import (
	"io"

	"github.com/danielledeleo/nestedtext/internal/parse"
)

// === Top-level API =========================================================

// Parse reads a NestedText input source and outputs a resulting hierarchy of values.
// Values are stored as strings, []interface{} or map[string]interface{} respectively.
// The concrete resulting top-level type depends on the top-level NestedText input type.
//
// If a non-nil error is returned, it will be of type NestedTextError.
func Parse(r io.Reader, opts ...DecodeOption) (interface{}, error) {
	p := parse.NewParser(makeFormatError, wrapIOError, makeParsingError, ErrCodeFormat)
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, err
		}
	}
	return p.Parse(r, makeFormatError, wrapIOError, ErrCodeFormatNoInput)
}

// --- Parser options --------------------------------------------------------

// DecodeOption configures the behavior of the parsing/decoding process.
// Multiple options may be passed to Parse, Unmarshal, or NewDecoder.
type DecodeOption func(*parse.Parser) error

// Minimal returns a DecodeOption that enables Minimal NestedText mode.
// In Minimal mode, the parser rejects:
//   - Inline list syntax: [...]
//   - Inline dict syntax: {...}
//   - Multi-line key syntax: ": key" prefix
//
// This enforces the Minimal NestedText subset as defined at
// https://nestedtext.org/en/latest/minimal-nestedtext.html
func Minimal() DecodeOption {
	return func(p *parse.Parser) error {
		p.MinimalMode = true
		return nil
	}
}

// --- Error helper functions for internal package ---------------------------

func makeFormatError(msg string) error {
	return MakeNestedTextError(ErrCodeFormat, msg)
}

func wrapIOError(msg string, err error) error {
	return WrapError(ErrCodeIO, msg, err)
}

func makeParsingError(token *parse.Token, code int, msg string) error {
	err := NestedTextError{
		Code: code,
		msg:  msg,
	}
	if token != nil {
		err.Line = token.LineNo
		err.Column = token.ColNo
	}
	return err
}
