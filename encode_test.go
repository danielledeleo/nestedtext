package nestedtext

import (
	"strings"
	"testing"
)

func TestEncoderOptions(t *testing.T) {
	var buf strings.Builder
	enc := NewEncoder(&buf, WithIndent(5), WithFlowWidth(80))
	err := enc.Encode("X")
	if err != nil {
		t.Error(err)
	}
	if buf.Len() != 4 { // "> X\n"
		t.Errorf("expected encoding to be of length 4, is %d", buf.Len())
	}
}

func TestEncodeSimpleString(t *testing.T) {
	expectEncode(t, "Hello\nWorld", `> Hello
> World
`)
}

func TestEncodeSimpleStringList(t *testing.T) {
	expectEncode(t, []string{"Hello", "World"}, "[Hello, World]\n")
}

func TestEncodeStringListWithComma(t *testing.T) {
	expectEncode(t, []string{"Hello", "Wo,rld"}, `- Hello
- Wo,rld
`)
}

func TestEncodeSimpleNumberList(t *testing.T) {
	expectEncode(t, []interface{}{1, 2, 3}, `- 1
- 2
- 3
`)
}

func TestEncodeConcreteNumberList(t *testing.T) {
	expectEncode(t, []int{1, 2, 3}, `[1, 2, 3]
`)
}

func TestEncodeStringListWithLongString(t *testing.T) {
	expectEncode(t, []string{"Hello", "World", "How\nare\nyou?"}, `- Hello
- World
-
  > How
  > are
  > you?
`)
}

func TestEncodeListOfObjects(t *testing.T) {
	expectEncode(t, []interface{}{4.1, 7.2}, `- 4.1
- 7.2
`)
}

func TestEncodeDict(t *testing.T) {
	expectEncode(t, map[string]string{"World": "Hello!", "How": "are\nyou?"}, `How:
  > are
  > you?
World: Hello!
`)
}

func TestEncodeMultilineKeys(t *testing.T) {
	expectEncode(t, map[string]string{"Hello": "World", "How\nare": "you?"}, `Hello: World
: How
: are
  > you?
`)
}

func TestEncodeNested(t *testing.T) {
	expectEncode(t, map[string]interface{}{
		"Key1": "Value1",
		"Key2": map[string]interface{}{
			"B": 2,
			"A": "a long\nstring",
		}}, `Key1: Value1
Key2:
  A:
    > a long
    > string
  B: 2
`)
}

func TestEncodeStruct(t *testing.T) {
	type Config struct {
		Name  string `nt:"name"`
		Port  int    `nt:"port"`
		Debug bool   `nt:"debug"`
	}

	config := Config{Name: "myapp", Port: 8080, Debug: true}
	expectEncode(t, config, `debug: true
name: myapp
port: 8080
`)
}

func TestEncodeStructOmitempty(t *testing.T) {
	type Config struct {
		Name     string `nt:"name"`
		Port     int    `nt:"port,omitempty"`
		Debug    bool   `nt:"debug,omitempty"`
		Optional string `nt:"optional,omitempty"`
	}

	config := Config{Name: "myapp"}
	expectEncode(t, config, `name: myapp
`)
}

func TestEncodeStructIgnoreField(t *testing.T) {
	type Config struct {
		Name     string `nt:"name"`
		Password string `nt:"-"`
	}

	config := Config{Name: "myapp", Password: "secret"}
	expectEncode(t, config, `name: myapp
`)
}

func TestEncodeNestedStruct(t *testing.T) {
	type Database struct {
		Host string `nt:"host"`
		Port int    `nt:"port"`
	}

	type Config struct {
		Name     string   `nt:"name"`
		Database Database `nt:"database"`
	}

	config := Config{
		Name: "myapp",
		Database: Database{
			Host: "localhost",
			Port: 5432,
		},
	}
	expectEncode(t, config, `database:
  host: localhost
  port: 5432
name: myapp
`)
}

func TestMarshal(t *testing.T) {
	data := map[string]string{"hello": "world"}
	result, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	expected := "hello: world\n"
	if string(result) != expected {
		t.Errorf("got %q, want %q", string(result), expected)
	}
}

func TestMarshalStruct(t *testing.T) {
	type Config struct {
		Name string `nt:"name"`
		Port int    `nt:"port"`
	}

	config := Config{Name: "myapp", Port: 8080}
	result, err := Marshal(config)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	expected := `name: myapp
port: 8080
`
	if string(result) != expected {
		t.Errorf("got %q, want %q", string(result), expected)
	}
}

