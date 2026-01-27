package parse

import (
	"fmt"
	"strings"
	"testing"
)

// Error helpers for tests
func testFormatError(msg string) error {
	return fmt.Errorf("format: %s", msg)
}

func testIOError(msg string, err error) error {
	return fmt.Errorf("io: %s: %w", msg, err)
}

func testParsingError(token *Token, code int, msg string) error {
	return fmt.Errorf("[%d,%d] %s", token.LineNo, token.ColNo, msg)
}

const testErrCodeFormat = 200

func TestLineBufferSplitter(t *testing.T) {
	inputDoc := strings.NewReader("Hello\nWorld\r?!\n")
	buf := NewLineBuffer(inputDoc, testFormatError, testIOError)
	buf.AdvanceCursor()
	r := buf.ReadLineRemainder()
	t.Logf("line: %q\n", r)
	if r != "ello" {
		t.Errorf("first line terminated by '\\n' not recognized?")
	}
	buf.AdvanceCursor()
	r = buf.ReadLineRemainder()
	t.Logf("line: %q\n", r)
	if r != "orld" {
		t.Errorf("second line terminated by '\\r' not recognized?")
	}
	buf.AdvanceCursor()
	r = buf.ReadLineRemainder()
	t.Logf("line: %q\n", r)
	if r != "!" {
		t.Errorf("last line terminated by '\\n' not recognized?")
	}
}

func TestLineBufferRemainder(t *testing.T) {
	inputDoc := strings.NewReader("Hello World\nHow are you?")
	buf := NewLineBuffer(inputDoc, testFormatError, testIOError)
	for i := 0; i < 6; i++ {
		buf.AdvanceCursor()
	}
	r := buf.ReadLineRemainder()
	t.Logf("remainder = '%s'", r)
	if r != "World" {
		t.Errorf("expected remainder to be 'World', is '%s'", r)
	}
	r = buf.ReadLineRemainder()
	t.Logf("remainder = '%s'", r)
	if r != "How are you?" {
		t.Errorf("expected remainder to be 'How are you?', is '%s'", r)
	}
}

func TestLineBufferAnyRuneOf(t *testing.T) {
	inputDoc := strings.NewReader("abc123")
	buf := NewLineBuffer(inputDoc, testFormatError, testIOError)

	// Test matching 'a'
	if !buf.Match(AnyRuneOf('a', 'b', 'c')) {
		t.Errorf("expected to match 'a' with AnyRuneOf('a', 'b', 'c')")
	}

	// Advance and test again
	buf.AdvanceCursor()
	if !buf.Match(AnyRuneOf('a', 'b', 'c')) {
		t.Errorf("expected to match 'b' with AnyRuneOf('a', 'b', 'c')")
	}

	// Test non-matching
	buf.AdvanceCursor()
	buf.AdvanceCursor()
	if buf.Match(AnyRuneOf('a', 'b', 'c')) {
		t.Errorf("expected '1' to not match AnyRuneOf('a', 'b', 'c')")
	}
}

func TestScannerCreate(t *testing.T) {
	r := strings.NewReader("")
	_, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
}

func TestScannerStart(t *testing.T) {
	r := strings.NewReader("# This is a comment to skip\n")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	if sc.Buf.isEof == 0 {
		t.Errorf("expected scanner to be at EOF, isn't")
	}
}

func TestScannerTopLevelIndent(t *testing.T) {
	r := strings.NewReader("# This is a comment\n   debug: false\n")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	tok := sc.NextToken()
	if tok.Error == nil {
		t.Errorf("tok.Error to reflect error code ErrCodeFormatToplevelIndent")
	}
}

func TestScannerUTF8(t *testing.T) {
	r := strings.NewReader("$€¥£₩₺₽₹ɃΞȄ: $€¥£₩₺₽₹ɃΞȄ")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	var token *Token
	_ = sc.NextToken() // doc root
	token = sc.NextToken()
	logToken(token, t)
	if token.Content == nil || token.Content[0] != "$€¥£₩₺₽₹ɃΞȄ" {
		t.Fatalf("UTF-8 decoding problem?")
	}
}

