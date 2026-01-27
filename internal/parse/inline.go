package parse

import (
	"strings"
	"unicode"
)

// InlineItemParser parses inline lists and dicts.
// Inline items are lists or dicts as one-liners. Examples would be
//
//	[ one, two three ]
//	{ one:1, two:2, three:3 }
//
// or nested instances like
//
//	{ one:1, two:2, three:3, all: [1, 2, 3] }
//
// We use a scannerless parser. It suffices to construct the prefix-automaton for inline lists and
// inline dicts (see file "automata.dot"). The parser traverses the states of the automaton,
// performing an optional action at each of the states encountered. This way, the inline parser will
// collect strings as keys and/or values.
type InlineItemParser struct {
	Text         string          // current line of NestedText
	TextPosition int             // position of reader in string
	Marker       int             // positional marker for start of key or value
	Input        *strings.Reader // reader for Text
	LineNo       int             // current input line number
	Stack        Stack           // parser stack

	// Error creation functions
	WrapIOError     func(msg string, err error) error
	MakeFormatError func(token *Token, code int, msg string) error
	ErrCodeFormat   int
}

// NewInlineParser creates a fresh inline parser instance.
func NewInlineParser(wrapIOError func(string, error) error, makeFormatError func(*Token, int, string) error, errCodeFormat int) *InlineItemParser {
	return &InlineItemParser{
		Stack:           make([]StackEntry, 0, 10),
		WrapIOError:     wrapIOError,
		MakeFormatError: makeFormatError,
		ErrCodeFormat:   errCodeFormat,
	}
}

func (p *InlineItemParser) Parse(initial InlineParserState, input string, makeFormatError func(string) error) (result interface{}, err error) {
	p.Text = input
	p.Input = strings.NewReader(input)
	p.Stack = p.Stack[:0]
	p.TextPosition, p.Marker = 0, 0

	p.pushNonterm(initial)
	var oldState, state InlineParserState = 0, initial
	for len(p.Stack) > 0 {
		ch, w, err := p.Input.ReadRune()
		if err != nil {
			err = p.WrapIOError("I/O-error reading inline item", err)
			return nil, err
		}
		chType := InlineTokenFor(ch)
		oldState, state = state, inlineStateMachine[state][chType]
		if IsErrorState(state) {
			break
		} else if IsNonterm(state) {
			nonterm := state
			p.pushNonterm(state)
			state = inlineStateMachine[state][chType]
			p.Stack.Tos().NontermState = inlineStateMachine[oldState][stateIndex(nonterm)]
		}
		ok := inlineStateMachineActions[state](p, oldState, state, ch, w, makeFormatError)
		if !ok {
			state = StateError // flag error by setting error state
			break
		}
		if IsAccept(state) {
			result, err = p.Stack.Tos().ReduceToItem()
			if err != nil {
				p.Stack.Tos().Error = err
				state = StateError
				break
			}
			state = p.Stack.Tos().NontermState
			p.Stack.Pop()
			if len(p.Stack) > 0 {
				if pushErr := p.Stack.PushKV(p.Stack.Tos().Key, result, makeFormatError); pushErr != nil {
					p.Stack.Tos().Error = pushErr
					state = StateError
					break
				}
			}
		}
		p.TextPosition += w
	}
	if IsErrorState(state) {
		if err = p.Stack[len(p.Stack)-1].Error; err == nil {
			t := Token{ColNo: p.TextPosition, LineNo: p.LineNo}
			err = p.MakeFormatError(&t, p.ErrCodeFormat, "format error")
		}
	}
	// Check for trailing content after the inline item
	if err == nil && len(p.Stack) == 0 {
		remainder := strings.TrimSpace(p.Text[p.TextPosition:])
		if len(remainder) > 0 {
			t := Token{ColNo: p.TextPosition, LineNo: p.LineNo}
			err = p.MakeFormatError(&t, p.ErrCodeFormat,
				"extra characters after closing delimiter: \""+remainder+"\"")
		}
	}
	return
}

// pushNonterm pushes a new (empty) stack entry onto the parser stack. Depending on whether
// the non-terminal represents a list item or a dict item, the .Keys slice will be initialized.
func (p *InlineItemParser) pushNonterm(state InlineParserState) {
	entry := StackEntry{
		Values: make([]interface{}, 0, 16),
	}
	if state == StateS1 { // dict
		entry.Keys = make([]string, 0, 16)
	}
	p.Stack.Push(&entry)
}

func (p *InlineItemParser) appendStringValue(isAccept bool, makeFormatError func(string) error) error {
	value := p.Text[p.Marker:p.TextPosition]
	// From the spec:
	// Both inline lists and dictionaries may be empty, and represent the only way to
	// represent empty lists or empty dictionaries in NestedText. An empty dictionary
	// is represented with {} and an empty list with []. In both cases there must be
	// no space between the opening and closing delimiters. An inline list that contains
	// only white spaces, such as [ ], is treated as a list with a single empty string
	// (the whitespace is considered a string value, and string values have leading and
	// trailing spaces removed, resulting in an empty string value). If a list contains
	// multiple values, no white space is required to represent an empty string
	// Thus, [] represents an empty list, [ ] a list with a single empty string value,
	// and [,] a list with two empty string values.
	if p.Stack.Tos().Key != nil {
		value = strings.TrimSpace(value)
		return p.Stack.PushKV(p.Stack.Tos().Key, value, makeFormatError)
	} else if !isAccept || len(value) > 0 || len(p.Stack.Tos().Values) > 0 {
		value = strings.TrimSpace(value)
		return p.Stack.PushKV(p.Stack.Tos().Key, value, makeFormatError)
	}
	return nil
}

