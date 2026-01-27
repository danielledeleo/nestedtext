package nestedtext

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// DefaultInlineLimit is the threshold above which lists and dicts are not encoded as inline lists/dicts.
const DefaultInlineLimit = 128

// MaxIndent is the maximum number of spaces used as indent.
// Indentation may be in the range of 1…MaxIndent.
const MaxIndent = 16

// Marshal returns the NestedText encoding of v.
//
// Marshal traverses the value v recursively. If an encountered value implements
// the Marshaler interface, Marshal calls its MarshalNT method to produce NestedText.
//
// Otherwise, Marshal uses the following type-dependent default encodings:
//
// Struct values encode as NestedText dicts. Each exported struct field becomes
// a member of the dict, using the field name as the key, unless the field is
// omitted for one of the reasons given below.
//
// The encoding of each struct field can be customized by the format string
// stored under the "nt" key in the struct field's tag. The format string gives
// the name of the field, possibly followed by a comma-separated list of options.
// The name may be empty in order to specify options without overriding the default
// field name.
//
// The "omitempty" option specifies that the field should be omitted from the
// encoding if the field has an empty value, defined as false, 0, a nil pointer,
// a nil interface value, and any empty array, slice, map, or string.
//
// As a special case, if the field tag is "-", the field is always omitted.
//
// Map keys must be strings; the map keys are sorted and used as dict keys.
//
// Slice and array values encode as NestedText lists.
//
// String values encode as NestedText strings.
//
// Integer and floating point values encode as NestedText strings containing
// the decimal representation of the number.
//
// Boolean values encode as the strings "true" or "false".
func Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Encoder writes NestedText values to an output stream.
type Encoder struct {
	w           io.Writer
	indentSize  int
	inlineLimit int
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:           w,
		indentSize:  2,
		inlineLimit: DefaultInlineLimit,
	}
}

// SetIndent sets the number of spaces per indentation level.
// The default is 2. Allowed values are 1…MaxIndent.
func (enc *Encoder) SetIndent(n int) {
	if n < 1 {
		n = 1
	} else if n > MaxIndent {
		n = MaxIndent
	}
	enc.indentSize = n
}

// SetInlineLimit sets the threshold above which lists and dicts are not inlined.
// Defaults to DefaultInlineLimit; may not exceed 2048.
func (enc *Encoder) SetInlineLimit(limit int) {
	if limit > 2048 {
		limit = 2048
	}
	enc.inlineLimit = limit
}

// Encode writes the NestedText encoding of v to the stream.
func (enc *Encoder) Encode(v interface{}) error {
	_, err := enc.encode(0, v, 0, nil)
	return err
}

// Marshaler is the interface implemented by types that can marshal themselves
// into valid NestedText. The returned value must be a string, []interface{},
// or map[string]interface{}.
type Marshaler interface {
	MarshalNT() (interface{}, error)
}