func TestScannerTerminate(t *testing.T) {
	r := strings.NewReader("> This is a string\n> and this too\n?    ")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	var token *Token
	token = sc.NextToken()
	logToken(token, t)
	token = sc.NextToken()
	logToken(token, t)
	token = sc.NextToken()
	logToken(token, t)
	token = sc.NextToken()
	logToken(token, t)
	token = sc.NextToken()
	logToken(token, t)
}

func TestScannerListItem(t *testing.T) {
	r := strings.NewReader("# This is a comment\n- debug\n")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	tok := sc.NextToken()
	if tok.Error != nil {
		t.Errorf("top-level document root expected to parse without error; didn't: %v", tok.Error)
	}
	tok = sc.NextToken()
	t.Logf("token = %v", tok)
	if tok.TokenType != ListItem {
		t.Errorf("item expected to be of type list item; is: %d", tok.TokenType)
	}
	if tok.Content[0] != "debug" {
		t.Errorf("item expected to have value 'debug'; is: %s", tok.Content)
	}
}

func TestScannerListItemIllegal(t *testing.T) {
	r := strings.NewReader("# This is a comment\n-debug\n")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	sc.NextToken()        // doc root
	tok := sc.NextToken() // item
	logToken(tok, t)
	if tok.Error == nil {
		t.Errorf("item expected to have error; hasn't")
	} else {
		t.Logf("Error caught: %v", tok.Error)
	}
}

func TestScannerLongListItem(t *testing.T) {
	r := strings.NewReader("# This is a comment\n-\n > debug\n")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	tok := sc.NextToken()
	if tok.Error != nil {
		t.Errorf("top-level document root expected to parse without error; didn't: %v", tok.Error)
	}
	tok = sc.NextToken()
	logToken(tok, t)
	if tok.TokenType != ListItemMultiline {
		t.Errorf("item expected to be of type multiline list item; is: %d", tok.TokenType)
	}
}

func TestScannerMultilineString(t *testing.T) {
	r := strings.NewReader("> Hello\n> World!\n")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	sc.NextToken()        // doc root
	tok := sc.NextToken() // string
	logToken(tok, t)
	if tok.TokenType != StringMultiline {
		t.Errorf("item expected to be of type multiline string; is: %d", tok.TokenType)
	}
	tok = sc.NextToken() // string
	logToken(tok, t)
	if tok.TokenType != StringMultiline {
		t.Errorf("item expected to be of type multiline string; is: %d", tok.TokenType)
	}
}

func TestScannerMultilineKey(t *testing.T) {
	r := strings.NewReader(": Hello\n  : Key\n")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	sc.NextToken()        // doc root
	tok := sc.NextToken() // key
	logToken(tok, t)
	if tok.TokenType != DictKeyMultiline {
		t.Errorf("item expected to be of type multiline key; is: %d", tok.TokenType)
	}
	tok = sc.NextToken() // key
	logToken(tok, t)
	if tok.TokenType != DictKeyMultiline {
		t.Errorf("item expected to be of type multiline key; is: %d", tok.TokenType)
	}
}

func TestScannerInlineError(t *testing.T) {
	r := strings.NewReader("[ hello, world }")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	sc.NextToken()        // doc root
	tok := sc.NextToken() // inline list with errors
	logToken(tok, t)
	if tok.TokenType != InlineList {
		t.Errorf("item expected to be of type inline list; is: %d", tok.TokenType)
	}
	if tok.Error == nil {
		t.Errorf("expected inline item to carry an error, doesn't")
	}
}

func TestScannerInlineDictKeyValue(t *testing.T) {
	r := strings.NewReader("Hello  : World!\n")
	sc, err := NewScanner(r, testFormatError, testIOError, testParsingError, testErrCodeFormat, testErrCodeFormat+1)
	if err != nil {
		t.Fatal(err)
	}
	sc.NextToken()        // doc root
	tok := sc.NextToken() // dict key-value pair
	logToken(tok, t)
	if tok.TokenType != InlineDictKeyValue {
		t.Errorf("item expected to be of type inline key-value; is: %d", tok.TokenType)
	}
}

// ---------------------------------------------------------------------------

func logToken(token *Token, t *testing.T) {
	t.Logf("token = %v", token)
	if token.Error != nil {
		t.Logf("      + error:  %v", token.Error)
	}
}