func TestEncoder(t *testing.T) {
	var buf strings.Builder
	enc := NewEncoder(&buf, WithIndent(4))

	data := map[string]interface{}{
		"key": map[string]string{"nested": "value"},
	}
	err := enc.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	expected := `key:
    nested: value
`
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

func TestEncodeBool(t *testing.T) {
	expectEncode(t, true, "> true\n")
	expectEncode(t, false, "> false\n")
}

func TestEncodeNumbers(t *testing.T) {
	expectEncode(t, 42, "> 42\n")
	expectEncode(t, int64(-123), "> -123\n")
	expectEncode(t, uint(456), "> 456\n")
	expectEncode(t, 3.14, "> 3.14\n")
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	type Database struct {
		Host string `nt:"host"`
		Port int    `nt:"port"`
	}

	type Config struct {
		Name     string   `nt:"name"`
		Port     int      `nt:"port"`
		Debug    bool     `nt:"debug"`
		Hosts    []string `nt:"hosts"`
		Database Database `nt:"database"`
	}

	original := Config{
		Name:  "myapp",
		Port:  8080,
		Debug: true,
		Hosts: []string{"localhost", "192.168.1.1"},
		Database: Database{
			Host: "localhost",
			Port: 5432,
		},
	}

	// Marshal
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	t.Logf("Marshaled:\n%s", string(data))

	// Unmarshal back
	var decoded Config
	err = Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Compare
	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Port != original.Port {
		t.Errorf("Port: got %d, want %d", decoded.Port, original.Port)
	}
	if decoded.Debug != original.Debug {
		t.Errorf("Debug: got %v, want %v", decoded.Debug, original.Debug)
	}
	if decoded.Database.Host != original.Database.Host {
		t.Errorf("Database.Host: got %q, want %q", decoded.Database.Host, original.Database.Host)
	}
	if decoded.Database.Port != original.Database.Port {
		t.Errorf("Database.Port: got %d, want %d", decoded.Database.Port, original.Database.Port)
	}
}

// --- Round-trip safety tests ---
// These tests verify that the encoder's output can be re-parsed to produce
// the original value. Failures indicate the encoder produces invalid NestedText.

func TestRoundTrip_EmptyList(t *testing.T) {
	// Issue: empty []interface{} produces no output instead of []
	encoded, err := Marshal([]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded output: %q", err, encoded)
	}
	list, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T (nil=%v)\nencoded output: %q", result, result == nil, encoded)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %v", list)
	}
}

func TestRoundTrip_KeyStartingWithHash(t *testing.T) {
	// Issue: key "#key" encodes as "#key: value" which re-parses as a comment
	input := map[string]interface{}{"#key": "value"}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded output: %q", err, encoded)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T (nil=%v)\nencoded output: %q", result, result == nil, encoded)
	}
	v, ok := m["#key"]
	if !ok {
		t.Errorf("key '#key' lost in round-trip\nencoded output: %q\nre-parsed: %v", encoded, m)
	} else if v != "value" {
		t.Errorf("value = %v, want 'value'", v)
	}
}

func TestRoundTrip_KeyStartingWithDash(t *testing.T) {
	// Issue: key "- item" encodes verbatim but re-parses as a list item
	input := map[string]interface{}{"- item": "value"}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded output: %q", err, encoded)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T\nencoded output: %q", result, encoded)
	}
	if _, ok := m["- item"]; !ok {
		t.Errorf("key '- item' lost in round-trip\nencoded output: %q\nre-parsed: %v", encoded, result)
	}
}

func TestRoundTrip_KeyStartingWithGreaterThan(t *testing.T) {
	// Issue: key "> text" encodes verbatim but re-parses as a string item
	input := map[string]interface{}{"> text": "value"}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded output: %q", err, encoded)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T\nencoded output: %q", result, encoded)
	}
	if _, ok := m["> text"]; !ok {
		t.Errorf("key '> text' lost in round-trip\nencoded output: %q\nre-parsed: %v", encoded, result)
	}
}

func TestRoundTrip_KeyStartingWithBracket(t *testing.T) {
	// Issue: key "[foo]" encodes verbatim but re-parses as an inline list
	input := map[string]interface{}{"[foo]": "value"}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded output: %q", err, encoded)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T\nencoded output: %q", result, encoded)
	}
	if _, ok := m["[foo]"]; !ok {
		t.Errorf("key '[foo]' lost in round-trip\nencoded output: %q\nre-parsed: %v", encoded, result)
	}
}

func TestRoundTrip_KeyStartingWithBrace(t *testing.T) {
	// Issue: key "{foo}" encodes verbatim but re-parses as an inline dict
	input := map[string]interface{}{"{foo}": "value"}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded output: %q", err, encoded)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T\nencoded output: %q", result, encoded)
	}
	if _, ok := m["{foo}"]; !ok {
		t.Errorf("key '{foo}' lost in round-trip\nencoded output: %q\nre-parsed: %v", encoded, result)
	}
}

