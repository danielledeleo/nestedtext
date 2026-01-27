package parse

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

// maxNestingDepth limits recursion to prevent stack overflow from malicious input.
const maxNestingDepth = 5000

// Parser is a recursive-descend parser working on a grammar on input lines.
// The scanner is expected to return line by line wrapped into `Token`.
type Parser struct {
	Sc          *Scanner          // line level scanner
	Token       *Token            // the current token from the scanner
	Inline      *InlineItemParser // sub-parser for inline lists/dicts
	TopLevel    string            // type of top-level item
	MinimalMode bool              // if true, reject inline syntax and multi-line keys
	Stack       Stack             // parser stack
	depth       int               // current nesting depth

	// Error creation functions
	MakeFormatError  func(string) error
	MakeParsingError func(token *Token, code int, msg string) error
	ErrCodeFormat    int
}

// NewParser creates a new parser with the given error creation functions.
func NewParser(makeFormatError func(string) error, wrapIOError func(string, error) error, makeParsingError func(*Token, int, string) error, errCodeFormat int) *Parser {
	p := &Parser{
		Inline:           NewInlineParser(wrapIOError, makeParsingError, errCodeFormat),
		Stack:            make([]StackEntry, 0, 10),
		MakeFormatError:  makeFormatError,
		MakeParsingError: makeParsingError,
		ErrCodeFormat:    errCodeFormat,
	}
	return p
}

// Parse parses the input from r and returns the result.
func (p *Parser) Parse(r io.Reader, makeFormatError func(string) error, wrapIOError func(string, error) error, errCodeNoInput int) (result interface{}, err error) {
	p.Sc, err = NewScanner(r, makeFormatError, wrapIOError, p.MakeParsingError, p.ErrCodeFormat, errCodeNoInput)
	if err != nil {
		return
	}
	result, err = p.parseDocument()
	if err == nil {
		result = p.WrapResult(result)
	}
	return
}

func (p *Parser) parseDocument() (result interface{}, err error) {
	// initial token from scanner is a health check for the input source
	if p.Token = p.Sc.NextToken(); p.Token.Error != nil {
		return nil, p.Token.Error
	}
	if p.Token.TokenType == EOF || p.Token.TokenType == EmptyDocument {
		return nil, nil
	}
	// read the first item line
	if p.Token = p.Sc.NextToken(); p.Token.Error != nil {
		return nil, p.Token.Error
	}
	result, err = p.parseAny(0)
	if err == nil && p.Token.TokenType != EOF {
		err = p.MakeParsingError(p.Token, p.ErrCodeFormat,
			"unused content following valid input")
	}
	return
}

func (p *Parser) parseAny(indent int) (result interface{}, err error) {
	if p.Token.Indent < indent {
		return nil, nil
	}
	if p.depth >= maxNestingDepth {
		return nil, p.MakeFormatError("exceeded max nesting depth")
	}
	p.depth++
	defer func() { p.depth-- }()
	switch p.Token.TokenType {
	case StringMultiline:
		result, err = p.parseMultiString(p.Token.Indent)
	case InlineList:
		if p.MinimalMode {
			return nil, p.MakeParsingError(p.Token, p.ErrCodeFormat,
				"inline list syntax is not allowed in minimal mode")
		}
		p.Inline.LineNo = p.Token.LineNo
		result, err = p.Inline.Parse(StateS2, p.Token.Content[0], p.MakeFormatError)
		if err == nil {
			if p.Token = p.Sc.NextToken(); p.Token.Error != nil {
				return nil, p.Token.Error
			}
		}
	case InlineDict:
		if p.MinimalMode {
			return nil, p.MakeParsingError(p.Token, p.ErrCodeFormat,
				"inline dict syntax is not allowed in minimal mode")
		}
		p.Inline.LineNo = p.Token.LineNo
		result, err = p.Inline.Parse(StateS1, p.Token.Content[0], p.MakeFormatError)
		if err == nil {
			if p.Token = p.Sc.NextToken(); p.Token.Error != nil {
				return nil, p.Token.Error
			}
		}
	case ListItem, ListItemMultiline:
		result, err = p.parseList(indent)
	case InlineDictKeyValue, InlineDictKey, DictKeyMultiline:
		if p.MinimalMode && p.Token.TokenType == DictKeyMultiline {
			return nil, p.MakeParsingError(p.Token, p.ErrCodeFormat,
				"multi-line key syntax is not allowed in minimal mode")
		}
		result, err = p.parseDict(indent)
	default:
		return nil, p.MakeFormatError(fmt.Sprintf("internal error: unknown item type %d", p.Token.TokenType))
	}
	return
}

