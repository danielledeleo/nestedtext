package nestext

import (
	"io"
	"strings"

	"github.com/npillmayer/nestext/internal/parse"
)

// === Top-level API =========================================================

// Parse reads a NestedText input source and outputs a resulting hierarchy of values.
// Values are stored as strings, []interface{} or map[string]interface{} respectively.
// The concrete resulting top-level type depends on the top-level NestedText input type.
//
// If a non-nil error is returned, it will be of type NestedTextError.
func Parse(r io.Reader, opts ...Option) (interface{}, error) {
	p := parse.NewParser(makeFormatError, wrapIOError, makeParsingError, ErrCodeFormat)
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, err
		}
	}
	return p.Parse(r, makeFormatError, wrapIOError, ErrCodeFormatNoInput)
}

// --- Parser options --------------------------------------------------------

// Option is a type to influence the behaviour of the parsing process.
// Multiple options may be passed to `Parse(…)`.
type Option func(*parse.Parser) error

// TopLevel determines the top-level type of the return value from parsing.
// Possible values are "list" and "dict". "list" will force the result to be an
// []interface{} (of possibly one item), while "dict" will force the result to be of
// type map[string]interface.
//
// For "dict", if the result is not a dict naturally, it will be wrapped in a map with a single
// key = "nestedtext". However, if the dict-option is given with a suffix (separated by '.'), the
// suffix string will be used as the top-level key. In this case, even naturally parsed dicts will
// be wrapped into a map with a single key (= the suffix to "dict.").
//
// Use as:
//
//	nestext.Parse(reader, nestext.TopLevel("dict.config"))
//
// This will result in a return-value of map[string]interface{} with a single entry
// map["config"] = …
//
// The default is for the parsing-result to be of the natural type corresponding to the
// top-level item of the input source.
// Option-strings other than "list" and "dict"/"dict.<suffix>" will result in an error
// returned by Parse(…).
func TopLevel(top string) Option {
	return func(p *parse.Parser) (err error) {
		switch top {
		case "dict":
			p.TopLevel = "dict"
		case "list":
			p.TopLevel = "list"
		default:
			if strings.HasPrefix(top, "dict.") {
				p.TopLevel = top[5:]
			} else {
				return MakeNestedTextError(ErrCodeUsage, `option TopLevel( "list" | "dict"(".<suffix>")? )`)
			}
		}
		return nil
	}
}

// KeepLegacyBidi requests the parser to keep Unicode LTR and RTL markers.
//
// Attention: This option is not yet functional!
func KeepLegacyBidi(keep bool) Option {
	// Default behaviour should be to strip LTR and RTL legacy control characters.
	// For security reasons applications should usually treat LTR/RTL cautiously when read
	// in from external sources. You can find various sources on the internet discussion
	// this problem, including a policy in place at GitHub.
	return func(p *parse.Parser) (err error) {
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