// encode is the top level function to encode data into NestedText format.
// It will be called recursively and therefore carries the current indentation depth
// as a parameter.
func (enc *Encoder) encode(indent int, tree interface{}, bcnt int, err error) (int, error) {
	if err != nil {
		return bcnt, err
	}

	// Check for Marshaler interface
	if m, ok := tree.(Marshaler); ok {
		v, marshalErr := m.MarshalNT()
		if marshalErr != nil {
			return bcnt, marshalErr
		}
		return enc.encode(indent, v, bcnt, nil)
	}

	if !isEncodable(tree) {
		return 0, MakeNestedTextError(ErrCodeSchema,
			fmt.Sprintf("unable to encode type %T", tree))
	}
	switch t := tree.(type) {
	// We first try a couple of standard-cases without relying on reflection
	case string:
		if ok, s := isInlineable(encAsString, t); ok {
			bcnt, err = enc.indent(bcnt, err, indent)
			bcnt, err = enc.wr(bcnt, err, []byte("> "))
			bcnt, err = enc.wr(bcnt, err, s)
			bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
		} else {
			S := strings.Split(t, "\n")
			for _, s := range S {
				bcnt, err = enc.indent(bcnt, err, indent)
				bcnt, err = enc.wr(bcnt, err, []byte{'>', ' '})
				bcnt, err = enc.wr(bcnt, err, []byte(s))
				bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
			}
		}
	case []string:
		if len(t) <= 5 { // max of 5 is completely arbitrary
			l := 0
			inlineable := true
			S := make([][]byte, len(t))
			for i, item := range t { // measure all list items
				l += len(item)
				ok, s := isInlineable(encAsList, item)
				inlineable = inlineable && ok
				if !inlineable || l > enc.inlineLimit {
					break // stop trying if not suited for inlining
				}
				S[i] = s
			}
			// if the complete array fits into one line, output "[ a, b, … ]"
			if inlineable && l <= enc.inlineLimit {
				bcnt, err = enc.indent(bcnt, err, indent)
				bcnt, err = enc.wr(bcnt, err, []byte{'['})
				for i, item := range t {
					if i > 0 {
						bcnt, err = enc.wr(bcnt, err, []byte{',', ' '})
					}
					bcnt, err = enc.wr(bcnt, err, []byte(item))
				}
				bcnt, err = enc.wr(bcnt, err, []byte{']', '\n'})
				break
			}
		}
		// general case: list item with '-' as tag
		for _, s := range t {
			bcnt, err = enc.indent(bcnt, err, indent)
			bcnt, err = enc.wr(bcnt, err, []byte{'-'})
			if strings.IndexByte(s, '\n') == -1 { // no newlines in string
				bcnt, err = enc.wr(bcnt, err, []byte{' '})
				bcnt, err = enc.wr(bcnt, err, []byte(s))
				bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
			} else { // contains newlines => item is multi-line string
				bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
				bcnt, err = enc.encode(indent+1, s, bcnt, err)
			}
		}
	case []int:
		if len(t) <= 10 { // max of 10 is completely arbitrary
			bcnt, err = enc.indent(bcnt, err, indent)
			bcnt, err = enc.wr(bcnt, err, []byte{'['})
			for i, n := range t {
				if i > 0 {
					bcnt, err = enc.wr(bcnt, err, []byte{',', ' '})
				}
				bcnt, err = enc.wr(bcnt, err, []byte(strconv.Itoa(n)))
			}
			bcnt, err = enc.wr(bcnt, err, []byte{']', '\n'})
			break
		}
		for _, n := range t {
			bcnt, err = enc.indent(bcnt, err, indent)
			bcnt, err = enc.wr(bcnt, err, []byte("- "))
			bcnt, err = enc.wr(bcnt, err, []byte(strconv.Itoa(n)))
			bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
		}
	case []interface{}:
		for _, item := range t {
			bcnt, err = enc.indent(bcnt, err, indent)
			bcnt, err = enc.wr(bcnt, err, []byte("-"))
			if ok, itemAsBytes := isInlineable(encAsList, item); ok {
				bcnt, err = enc.wr(bcnt, err, []byte{' '})
				bcnt, err = enc.wr(bcnt, err, itemAsBytes)
				bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
			} else {
				bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
				bcnt, err = enc.encode(indent+1, item, bcnt, err)
			}
		}
	case bool:
		bcnt, err = enc.indent(bcnt, err, indent)
		bcnt, err = enc.wr(bcnt, err, []byte("> "))
		if t {
			bcnt, err = enc.wr(bcnt, err, []byte("true"))
		} else {
			bcnt, err = enc.wr(bcnt, err, []byte("false"))
		}
		bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
	case int, int8, int16, int32, int64:
		bcnt, err = enc.indent(bcnt, err, indent)
		bcnt, err = enc.wr(bcnt, err, []byte("> "))
		bcnt, err = enc.wr(bcnt, err, []byte(fmt.Sprintf("%d", t)))
		bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
	case uint, uint8, uint16, uint32, uint64:
		bcnt, err = enc.indent(bcnt, err, indent)
		bcnt, err = enc.wr(bcnt, err, []byte("> "))
		bcnt, err = enc.wr(bcnt, err, []byte(fmt.Sprintf("%d", t)))
		bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
	case float32, float64:
		bcnt, err = enc.indent(bcnt, err, indent)
		bcnt, err = enc.wr(bcnt, err, []byte("> "))
		bcnt, err = enc.wr(bcnt, err, []byte(fmt.Sprintf("%v", t)))
		bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
	default:
		bcnt, err = enc.encodeReflected(indent, tree, bcnt, err)
	}
	return bcnt, err
}

