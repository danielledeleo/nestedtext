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
	d := &Decoder{}
	for _, opt := range opts {
		if err := opt(d); err != nil {
			return nil, err
		}
	}
	return parseWithConfig(r, d.minimalMode, nil)
}

// ParseOrdered is like Parse but returns *Dict instead of map[string]interface{}
// for dictionaries, preserving the insertion order of keys as defined in the
// NestedText spec. Lists and strings are returned as []interface{} and string,
// the same as Parse.
func ParseOrdered(r io.Reader, opts ...DecodeOption) (interface{}, error) {
	d := &Decoder{}
	for _, opt := range opts {
		if err := opt(d); err != nil {
			return nil, err
		}
	}
	return parseWithConfig(r, d.minimalMode, orderedDictBuilder)
}

// orderedDictBuilder constructs a *Dict from parallel key/value slices.
var orderedDictBuilder parse.DictBuilder = func(keys []string, values []interface{}) interface{} {
	d := &Dict{entries: make([]DictEntry, len(keys))}
	for i, key := range keys {
		d.entries[i] = DictEntry{Key: key, Value: values[i]}
	}
	return d
}

// parseWithConfig is the internal parsing function that accepts configuration directly.
func parseWithConfig(r io.Reader, minimalMode bool, buildDict parse.DictBuilder) (interface{}, error) {
	p := parse.NewParser(makeFormatError, wrapIOError, makeParsingError, ErrCodeFormat)
	p.MinimalMode = minimalMode
	p.BuildDict = buildDict
	p.Inline.BuildDict = buildDict
	return p.Parse(r, makeFormatError, wrapIOError, ErrCodeFormatNoInput)
}

// --- Parser options --------------------------------------------------------

// DecodeOption configures the behavior of the parsing/decoding process.
// Multiple options may be passed to Parse, Unmarshal, or NewDecoder.
type DecodeOption func(*Decoder) error

// Minimal returns a DecodeOption that enables Minimal NestedText mode.
// In Minimal mode, the parser rejects:
//   - Inline list syntax: [...]
//   - Inline dict syntax: {...}
//   - Multi-line key syntax: ": key" prefix
//
// This enforces the Minimal NestedText subset as defined at
// https://nestedtext.org/en/latest/minimal-nestedtext.html
func Minimal() DecodeOption {
	return func(d *Decoder) error {
		d.minimalMode = true
		return nil
	}
}

// --- Error helper functions for internal package ---------------------------

func makeFormatError(msg string) error {
	return makeNestedTextError(ErrCodeFormat, msg)
}

func wrapIOError(msg string, err error) error {
	return wrapError(ErrCodeIO, msg, err)
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
