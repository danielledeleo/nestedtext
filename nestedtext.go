// Package nestedtext provides encoding and decoding of NestedText, a human-friendly data format.
// See https://nestedtext.org for the format specification.
//
// # Unmarshaling
//
// Use [Unmarshal] to decode NestedText into Go structs with automatic type coercion:
//
//	var config struct {
//	    Name string `nt:"name"`
//	    Port int    `nt:"port"`
//	}
//	err := nestedtext.Unmarshal(data, &config)
//
// # Marshaling
//
// Use [Marshal] to encode Go values to NestedText:
//
//	data, err := nestedtext.Marshal(config)
//
// # Options
//
// Both encoding and decoding functions accept options:
//
//	// Decode with Minimal mode (reject inline syntax)
//	result, err := nestedtext.Parse(r, nestedtext.Minimal())
//
//	// Encode with custom indentation
//	data, err := nestedtext.Marshal(v, nestedtext.WithIndent(4))
//
// # Minimal NestedText
//
// Minimal NestedText is a subset that excludes inline lists, inline dicts,
// and multi-line keys. Use [Minimal] for decoding and [WithMinimal] for encoding.
//
// # Low-level API
//
// Use [Parse] for dynamic data that returns interface{} (string, []interface{},
// or map[string]interface{}). Use [NewEncoder] and [NewDecoder] for streaming.
package nestedtext

import (
	"fmt"
	"strings"
)

// --- Error type ------------------------------------------------------------

// NestedTextError is a custom error type for working with NestedText instances.
type NestedTextError struct {
	Code         int // error code
	Line, Column int // error position
	msg          string
	wrappedError error
}

// We use a custom error type which contains a numeric error code.
const (
	NoError       = 0
	ErrCodeUsage  = 1   // erroneous API call
	ErrCodeIO     = 10  // error will wrap an underlying I/O error
	ErrCodeSchema = 100 // schema violation; error may wrap an underlying error

	// all errors rooted in format violations have code >= ErrCodeFormat
	ErrCodeFormat               = 200 + iota // NestedText format error
	ErrCodeFormatNoInput                     // NestedText format error: no input present
	ErrCodeFormatToplevelIndent              // NestedText format error: top-level item was indented
	ErrCodeFormatIllegalTag                  // NestedText format error: tag not recognized

	// Unmarshal errors
	ErrCodeUnmarshal     // unmarshal error
	ErrCodeUnmarshalType // type mismatch during unmarshal
)

// Error produces an error message from a NestedText error.
func (e NestedTextError) Error() string {
	return fmt.Sprintf("[%d,%d] %s", e.Line, e.Column, e.msg)
}

// Unwrap returns an optionally present underlying error condition, e.g., an I/O-Error.
func (e NestedTextError) Unwrap() error {
	return e.wrappedError
}

// makeNestedTextError creates a NestedTextError with a given error code and message.
func makeNestedTextError(code int, errMsg string) NestedTextError {
	err := NestedTextError{
		Code: code,
		msg:  errMsg,
	}
	return err
}

// wrapError wraps an error into a NestedTextError.
func wrapError(code int, errMsg string, err error) NestedTextError {
	e := makeNestedTextError(code, errMsg)
	e.wrappedError = err
	return e
}

// Unmarshaler is the interface implemented by types that can unmarshal
// a NestedText value of themselves. The input can be a string, []interface{},
// or map[string]interface{} depending on the NestedText structure.
type Unmarshaler interface {
	UnmarshalNT(value interface{}) error
}

// --- Struct tag parsing -----------------------------------------------------

// ntTagOptions holds the parsed options from a struct field's "nt" tag.
type ntTagOptions struct {
	name      string // custom field name (empty if not specified)
	omitEmpty bool   // omitempty option present
	ignore    bool   // field should be ignored (tag == "-")
}

// parseNTTag parses a struct field's "nt" tag and returns the options.
// Tag format: "name,omitempty" or "-" to ignore the field.
func parseNTTag(tag string) ntTagOptions {
	var opts ntTagOptions
	if tag == "-" {
		opts.ignore = true
		return opts
	}
	if tag == "" {
		return opts
	}
	parts := strings.Split(tag, ",")
	if parts[0] != "" {
		opts.name = parts[0]
	}
	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			opts.omitEmpty = true
		}
	}
	return opts
}