// encodeReflected encodes container types slice, map, and struct using reflection.
func (enc *Encoder) encodeReflected(indent int, tree interface{}, bcnt int, err error) (int, error) {
	v := reflect.ValueOf(tree)
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return bcnt, nil
		}
		return enc.encode(indent, v.Elem().Interface(), bcnt, err)
	case reflect.Slice, reflect.Array:
		l := v.Len()
		for i := 0; i < l; i++ {
			item := v.Index(i).Interface()
			bcnt, err = enc.indent(bcnt, err, indent)
			bcnt, err = enc.wr(bcnt, err, []byte{'-'})
			if ok, itemAsBytes := isInlineable(encAsList, item); ok {
				bcnt, err = enc.wr(bcnt, err, []byte{' '})
				bcnt, err = enc.wr(bcnt, err, itemAsBytes)
				bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
			} else {
				bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
				bcnt, err = enc.encode(indent+1, item, bcnt, err)
			}
		}
	case reflect.Map:
		keys := v.MapKeys()
		// special case: empty map
		if len(keys) == 0 {
			return enc.wr(bcnt, err, []byte("{}\n"))
		}
		// first sort items alphabetically by key
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})
		for _, k := range keys {
			if k.Kind() != reflect.String {
				return 0, MakeNestedTextError(ErrCodeSchema,
					"map key is not a string; can only encode keys of type string")
			}
			key := k.Interface().(string)
			item := v.MapIndex(k).Interface()
			if ok, keyAsBytes := isInlineable(encAsKey, key); ok {
				bcnt, err = enc.indent(bcnt, err, indent)
				bcnt, err = enc.wr(bcnt, err, keyAsBytes)
				bcnt, err = enc.wr(bcnt, err, []byte{':'})
				if ok, itemAsBytes := isInlineable(encAsString, item); ok {
					bcnt, err = enc.wr(bcnt, err, []byte{' '})
					bcnt, err = enc.wr(bcnt, err, itemAsBytes)
					bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
				} else {
					bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
					bcnt, err = enc.encodeIfNotEmpty(item, indent, bcnt, err)
				}
			} else { // output key as a multi-line key
				S := strings.Split(key, "\n")
				for _, s := range S {
					bcnt, err = enc.indent(bcnt, err, indent)
					if s == "" {
						bcnt, err = enc.wr(bcnt, err, []byte(":"))
					} else {
						bcnt, err = enc.wr(bcnt, err, []byte(": "))
						bcnt, err = enc.wr(bcnt, err, []byte(s))
					}
					bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
				}
				bcnt, err = enc.encodeIfNotEmpty(item, indent, bcnt, err)
			}
		}
	case reflect.Struct:
		bcnt, err = enc.encodeStruct(indent, v, bcnt, err)
	default:
		err = MakeNestedTextError(ErrCodeSchema,
			fmt.Sprintf("unable to encode type %T", tree))
	}
	return bcnt, err
}

// encodeStruct encodes a struct value as a NestedText dict.
func (enc *Encoder) encodeStruct(indent int, v reflect.Value, bcnt int, err error) (int, error) {
	t := v.Type()

	type fieldEntry struct {
		name  string
		value reflect.Value
	}
	fields := make([]fieldEntry, 0, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldValue := v.Field(i)
		name := field.Name

		tag := field.Tag.Get("nt")
		if tag == "-" {
			continue // skip this field
		}

		omitempty := false
		if tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" {
				name = parts[0]
			}
			for _, opt := range parts[1:] {
				if opt == "omitempty" {
					omitempty = true
				}
			}
		}

		if omitempty && isEmptyValue(fieldValue) {
			continue
		}

		fields = append(fields, fieldEntry{name: name, value: fieldValue})
	}

	// Sort fields by name for consistent output
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].name < fields[j].name
	})

	// Empty struct
	if len(fields) == 0 {
		return enc.wr(bcnt, err, []byte("{}\n"))
	}

	for _, f := range fields {
		item := f.value.Interface()
		if ok, keyAsBytes := isInlineable(encAsKey, f.name); ok {
			bcnt, err = enc.indent(bcnt, err, indent)
			bcnt, err = enc.wr(bcnt, err, keyAsBytes)
			bcnt, err = enc.wr(bcnt, err, []byte{':'})
			if ok, itemAsBytes := isInlineable(encAsString, item); ok {
				bcnt, err = enc.wr(bcnt, err, []byte{' '})
				bcnt, err = enc.wr(bcnt, err, itemAsBytes)
				bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
			} else {
				bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
				bcnt, err = enc.encodeIfNotEmpty(item, indent, bcnt, err)
			}
		} else {
			// Multi-line key (rare for struct field names)
			S := strings.Split(f.name, "\n")
			for _, s := range S {
				bcnt, err = enc.indent(bcnt, err, indent)
				if s == "" {
					bcnt, err = enc.wr(bcnt, err, []byte(":"))
				} else {
					bcnt, err = enc.wr(bcnt, err, []byte(": "))
					bcnt, err = enc.wr(bcnt, err, []byte(s))
				}
				bcnt, err = enc.wr(bcnt, err, []byte{'\n'})
			}
			bcnt, err = enc.encodeIfNotEmpty(item, indent, bcnt, err)
		}
	}
	return bcnt, err
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

