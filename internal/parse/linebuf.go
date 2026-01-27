package parse

import (
	"bufio"
	"errors"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// LineBuffer is an abstraction of a NestedText document source.
// The scanner will use a LineBuffer for input.
type LineBuffer struct {
	Lookahead   rune            // the next UTF-8 character
	Cursor      int64           // position of lookahead in character count
	ByteCursor  int64           // position of lookahead in byte count
	CurrentLine int             // current line number, starting at 1 (= next "expected line")
	Input       *bufio.Scanner  // we use this to break up input into lines
	Text        string          // holds a copy of Input
	Line        *strings.Reader // reader on Text
	isEof       int             // is this buffer done reading? May be 0, 1 or 2.
	LastError   error           // last error, if any (except EOF errors)

	// Error creation functions - set by the main package
	MakeFormatError func(msg string) error
	WrapIOError     func(msg string, err error) error
}

const EOLMarker = '\n'

var ErrAtEof = errors.New("EOF")

func NewLineBuffer(inputDoc io.Reader, makeFormatError func(string) error, wrapIOError func(string, error) error) *LineBuffer {
	input := bufio.NewScanner(inputDoc)
	// From the spec:
	// Line breaks: A NestedText document is partitioned into lines where the lines are split by
	// CR LF, CR, or LF where CR and LF are the ASCII carriage return and line feed characters.
	// A single document may employ any or all of these ways of splitting lines.
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = bufio.ScanLines(data, atEOF)
		for i, ch := range data {
			if ch == '\r' {
				if i < len(data)-1 && i < len(token) {
					if data[i+1] != '\n' {
						advance = i + 1
						token = data[:i]
						err = nil
						return
					}
				}
			}
		}
		return
	}
	input.Split(split)
	buf := &LineBuffer{
		Input:           input,
		MakeFormatError: makeFormatError,
		WrapIOError:     wrapIOError,
	}
	err := buf.AdvanceLine()
	if err != ErrAtEof {
		buf.LastError = err
	}
	// Strip BOM (Byte Order Mark) at start of document if present
	// BOM is U+FEFF
	if buf.Lookahead == '\uFEFF' {
		buf.AdvanceCursor()
	}
	return buf
}

func (buf *LineBuffer) IsEof() bool {
	return buf.isEof >= 2 || buf.Line.Size() == 0
}

// AdvanceCursor moves the rune cursor within the current line one character forward.
// If the cursor is already at the end of the line, `EOLMarker` is returned. No moving to
// the next line is performed.
func (buf *LineBuffer) AdvanceCursor() error {
	if buf.isEof > 2 {
		return ErrAtEof
	}
	if buf.ByteCursor >= buf.Line.Size() { // at end of line, set lookahead to EOLMarker
		buf.Lookahead = EOLMarker
	} else {
		r, err := buf.readRune()
		if err != nil {
			return err
		}
		buf.Lookahead = r
	}
	return nil
}

func (buf *LineBuffer) readRune() (rune, error) {
	r, runeLen, readerErr := buf.Line.ReadRune()
	if readerErr != nil {
		return 0, buf.WrapIOError("I/O error while reading input character", readerErr)
	}
	// Check for invalid UTF-8: ReadRune returns RuneError with width 1 for invalid bytes
	if r == utf8.RuneError && runeLen == 1 {
		return 0, buf.MakeFormatError("invalid UTF-8 byte sequence")
	}
	buf.ByteCursor += int64(runeLen)
	buf.Cursor++
	return r, nil
}

// AdvanceLine will advance the input buffer to the next line. Will return ErrAtEof if EOF has been
// encountered.
//
// Blank lines and comment lines are skipped. This may be a somewhat questionable decision in terms
// of separation of concerns, as empty lines and comments are artifacts for which the scanner should
// take care of. However, it makes implementing the scanner rules much more convenient
//
// Lookahead will be set to first rune (UFT-8 character) of the resulting current line.
// Line-count and cursor are updated.
func (buf *LineBuffer) AdvanceLine() error {
	buf.Cursor = 0
	buf.ByteCursor = 0
	// iterate over the lines of the input document until valid line found or EOF
	if buf.isEof == 1 {
		buf.isEof = 2
		return ErrAtEof
	}
	for buf.isEof == 0 {
		buf.CurrentLine++
		if !buf.Input.Scan() { // could not read a new line: either I/O-error or EOF
			if err := buf.Input.Err(); err != nil {
				return buf.WrapIOError("I/O error while reading input", err)
			}
			buf.isEof = 1
			buf.Line = strings.NewReader("")
			return ErrAtEof
		}
		buf.Text = buf.Input.Text()
		buf.Line = strings.NewReader(buf.Text) // Set Line early to prevent nil pointer issues
		// Validate UTF-8
		if !utf8.ValidString(buf.Text) {
			return buf.MakeFormatError("invalid UTF-8 byte sequence")
		}
		if !buf.IsIgnoredLine() {
			break
		}
	}
	return buf.AdvanceCursor()
}

var blankPattern *regexp.Regexp
var commentPattern *regexp.Regexp

// IsIgnoredLine is a predicate for the current line of input. From the spec:
// Blank lines are lines that are empty or consist only of white space characters (spaces or tabs).
// Comments are lines that have # as the first non-white-space character on the line.
func (buf *LineBuffer) IsIgnoredLine() bool {
	if blankPattern == nil {
		blankPattern = regexp.MustCompile(`^\s*$`)
		commentPattern = regexp.MustCompile(`^\s*#`)
	}
	if blankPattern.MatchString(buf.Text) || commentPattern.MatchString(buf.Text) {
		return true
	}
	return false
}

// ReadLineRemainder returns the remainder of the current line of input text.
// This is a frequent operation for NestedText items.
func (buf *LineBuffer) ReadLineRemainder() string {
	var s string
	if buf.IsEof() {
		s = ""
	} else if buf.Lookahead == EOLMarker {
		// At end of line (value is empty), return empty string
		s = ""
	} else if buf.ByteCursor == buf.Line.Size() {
		// At last character, return just that character
		s = string(buf.Lookahead)
	} else if buf.ByteCursor > buf.Line.Size() {
		s = ""
	} else {
		s = string(buf.Lookahead) + buf.Text[buf.ByteCursor:buf.Line.Size()]
	}
	buf.LastError = buf.AdvanceLine()
	return s
}

// The scanner has to match UTF-8 characters (runes) from the input. Matching is done using
// predicate functions (instead of direct comparison).

// SingleRune returns a predicate to match a single rune
func SingleRune(r rune) func(rune) bool {
	return func(arg rune) bool {
		return arg == r
	}
}

// Anything returns a predicate that matches any rune.
func Anything(runes ...rune) func(rune) bool {
	return func(rune) bool {
		return true
	}
}

// AnyRuneOf returns a predicate to match a single rune out of a set of runes.
func AnyRuneOf(runes ...rune) func(rune) bool {
	return func(arg rune) bool {
		for _, r := range runes {
			if arg == r {
				return true
			}
		}
		return false
	}
}

func (buf *LineBuffer) Match(predicate func(rune) bool) bool {
	if buf.IsEof() || buf.LastError != nil {
		return false
	}
	if !predicate(buf.Lookahead) {
		return false
	}
	var err error
	if buf.Lookahead == EOLMarker {
		err = buf.AdvanceLine()
	} else {
		err = buf.AdvanceCursor()
	}
	if err != nil && err != ErrAtEof {
		buf.LastError = err
		return false
	}
	return true
}
