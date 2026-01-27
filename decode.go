package nestedtext

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// Unmarshal parses NestedText data and stores the result in the value pointed to by v.
// If v is nil or not a pointer, Unmarshal returns an error.
//
// Unmarshal uses the following rules to decode values:
//
//   - Structs are decoded from NestedText dicts. Keys are matched to struct field names
//     (case-insensitive) or the `nt` tag if present.
//   - Slices are decoded from NestedText lists.
//   - Maps are decoded from NestedText dicts.
//   - Strings are decoded directly.
//   - Numeric types (int, float64, etc.) are decoded from NestedText strings using strconv.
//   - Booleans are decoded from strings: "true"/"false" or "1"/"0".
//
// Type coercion automatically converts NestedText strings to the target Go type.
func Unmarshal(data []byte, v interface{}, opts ...DecodeOption) error {
	d := NewDecoder(bytes.NewReader(data), opts...)
	return d.Decode(v)
}

// Decoder reads and decodes NestedText values from an input stream.
type Decoder struct {
	r    io.Reader
	opts []DecodeOption
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader, opts ...DecodeOption) *Decoder {
	return &Decoder{r: r, opts: opts}
}

// Decode reads the next NestedText value from its input and stores it in the value pointed to by v.
func (d *Decoder) Decode(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return MakeNestedTextError(ErrCodeUnmarshal, "Decode requires non-nil pointer argument")
	}

	parsed, err := Parse(d.r, d.opts...)
	if err != nil {
		return err
	}

	return decode(parsed, rv.Elem())
}

// structInfo holds cached metadata about a struct type.
type structInfo struct {
	fields []fieldInfo
}

// fieldInfo holds metadata about a single struct field.
type fieldInfo struct {
	name      string       // Go field name
	index     int          // field index
	tag       string       // nt tag name (empty if not specified)
	omitEmpty bool         // omitempty option
	ignore    bool         // "-" tag
	fieldType reflect.Type // field type
}

// structInfoCache caches struct metadata to avoid repeated reflection.
var structInfoCache sync.Map // map[reflect.Type]*structInfo

// getStructInfo returns cached struct metadata for the given type.
func getStructInfo(t reflect.Type) *structInfo {
	if cached, ok := structInfoCache.Load(t); ok {
		return cached.(*structInfo)
	}

	info := &structInfo{
		fields: make([]fieldInfo, 0, t.NumField()),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fi := fieldInfo{
			name:      field.Name,
			index:     i,
			fieldType: field.Type,
		}

		tag := field.Tag.Get("nt")
		if tag == "-" {
			fi.ignore = true
		} else if tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" {
				fi.tag = parts[0]
			}
			for _, opt := range parts[1:] {
				if opt == "omitempty" {
					fi.omitEmpty = true
				}
			}
		}

		info.fields = append(info.fields, fi)
	}

	structInfoCache.Store(t, info)
	return info
}

// decode recursively populates v from parsed NestedText data.
func decode(data interface{}, v reflect.Value) error {
	// Handle nil data
	if data == nil {
		return nil
	}

	// Allocate pointer if needed
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	// Check for Unmarshaler interface on addressable values
	if v.CanAddr() {
		if u, ok := v.Addr().Interface().(Unmarshaler); ok {
			return u.UnmarshalNT(data)
		}
	}

	switch v.Kind() {
	case reflect.Interface:
		// For interface{}, just set the value directly
		v.Set(reflect.ValueOf(data))
		return nil

	case reflect.String:
		return decodeString(data, v)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return decodeInt(data, v)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return decodeUint(data, v)

	case reflect.Float32, reflect.Float64:
		return decodeFloat(data, v)

	case reflect.Bool:
		return decodeBool(data, v)

	case reflect.Slice:
		return decodeSlice(data, v)

	case reflect.Map:
		return decodeMap(data, v)

	case reflect.Struct:
		return decodeStruct(data, v)

	default:
		return &UnmarshalTypeError{
			Value: typeNameOf(data),
			Type:  v.Type(),
		}
	}
}

// decodeString decodes a NestedText string into a Go string.
func decodeString(data interface{}, v reflect.Value) error {
	s, ok := data.(string)
	if !ok {
		return &UnmarshalTypeError{
			Value: typeNameOf(data),
			Type:  v.Type(),
		}
	}
	v.SetString(s)
	return nil
}

// decodeInt decodes a NestedText string into a Go int type.
func decodeInt(data interface{}, v reflect.Value) error {
	s, ok := data.(string)
	if !ok {
		return &UnmarshalTypeError{
			Value: typeNameOf(data),
			Type:  v.Type(),
		}
	}

	n, err := strconv.ParseInt(s, 10, v.Type().Bits())
	if err != nil {
		return &UnmarshalTypeError{
			Value: fmt.Sprintf("string %q", s),
			Type:  v.Type(),
		}
	}
	v.SetInt(n)
	return nil
}

// decodeUint decodes a NestedText string into a Go uint type.
func decodeUint(data interface{}, v reflect.Value) error {
	s, ok := data.(string)
	if !ok {
		return &UnmarshalTypeError{
			Value: typeNameOf(data),
			Type:  v.Type(),
		}
	}

	n, err := strconv.ParseUint(s, 10, v.Type().Bits())
	if err != nil {
		return &UnmarshalTypeError{
			Value: fmt.Sprintf("string %q", s),
			Type:  v.Type(),
		}
	}
	v.SetUint(n)
	return nil
}