func (enc *Encoder) encodeIfNotEmpty(item interface{}, indent, bcnt int, err error) (int, error) {
	if err != nil {
		return bcnt, err
	}
	if s, ok := item.(string); ok {
		if s == "" {
			return bcnt, err
		}
	}
	return enc.encode(indent+1, item, bcnt, err)
}

func isEncodable(item interface{}) bool {
	switch reflect.ValueOf(item).Kind() {
	case reflect.Chan, reflect.Func, reflect.Invalid, reflect.Uintptr, reflect.UnsafePointer:
		return false
	}
	return true
}

// item categories for encoding
const (
	encAsKey int = iota
	encAsString
	encAsList
	encAsDict
)

// encItemPattern holds a string (list of characters) per item category which are
// forbidden for this item.
var encItemPattern = []string{
	":\n",    // Key
	"\n",     // String
	"[],\n",  // List
	"{},:\n", // Dict
}

func isInlineable(what int, item interface{}) (bool, []byte) {
	switch reflect.ValueOf(item).Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.Struct:
		return false, nil
	case reflect.String:
		s := item.(string)
		if s == "" {
			return false, nil
		}
		if strings.ContainsAny(s, encItemPattern[what]) {
			return false, nil
		}
		return true, []byte(s)
	case reflect.Bool:
		if item.(bool) {
			return true, []byte("true")
		}
		return true, []byte("false")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		v := fmt.Sprintf("%v", item)
		return true, []byte(v)
	default:
		v := fmt.Sprintf("%v", item)
		if strings.ContainsAny(v, encItemPattern[what]) {
			return false, nil
		}
		return true, []byte(v)
	}
}

// used for indentation
var encSpaces = [MaxIndent]byte{
	' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ',
}

// indent writes the correct amount of spaces for the current indentation level.
func (enc *Encoder) indent(bcnt int, err error, indent int) (int, error) {
	c := 0
	for i := 0; i < indent; i++ {
		c, err = enc.wr(0, err, encSpaces[:enc.indentSize])
		bcnt += c
	}
	return bcnt, err
}

// wr is a wrapper around w.Write(…). We wrap it to suppress the call if err is non-nil
// and to add up the count of written bytes.
func (enc *Encoder) wr(bcnt int, err error, data []byte) (int, error) {
	if err != nil {
		return bcnt, err
	}
	c, err := enc.w.Write(data)
	if err != nil {
		err = WrapError(ErrCodeIO, "write error during encoding", err)
	}
	return bcnt + c, err
}

// --- Legacy API for compatibility ---

// EncoderOption is a type to influence the behaviour of the encoding process.
type EncoderOption func(*Encoder)

// IndentBy sets the number of spaces per indentation level. The default is 2.
// Allowed values are 1…MaxIndent
func IndentBy(indentSize int) EncoderOption {
	return func(enc *Encoder) {
		enc.SetIndent(indentSize)
	}
}

// InlineLimited sets the threshold above which lists and dicts are never inlined.
// If set to a small number, inlining is suppressed.
// Defaults to DefaultInlineLimit; may not exceed 2048.
func InlineLimited(limit int) EncoderOption {
	return func(enc *Encoder) {
		enc.SetInlineLimit(limit)
	}
}

// Encode encodes its argument `tree` as a byte stream in NestedText format.
// It returns the number of bytes written and possibly an error.
//
// This is the legacy API; prefer using Marshal or NewEncoder for new code.
func Encode(tree interface{}, w io.Writer, opts ...EncoderOption) (int, error) {
	enc := NewEncoder(w)
	for _, opt := range opts {
		opt(enc)
	}
	return enc.encode(0, tree, 0, nil)
}