func TestRoundTrip_WhitespaceOnlyKey(t *testing.T) {
	// Issue: whitespace-only key "   " encodes as "   : value" but parser
	// trims it to "", which is an invalid empty key
	input := map[string]interface{}{"   ": "value"}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded output: %q", err, encoded)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T\nencoded output: %q", result, encoded)
	}
	if _, ok := m["   "]; !ok {
		t.Errorf("whitespace-only key lost in round-trip\nencoded output: %q\nre-parsed keys: %v", encoded, m)
	}
}

func TestRoundTrip_KeyWithColonNoSpace(t *testing.T) {
	// Keys containing ":" but not ": " should stay inline, not be forced
	// to multi-line key format
	input := map[string]interface{}{"http://example.com": "value"}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	// Should be inline: "http://example.com: value\n"
	// Not multi-line: ": http://example.com\n    > value\n"
	if strings.Contains(string(encoded), ": http") {
		t.Errorf("key with colon (no space) was unnecessarily forced to multi-line\nencoded output: %q", encoded)
	}
	result, err := Parse(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded output: %q", err, encoded)
	}
	m := result.(map[string]interface{})
	if _, ok := m["http://example.com"]; !ok {
		t.Errorf("key lost in round-trip\nencoded output: %q\nre-parsed: %v", encoded, m)
	}
}

func TestEncode_NilInList(t *testing.T) {
	// nil values in lists should not encode as the string "<nil>"
	input := []interface{}{"a", nil, "b"}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "<nil>") {
		t.Errorf("nil encoded as string '<nil>'\nencoded output: %q", encoded)
	}
	result, err := Parse(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded output: %q", err, encoded)
	}
	list := result.([]interface{})
	if len(list) != 3 {
		t.Fatalf("expected 3 items, got %d\nencoded output: %q", len(list), encoded)
	}
	// nil should round-trip as empty string (NestedText has no null type)
	if list[0] != "a" || list[1] != "" || list[2] != "b" {
		t.Errorf("list = %v, want [a  b]", list)
	}
}

// ----------------------------------------------------------------------

func expectEncode(t *testing.T, tree interface{}, target string) {
	t.Helper()
	out := &strings.Builder{}
	NewEncoder(out).Encode(tree)
	str := out.String()
	t.Logf("encoded:\n%s", str)
	S := strings.Split(str, "\n")
	T := strings.Split(target, "\n")
	if len(S) != len(T) {
		t.Errorf("expected output to have %d lines, has %d", len(T), len(S))
	}
	for i, s := range S {
		if i >= len(T) {
			break
		}
		if T[i] != s {
			t.Errorf("%q != %q", s, T[i])
		}
	}
}

// --- Option tests ---

func TestEncoderWithOptions(t *testing.T) {
	var buf strings.Builder
	enc := NewEncoder(&buf, WithIndent(4), WithFlowWidth(0))
	err := enc.Encode(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	expected := "key: value\n"
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

func TestMarshalWithOptions(t *testing.T) {
	data := map[string]interface{}{
		"nested": map[string]string{"a": "b"},
	}
	result, err := Marshal(data, WithIndent(4))
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	expected := `nested:
    a: b
`
	if string(result) != expected {
		t.Errorf("got %q, want %q", string(result), expected)
	}
}

// --- Minimal mode tests ---

func TestWithMinimalNoInlineLists(t *testing.T) {
	// Without minimal mode, short string lists are inlined
	data := []string{"a", "b"}

	// With minimal mode, they should be block style
	result, err := Marshal(data, WithMinimal())
	if err != nil {
		t.Fatalf("Marshal with WithMinimal failed: %v", err)
	}
	expected := `- a
- b
`
	if string(result) != expected {
		t.Errorf("got %q, want %q", string(result), expected)
	}
}

func TestWithMinimalErrorOnMultilineKey(t *testing.T) {
	// Key containing newline should error in minimal mode
	data := map[string]string{"key\nwith\nnewlines": "value"}
	_, err := Marshal(data, WithMinimal())
	if err == nil {
		t.Error("expected error for multi-line key in minimal mode, got nil")
	}
}

func TestWithMinimalAcceptsNormalKeys(t *testing.T) {
	data := map[string]string{"normal-key": "value"}
	result, err := Marshal(data, WithMinimal())
	if err != nil {
		t.Fatalf("Marshal with WithMinimal failed: %v", err)
	}
	expected := "normal-key: value\n"
	if string(result) != expected {
		t.Errorf("got %q, want %q", string(result), expected)
	}
}