func (p *Parser) parseList(indent int) (result interface{}, err error) {
	p.pushNonterm(false)
	_, err = p.parseListItems(p.Token.Indent)
	if err != nil {
		return nil, err
	}
	result, err = p.Stack.Tos().ReduceToItem()
	p.Stack.Pop()
	return
}

func (p *Parser) parseListItems(indent int) (result interface{}, err error) {
	var value interface{}
	for p.Token.TokenType == ListItem || p.Token.TokenType == ListItemMultiline {
		if p.Token.TokenType == ListItem {
			value, err = p.parseListItem(indent)
		} else {
			value, err = p.parseListItemMultiline(indent)
		}
		if value != nil && err == nil {
			if pushErr := p.Stack.PushKV(nil, value, p.MakeFormatError); pushErr != nil {
				return nil, pushErr
			}
		} else if err != nil {
			return
		} else if value == nil {
			break
		}
	}
	return p.Stack.Tos().Values, err
}

func (p *Parser) parseListItem(indent int) (result interface{}, err error) {
	if p.Token.Indent > indent {
		return nil, p.MakeFormatError(
			"invalid indent: may only follow an item that does not already have a value")
	}
	if p.Token.Indent < indent {
		return nil, nil
	}
	value := p.Token.Content[0]
	if p.Token = p.Sc.NextToken(); p.Token.Error != nil {
		return nil, p.Token.Error
	}
	return value, err
}

func (p *Parser) parseListItemMultiline(indent int) (result interface{}, err error) {
	if p.Token.Indent != indent {
		return nil, nil
	}
	if p.Token = p.Sc.NextToken(); p.Token.Error != nil {
		return nil, p.Token.Error
	}
	if p.Token.Indent <= indent {
		return "", nil
	}
	result, err = p.parseAny(p.Token.Indent)
	if p.Token.Indent > indent {
		return nil, p.MakeFormatError(
			"invalid indent: may only follow an item that does not already have a value")
	}
	return
}

func (p *Parser) parseDict(indent int) (result interface{}, err error) {
	p.pushNonterm(true)
	_, err = p.parseDictKeyValuePairs(p.Token.Indent)
	if err != nil {
		return nil, err
	}
	result, err = p.Stack.Tos().ReduceToItem()
	p.Stack.Pop()
	if p.Token.Indent > indent {
		err = p.MakeFormatError("partial dedent")
	}
	return
}

// keyValuePair is a helper type to hold dict key-values as return-type.
type keyValuePair struct {
	key   *string
	value interface{}
}

func (p *Parser) parseDictKeyValuePairs(indent int) (result interface{}, err error) {
	var kv keyValuePair
	for p.Token.TokenType == InlineDictKeyValue || p.Token.TokenType == InlineDictKey ||
		p.Token.TokenType == DictKeyMultiline {
		//
		switch p.Token.TokenType {
		case InlineDictKeyValue:
			kv, err = p.parseDictKeyValuePair(indent)
		case InlineDictKey:
			kv, err = p.parseDictKeyAnyValuePair(indent)
		case DictKeyMultiline:
			if p.MinimalMode {
				return nil, p.MakeParsingError(p.Token, p.ErrCodeFormat,
					"multi-line key syntax is not allowed in minimal mode")
			}
			kv, err = p.parseDictKeyValuePairWithMultilineKey(indent)
		}
		if kv.value != nil {
			if err != nil {
				return
			}
			if pushErr := p.Stack.PushKV(kv.key, kv.value, p.MakeFormatError); pushErr != nil {
				return nil, pushErr
			}
		} else {
			break
		}
	}
	return p.Stack.Tos().Keys, err
}

