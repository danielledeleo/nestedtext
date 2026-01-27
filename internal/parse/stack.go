package parse

import "fmt"

// Stack is the parser stack = slice of stack entries
type Stack []StackEntry

func (s Stack) Tos() *StackEntry {
	if len(s) > 0 {
		return &s[len(s)-1]
	}
	return nil
}

func (s *Stack) Pop() (tos *StackEntry) {
	if len(*s) > 0 {
		tos = s.Tos()
		*s = (*s)[:len(*s)-1]
	}
	return tos
}

func (s *Stack) Push(e *StackEntry) (tos *StackEntry) {
	if len(*s) > 0 {
		tos = s.Tos()
	}
	*s = append(*s, *e)
	return tos
}

// PushKV will push a value and an option key onto the stack by appending it to the
// top-most stack entry.
// The containing stack-entry has to be provided by a non-term (pushNonterm).
// Returns an error if a duplicate key is detected.
func (s *Stack) PushKV(str *string, val interface{}, makeFormatError func(string) error) error {
	if s == nil || len(*s) == 0 {
		panic("use of un-initialized parser stack")
	}
	tos := &(*s)[len(*s)-1]
	if str != nil {
		if tos.Keys == nil {
			return makeFormatError("unexpected key in non-dict context")
		}
		// Check for duplicate key
		for _, k := range tos.Keys {
			if k == *str {
				return makeFormatError(fmt.Sprintf("duplicate key: %s", *str))
			}
		}
		tos.Keys = append(tos.Keys, *str)
	}
	tos.Values = append(tos.Values, val)
	return nil
}

// StackEntry represents the parser stack entry for a non-terminal.
// Stack entries collect the information for an item, either a list or a dict.
type StackEntry struct {
	Values       []interface{}      // list of values, either list items or dict values
	Keys         []string           // list of keys, empty for list items
	Key          *string            // current key to set value for, if in a dict
	Error        error              // if error occurred: remember it
	NontermState InlineParserState  // sub-nonterm, or 0 for root entry (used for inline-parser only)
}

func (entry StackEntry) ReduceToItem() (interface{}, error) {
	if entry.Keys == nil {
		return entry.Values, nil
	}
	dict := make(map[string]interface{}, len(entry.Values))
	if len(entry.Keys) > 0 && len(entry.Values) != len(entry.Keys) {
		panic(fmt.Sprintf("mixed item: number of keys (%d) not equal to number of values (%d)",
			len(entry.Keys), len(entry.Values)))
	}
	for i, key := range entry.Keys {
		dict[key] = entry.Values[i]
	}
	return dict, nil
}

// InlineParserState represents states in the inline parser automaton
type InlineParserState int8

// Inline parser states
const (
	StateError InlineParserState = -1   // error state
	StateS1    InlineParserState = 11   // non-terminal S1 (dict)
	StateS2    InlineParserState = 12   // non-terminal S2 (list)
	StateA1    InlineParserState = 13   // acceptance state A1
	StateA2    InlineParserState = 14   // acceptance state A2
)

// IsErrorState is a predicate on parser states.
func IsErrorState(state InlineParserState) bool {
	return state < 0
}

// IsNonterm is a predicate on parser states.
func IsNonterm(state InlineParserState) bool {
	return state == StateS1 || state == StateS2
}

// IsAccept is a predicate on parser states.
func IsAccept(state InlineParserState) bool {
	return state == StateA1 || state == StateA2
}

// IsGhost is a predicate on parser states. It returns true if state is a
// "ghost state" (dashed line in the automata.dot diagram) which follows the
// acceptance of a nested non-terminal.
func IsGhost(state InlineParserState) bool {
	return state == 5 || state == 10
}
