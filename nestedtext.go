// Package nestedtext provides tools for processing NestedText, a human friendly data format.
// For more information on NestedText see
// https://nestedtext.org .
//
// To get a feel for the NestedText format, take a look at the following example
// (shortened version from the NestedText site):
/*
   # Contact information for our officers

   president:
      name: Katheryn McDaniel
      address:
         > 138 Almond Street
         > Topeka, Kansas 20697
      phone:
         cell: 1-210-555-5297
         home: 1-210-555-8470
            # Katheryn prefers that we always call her on her cell phone.
      email: KateMcD@aol.com
      additional roles:
         - board member

   vice president:
      name: Margaret Hodge
      …
*/
// NestedText is somewhat reminiscent of YAML, without the complexity of the latter and
// without the sometimes confusing details of interpretation.
// NestedText does not interpret any data types (unlike YAML), nor does it impose a schema.
// All of that has to be done by the application.
//
// # Parsing NestedText
//
// Parse is the low-level API that returns interface{} values:
//
//	input := `
//	# Example for a NestedText dict
//	a: Hello
//	b: World
//	`
//
//	result, err := Parse(strings.NewReader(input))
//	if err != nil {
//	    log.Fatal("parsing failed")
//	}
//	fmt.Printf("result = %#v\n", result)
//
// will yield:
//
//	result = map[string]interface {}{"a":"Hello", "b":"World"}
//
// # Unmarshaling into structs
//
// For type-safe parsing, use Unmarshal with struct tags:
//
//	type Config struct {
//	    Name string `nt:"name"`
//	    Port int    `nt:"port"`
//	}
//
//	var config Config
//	err := Unmarshal([]byte(input), &config)
//
// # Encoding to NestedText
//
// Use Marshal to encode Go values to NestedText:
//
//	data, err := Marshal(config)
//
// Or use Encode for streaming to an io.Writer.
package nestedtext

import "fmt"

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

// MakeNestedTextError creates a NestedTextError with a given error code and message.
func MakeNestedTextError(code int, errMsg string) NestedTextError {
	err := NestedTextError{
		Code: code,
		msg:  errMsg,
	}
	return err
}

// WrapError wraps an error into a NestedTextError
func WrapError(code int, errMsg string, err error) NestedTextError {
	e := MakeNestedTextError(code, errMsg)
	e.wrappedError = err
	return e
}

// Unmarshaler is the interface implemented by types that can unmarshal
// a NestedText value of themselves. The input can be a string, []interface{},
// or map[string]interface{} depending on the NestedText structure.
type Unmarshaler interface {
	UnmarshalNT(value interface{}) error
}
