package nestedtext

import (
	"io"
	"strings"
	"testing"
)

func TestEncodeOptions(t *testing.T) {
	n, err := Encode("X", io.Discard, IndentBy(5), InlineLimited(80))
	if err != nil {
		t.Error(err)
	}
	if n != 4 { // "> X\n"
		t.Errorf("expected encoding to be of length 4, is %d", n)
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
	enc := NewEncoder(&buf)
	enc.SetIndent(4)

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

// ----------------------------------------------------------------------

func expectEncode(t *testing.T, tree interface{}, target string) {
	t.Helper()
	out := &strings.Builder{}
	Encode(tree, out)
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