func (p *Parser) parseDictKeyValuePair(indent int) (kv keyValuePair, err error) {
	if p.Token.Indent != indent {
		return
	}
	key := p.Token.Content[0]
	value := p.Token.Content[1]
	if p.Token = p.Sc.NextToken(); p.Token.Error != nil {
		return kv, p.Token.Error
	}
	return keyValuePair{key: &key, value: value}, err
}

func (p *Parser) parseDictKeyAnyValuePair(indent int) (kv keyValuePair, err error) {
	if p.Token.Indent != indent {
		return
	}
	kv.key = &p.Token.Content[0]
	if p.Token = p.Sc.NextToken(); p.Token.Error != nil {
		return kv, p.Token.Error
	}
	if p.Token.Indent <= indent {
		kv.value = ""
		return
	}
	kv.value, err = p.parseAny(p.Token.Indent)
	return
}

func allowVoid(val []string, i int) string {
	if val == nil || len(val) <= i {
		return ""
	}
	return val[i]
}

func (p *Parser) parseDictKeyValuePairWithMultilineKey(indent int) (kv keyValuePair, err error) {
	if p.Token.Indent != indent {
		return
	}
	builder := strings.Builder{}
	builder.WriteString(allowVoid(p.Token.Content, 0))
	for err == nil {
		p.Token = p.Sc.NextToken()
		if p.Token.Error != nil {
			return kv, p.Token.Error
		}
		if p.Token.TokenType != DictKeyMultiline || p.Token.Indent != indent {
			break
		}
		builder.WriteRune('\n')
		builder.WriteString(allowVoid(p.Token.Content, 0))
	}
	key := builder.String()
	kv.key = &key
	// Multiline key MUST be followed by an indented value
	if p.Token.Indent <= indent {
		return kv, p.MakeFormatError("multiline key requires a value")
	}
	kv.value, err = p.parseAny(p.Token.Indent)
	return
}

func (p *Parser) parseMultiString(indent int) (result interface{}, err error) {
	if p.Token.Indent != indent {
		return nil, nil
	}
	builder := strings.Builder{}
	builder.WriteString(allowVoid(p.Token.Content, 0))
	for err == nil {
		p.Token = p.Sc.NextToken()
		if p.Token.Error != nil {
			return builder.String(), p.Token.Error
		}
		if p.Token.TokenType != StringMultiline || p.Token.Indent != indent {
			break
		}
		builder.WriteRune('\n')
		builder.WriteString(allowVoid(p.Token.Content, 0))
	}
	return builder.String(), nil
}

func (p *Parser) pushNonterm(isDict bool) {
	entry := StackEntry{
		Values: make([]interface{}, 0, 16),
	}
	if isDict { // dict
		entry.Keys = make([]string, 0, 16)
	}
	p.Stack.Push(&entry)
}

// WrapResult wraps the result according to the TopLevel option.
func (p *Parser) WrapResult(result interface{}) interface{} {
	switch p.TopLevel {
	case "":
		// do nothing
	case "list":
		v := reflect.ValueOf(result)
		if v.Kind() != reflect.Slice {
			result = []interface{}{result}
		}
	case "dict":
		v := reflect.ValueOf(result)
		if v.Kind() != reflect.Map {
			result = map[string]interface{}{
				"nestedtext": result,
			}
		}
	default:
		result = map[string]interface{}{
			p.TopLevel: result,
		}
	}
	return result
}