// InlineTokenFor returns the inline token type for a rune.
func InlineTokenFor(r rune) int {
	switch r {
	case ' ':
		return 1 // whitespace
	case '\n':
		return 2 // newline
	case ',':
		return 3 // comma
	case ':':
		return 4 // colon
	case '[':
		return 5 // listOpen
	case ']':
		return 6 // listClose
	case '{':
		return 7 // dictOpen
	case '}':
		return 8 // dictClose
	default:
		if unicode.IsSpace(r) {
			return 1 // whitespace
		}
		return 0 // character
	}
}

const chClassCnt = 11

// stateIndex returns a non-terminal state as a pseudo character class.
// This is used to determine the "ghost state" which follows the acceptance of a nested
// non-terminal.
func stateIndex(s InlineParserState) int {
	return int(s-StateS1) + chClassCnt - 2
}

// Character classes:
//
//	A  ws \n ,  :  [  ]  {  }  _S(S1) _S(S2)
var inlineStateMachine = [...][chClassCnt]InlineParserState{
	{StateError, StateError, StateError, StateError, StateError, 7, StateError, 1, StateError, StateError, StateError}, // state 0, initial
	{2, 2, StateError, StateError, 3, StateError, StateError, StateError, StateA1, StateError, StateError},             // state 1
	{2, 2, StateError, StateError, 3, StateError, StateError, StateError, StateError, StateError, StateError},          // state 2
	{4, 3, StateError, 6, StateError, StateS2, StateError, StateS1, StateA1, 5, 5},                                     // state 3
	{4, 4, StateError, 6, StateError, StateError, StateError, StateError, StateA1, StateError, StateError},             // state 4
	{StateError, 5, StateError, 6, StateError, StateError, StateError, StateError, StateA1, StateError, StateError},    // state 5
	{2, 6, StateError, StateError, 3, StateError, StateError, StateError, StateError, StateError, StateError},          // state 6
	{9, 8, StateError, 7, 9, StateS2, StateA2, StateS1, StateError, 10, 10},                                            // state 7
	{9, 8, StateError, 7, 9, StateS2, StateA2, StateS1, StateError, 10, 10},                                            // state 8
	{9, 9, StateError, 7, 9, StateError, StateA2, StateError, StateError, StateError, StateError},                      // state 9
	{StateError, 10, StateError, 7, StateError, StateError, StateA2, StateError, StateError, StateError, StateError},   // state 10
	{StateError, StateError, StateError, StateError, StateError, StateError, StateError, 1, StateError, StateError, StateError}, // state S1
	{StateError, StateError, StateError, StateError, StateError, 7, StateError, StateError, StateError, StateError, StateError}, // state S2
	{StateError, StateError, StateError, StateError, StateError, StateError, StateError, StateError, StateError, StateError, StateError}, // state A1
	{StateError, StateError, StateError, StateError, StateError, StateError, StateError, StateError, StateError, StateError, StateError}, // state A2
}

// Action function type for the inline parser state machine
type inlineAction func(p *InlineItemParser, from, to InlineParserState, ch rune, w int, makeFormatError func(string) error) bool

var inlineStateMachineActions = [...]inlineAction{
	nop, // 0
	func(p *InlineItemParser, from, to InlineParserState, ch rune, w int, makeFormatError func(string) error) bool { // 1
		p.Marker = p.TextPosition + w // get ready for first key
		return true
	},
	nop, // 2
	func(p *InlineItemParser, from, to InlineParserState, ch rune, w int, makeFormatError func(string) error) bool { // 3
		if from != 3 {
			key := p.Text[p.Marker:p.TextPosition]
			key = strings.TrimSpace(key)
			p.Stack.Tos().Key = &key
			p.Marker = p.TextPosition + w // get ready for value
		}
		return true
	},
	nop, // 4
	nop, // 5
	func(p *InlineItemParser, from, to InlineParserState, ch rune, w int, makeFormatError func(string) error) bool { // 6
		if from != 6 {
			if p.Marker > 0 && !IsGhost(from) {
				if err := p.appendStringValue(false, makeFormatError); err != nil {
					p.Stack.Tos().Error = err
					return false
				}
			}
			p.Stack.Tos().Key = nil
			p.Marker = p.TextPosition + w // get ready for next key
		}
		return true
	},
	func(p *InlineItemParser, from, to InlineParserState, ch rune, w int, makeFormatError func(string) error) bool { // 7
		if ch == ',' && p.Marker > 0 && !IsGhost(from) {
			if err := p.appendStringValue(false, makeFormatError); err != nil {
				p.Stack.Tos().Error = err
				return false
			}
		}
		p.Marker = p.TextPosition + w // get ready for next item
		return true
	},
	nop, // 8
	nop, // 9
	nop, // 10
	func(p *InlineItemParser, from, to InlineParserState, ch rune, w int, makeFormatError func(string) error) bool { // S1
		if from == 3 || from == 8 {
			p.Marker = 0
		}
		return true
	},
	func(p *InlineItemParser, from, to InlineParserState, ch rune, w int, makeFormatError func(string) error) bool { // S2
		if from == 3 || from == 8 {
			p.Marker = 0
		}
		return true
	},
	accept, // A1
	accept, // A2
}

// nop is a no-op state machine action.
func nop(p *InlineItemParser, from, to InlineParserState, ch rune, w int, makeFormatError func(string) error) bool {
	return true
}

func accept(p *InlineItemParser, from, to InlineParserState, ch rune, w int, makeFormatError func(string) error) bool {
	if p.Marker > 0 && !IsGhost(from) {
		if err := p.appendStringValue(true, makeFormatError); err != nil {
			p.Stack.Tos().Error = err
			return false
		}
	}
	return true
}