// decodeFloat decodes a NestedText string into a Go float type.
func decodeFloat(data interface{}, v reflect.Value) error {
	s, ok := data.(string)
	if !ok {
		return &UnmarshalTypeError{
			Value: typeNameOf(data),
			Type:  v.Type(),
		}
	}

	n, err := strconv.ParseFloat(s, v.Type().Bits())
	if err != nil {
		return &UnmarshalTypeError{
			Value: fmt.Sprintf("string %q", s),
			Type:  v.Type(),
		}
	}
	v.SetFloat(n)
	return nil
}

// decodeBool decodes a NestedText string into a Go bool.
// Accepts: "true"/"false", "1"/"0" (case-sensitive).
func decodeBool(data interface{}, v reflect.Value) error {
	s, ok := data.(string)
	if !ok {
		return &UnmarshalTypeError{
			Value: typeNameOf(data),
			Type:  v.Type(),
		}
	}

	switch s {
	case "true", "1":
		v.SetBool(true)
	case "false", "0":
		v.SetBool(false)
	default:
		return &UnmarshalTypeError{
			Value: fmt.Sprintf("string %q", s),
			Type:  v.Type(),
		}
	}
	return nil
}

// decodeSlice decodes a NestedText list into a Go slice.
func decodeSlice(data interface{}, v reflect.Value) error {
	list, ok := data.([]interface{})
	if !ok {
		return &UnmarshalTypeError{
			Value: typeNameOf(data),
			Type:  v.Type(),
		}
	}

	slice := reflect.MakeSlice(v.Type(), len(list), len(list))
	for i, item := range list {
		if err := decode(item, slice.Index(i)); err != nil {
			if ute, ok := err.(*UnmarshalTypeError); ok {
				ute.Path = fmt.Sprintf("[%d]%s", i, ute.Path)
			}
			return err
		}
	}
	v.Set(slice)
	return nil
}

// decodeMap decodes a NestedText dict into a Go map.
func decodeMap(data interface{}, v reflect.Value) error {
	dict, ok := data.(map[string]interface{})
	if !ok {
		return &UnmarshalTypeError{
			Value: typeNameOf(data),
			Type:  v.Type(),
		}
	}

	// Only support string keys
	if v.Type().Key().Kind() != reflect.String {
		return &UnmarshalTypeError{
			Value: "dict",
			Type:  v.Type(),
		}
	}

	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}

	elemType := v.Type().Elem()
	for key, val := range dict {
		elemValue := reflect.New(elemType).Elem()
		if err := decode(val, elemValue); err != nil {
			if ute, ok := err.(*UnmarshalTypeError); ok {
				ute.Path = "." + key + ute.Path
			}
			return err
		}
		v.SetMapIndex(reflect.ValueOf(key), elemValue)
	}
	return nil
}

// decodeStruct decodes a NestedText dict into a Go struct.
func decodeStruct(data interface{}, v reflect.Value) error {
	dict, ok := data.(map[string]interface{})
	if !ok {
		return &UnmarshalTypeError{
			Value: typeNameOf(data),
			Type:  v.Type(),
		}
	}

	info := getStructInfo(v.Type())

	for key, val := range dict {
		fi := findField(info, key)
		if fi == nil {
			// Unknown field, skip it
			continue
		}

		field := v.Field(fi.index)
		if err := decode(val, field); err != nil {
			if ute, ok := err.(*UnmarshalTypeError); ok {
				ute.Path = "." + v.Type().Name() + "." + fi.name + ute.Path
			}
			return err
		}
	}
	return nil
}

// findField finds a struct field matching the given key.
// Matches by tag name first, then by field name (case-insensitive).
func findField(info *structInfo, key string) *fieldInfo {
	keyLower := strings.ToLower(key)

	// First pass: match by tag
	for i := range info.fields {
		fi := &info.fields[i]
		if fi.ignore {
			continue
		}
		if fi.tag == key {
			return fi
		}
	}

	// Second pass: match by field name (case-insensitive)
	for i := range info.fields {
		fi := &info.fields[i]
		if fi.ignore {
			continue
		}
		if fi.tag == "" && strings.ToLower(fi.name) == keyLower {
			return fi
		}
	}

	return nil
}

// typeNameOf returns a descriptive name for the NestedText type.
func typeNameOf(data interface{}) string {
	switch data.(type) {
	case string:
		return "string"
	case []interface{}:
		return "list"
	case map[string]interface{}:
		return "dict"
	default:
		return fmt.Sprintf("%T", data)
	}
}

// UnmarshalTypeError describes a type mismatch during unmarshaling.
type UnmarshalTypeError struct {
	Value string       // Description of the NestedText value
	Type  reflect.Type // Target Go type
	Path  string       // Path to the error (e.g., ".Config.Database.Port")
}

func (e *UnmarshalTypeError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("nestedtext: cannot unmarshal %s into Go value of type %s at %s", e.Value, e.Type, e.Path)
	}
	return fmt.Sprintf("nestedtext: cannot unmarshal %s into Go value of type %s", e.Value, e.Type)
}
