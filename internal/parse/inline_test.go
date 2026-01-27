package parse

import (
	"fmt"
	"testing"
)

func testInlineIOError(msg string, err error) error {
	return fmt.Errorf("io: %s: %w", msg, err)
}

func testInlineFormatError(token *Token, code int, msg string) error {
	return fmt.Errorf("[%d,%d] %s", token.LineNo, token.ColNo, msg)
}

func testSimpleFormatError(msg string) error {
	return fmt.Errorf("format: %s", msg)
}

const testInlineErrCodeFormat = 200

func TestInlineParseEOF(t *testing.T) {
	p := NewInlineParser(testInlineIOError, testInlineFormatError, testInlineErrCodeFormat)
	_, err := p.Parse(StateS2, "", testSimpleFormatError)
	if err != nil {
		t.Logf("error = %q", err.Error())
	} else {
		t.Fatal("expected empty input to result in error; didn't")
	}
}

func TestInlineParseItemsTable(t *testing.T) {
	p := NewInlineParser(testInlineIOError, testInlineFormatError, testInlineErrCodeFormat)
	inputs := []struct {
		text    string
		initial InlineParserState
		output  string
	}{
		{"[]", StateS2, "[]"},
		{"[ ]", StateS2, "[]"},
		{"[x]", StateS2, "[x]"},
		{"[x,y]", StateS2, "[x y]"},
		{"[[]]", StateS2, "[[]]"},
		{"{}", StateS1, "map[]"},
		{"{:}", StateS1, "map[:]"},
		{"{a:x}", StateS1, "map[a:x]"},
		{"{a: [x]}", StateS1, "map[a:[x]]"},
		{"{a:[x,y] }", StateS1, "map[a:[x y]]"},
		{"{a: {b: x} }", StateS1, "map[a:map[b:x]]"},
		{"{ a : { A : 0 } , b : { B : 1 } }   ", StateS1, "map[a:map[A:0] b:map[B:1]]"},
		{"{a: {b:0, c:1}, d: {e:2, f:3}}", StateS1, "map[a:map[b:0 c:1] d:map[e:2 f:3]]"},
		{"[[11, 12, 13], [21, 22, 23]]", StateS2, "[[11 12 13] [21 22 23]]"},
	}
	for i, input := range inputs {
		r, err := p.Parse(input.initial, input.text, testSimpleFormatError)
		if err != nil {
			t.Errorf(err.Error())
		}
		t.Logf("[%2d] result = %v of type %#T", i, r, r)
		if fmt.Sprintf("%v", r) != input.output {
			t.Errorf("[%2d] however, expected %q", i, input.output)
		}
		t.Logf("------------------------------------------")
	}
}

func TestInlineParseErrors(t *testing.T) {
	p := NewInlineParser(testInlineIOError, testInlineFormatError, testInlineErrCodeFormat)

	// Test mismatched brackets
	_, err := p.Parse(StateS2, "[}", testSimpleFormatError)
	if err == nil {
		t.Errorf("expected error for mismatched brackets")
	} else {
		t.Logf("correctly got error: %v", err)
	}

	// Test unclosed bracket
	_, err = p.Parse(StateS2, "[a, b", testSimpleFormatError)
	if err == nil {
		t.Errorf("expected error for unclosed bracket")
	} else {
		t.Logf("correctly got error: %v", err)
	}

	// Test unclosed dict
	_, err = p.Parse(StateS1, "{a: b", testSimpleFormatError)
	if err == nil {
		t.Errorf("expected error for unclosed dict")
	} else {
		t.Logf("correctly got error: %v", err)
	}
}
