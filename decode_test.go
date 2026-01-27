package nestedtext

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestUnmarshalBasicStruct(t *testing.T) {
	input := `
name: myapp
port: 8080
debug: true
`
	type Config struct {
		Name  string `nt:"name"`
		Port  int    `nt:"port"`
		Debug bool   `nt:"debug"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if config.Name != "myapp" {
		t.Errorf("Name = %q, want %q", config.Name, "myapp")
	}
	if config.Port != 8080 {
		t.Errorf("Port = %d, want %d", config.Port, 8080)
	}
	if config.Debug != true {
		t.Errorf("Debug = %v, want %v", config.Debug, true)
	}
}

func TestUnmarshalNestedStruct(t *testing.T) {
	input := `
name: myapp
port: 8080
database:
    host: localhost
    port: 5432
`
	type Database struct {
		Host string `nt:"host"`
		Port int    `nt:"port"`
	}

	type Config struct {
		Name     string   `nt:"name"`
		Port     int      `nt:"port"`
		Database Database `nt:"database"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if config.Name != "myapp" {
		t.Errorf("Name = %q, want %q", config.Name, "myapp")
	}
	if config.Port != 8080 {
		t.Errorf("Port = %d, want %d", config.Port, 8080)
	}
	if config.Database.Host != "localhost" {
		t.Errorf("Database.Host = %q, want %q", config.Database.Host, "localhost")
	}
	if config.Database.Port != 5432 {
		t.Errorf("Database.Port = %d, want %d", config.Database.Port, 5432)
	}
}

func TestUnmarshalSlice(t *testing.T) {
	input := `
- one
- two
- three
`
	var result []string
	err := Unmarshal([]byte(input), &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	want := []string{"one", "two", "three"}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("got %v, want %v", result, want)
	}
}

func TestUnmarshalSliceOfInts(t *testing.T) {
	input := `
- 1
- 2
- 3
`
	var result []int
	err := Unmarshal([]byte(input), &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	want := []int{1, 2, 3}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("got %v, want %v", result, want)
	}
}

func TestUnmarshalStructWithSlice(t *testing.T) {
	input := `
name: myapp
hosts:
    - localhost
    - 192.168.1.1
`
	type Config struct {
		Name  string   `nt:"name"`
		Hosts []string `nt:"hosts"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if config.Name != "myapp" {
		t.Errorf("Name = %q, want %q", config.Name, "myapp")
	}
	want := []string{"localhost", "192.168.1.1"}
	if !reflect.DeepEqual(config.Hosts, want) {
		t.Errorf("Hosts = %v, want %v", config.Hosts, want)
	}
}

func TestUnmarshalMap(t *testing.T) {
	input := `
a: Hello
b: World
`
	var result map[string]string
	err := Unmarshal([]byte(input), &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	want := map[string]string{"a": "Hello", "b": "World"}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("got %v, want %v", result, want)
	}
}

func TestUnmarshalMapIntValues(t *testing.T) {
	input := `
a: 1
b: 2
c: 3
`
	var result map[string]int
	err := Unmarshal([]byte(input), &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	want := map[string]int{"a": 1, "b": 2, "c": 3}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("got %v, want %v", result, want)
	}
}

func TestUnmarshalIgnoreField(t *testing.T) {
	input := `
name: myapp
password: secret123
`
	type Config struct {
		Name     string `nt:"name"`
		Password string `nt:"-"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if config.Name != "myapp" {
		t.Errorf("Name = %q, want %q", config.Name, "myapp")
	}
	if config.Password != "" {
		t.Errorf("Password = %q, want %q (should be ignored)", config.Password, "")
	}
}

func TestUnmarshalBoolVariants(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			input := "value: " + tt.input
			type Config struct {
				Value bool `nt:"value"`
			}
			var config Config
			err := Unmarshal([]byte(input), &config)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if config.Value != tt.want {
				t.Errorf("got %v, want %v", config.Value, tt.want)
			}
		})
	}
}

func TestUnmarshalNumericTypes(t *testing.T) {
	input := `
int8val: -128
int16val: -32768
int32val: -2147483648
int64val: -9223372036854775808
uint8val: 255
uint16val: 65535
uint32val: 4294967295
uint64val: 18446744073709551615
float32val: 3.14
float64val: 3.141592653589793
`
	type Numbers struct {
		Int8val    int8    `nt:"int8val"`
		Int16val   int16   `nt:"int16val"`
		Int32val   int32   `nt:"int32val"`
		Int64val   int64   `nt:"int64val"`
		Uint8val   uint8   `nt:"uint8val"`
		Uint16val  uint16  `nt:"uint16val"`
		Uint32val  uint32  `nt:"uint32val"`
		Uint64val  uint64  `nt:"uint64val"`
		Float32val float32 `nt:"float32val"`
		Float64val float64 `nt:"float64val"`
	}

	var numbers Numbers
	err := Unmarshal([]byte(input), &numbers)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if numbers.Int8val != -128 {
		t.Errorf("Int8val = %d, want %d", numbers.Int8val, -128)
	}
	if numbers.Uint8val != 255 {
		t.Errorf("Uint8val = %d, want %d", numbers.Uint8val, 255)
	}
	if numbers.Uint64val != 18446744073709551615 {
		t.Errorf("Uint64val = %d, want %d", numbers.Uint64val, uint64(18446744073709551615))
	}
	if numbers.Float64val != 3.141592653589793 {
		t.Errorf("Float64val = %f, want %f", numbers.Float64val, 3.141592653589793)
	}
}

func TestUnmarshalCaseInsensitiveFieldMatch(t *testing.T) {
	input := `
NAME: myapp
Port: 8080
DEBUG: true
`
	type Config struct {
		Name  string
		Port  int
		Debug bool
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if config.Name != "myapp" {
		t.Errorf("Name = %q, want %q", config.Name, "myapp")
	}
	if config.Port != 8080 {
		t.Errorf("Port = %d, want %d", config.Port, 8080)
	}
	if config.Debug != true {
		t.Errorf("Debug = %v, want %v", config.Debug, true)
	}
}

func TestUnmarshalTagOverridesFieldName(t *testing.T) {
	input := `
app_name: myapp
server_port: 8080
`
	type Config struct {
		Name string `nt:"app_name"`
		Port int    `nt:"server_port"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if config.Name != "myapp" {
		t.Errorf("Name = %q, want %q", config.Name, "myapp")
	}
	if config.Port != 8080 {
		t.Errorf("Port = %d, want %d", config.Port, 8080)
	}
}

func TestUnmarshalPointerFields(t *testing.T) {
	input := `
name: myapp
port: 8080
`
	type Config struct {
		Name *string `nt:"name"`
		Port *int    `nt:"port"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if config.Name == nil || *config.Name != "myapp" {
		t.Errorf("Name = %v, want %q", config.Name, "myapp")
	}
	if config.Port == nil || *config.Port != 8080 {
		t.Errorf("Port = %v, want %d", config.Port, 8080)
	}
}

func TestUnmarshalIntoInterface(t *testing.T) {
	input := `
a: Hello
b: World
`
	var result interface{}
	err := Unmarshal([]byte(input), &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	dict, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result is not a map, got %T", result)
	}

	if dict["a"] != "Hello" {
		t.Errorf("dict[a] = %q, want %q", dict["a"], "Hello")
	}
	if dict["b"] != "World" {
		t.Errorf("dict[b] = %q, want %q", dict["b"], "World")
	}
}

func TestUnmarshalErrors(t *testing.T) {
	t.Run("nil pointer", func(t *testing.T) {
		err := Unmarshal([]byte("a: b"), nil)
		if err == nil {
			t.Error("expected error for nil pointer")
		}
	})

	t.Run("non-pointer", func(t *testing.T) {
		var s string
		err := Unmarshal([]byte("hello"), s)
		if err == nil {
			t.Error("expected error for non-pointer")
		}
	})

	t.Run("type mismatch: list to string", func(t *testing.T) {
		input := `
- one
- two
`
		var s string
		err := Unmarshal([]byte(input), &s)
		if err == nil {
			t.Error("expected error for type mismatch")
		}
		var ute *UnmarshalTypeError
		if !errors.As(err, &ute) {
			t.Errorf("expected UnmarshalTypeError, got %T", err)
		}
	})

	t.Run("invalid int", func(t *testing.T) {
		input := "value: not-a-number"
		type Config struct {
			Value int `nt:"value"`
		}
		var config Config
		err := Unmarshal([]byte(input), &config)
		if err == nil {
			t.Error("expected error for invalid int")
		}
	})

	t.Run("invalid bool", func(t *testing.T) {
		input := "value: maybe"
		type Config struct {
			Value bool `nt:"value"`
		}
		var config Config
		err := Unmarshal([]byte(input), &config)
		if err == nil {
			t.Error("expected error for invalid bool")
		}
	})
}

func TestUnmarshalUnknownFieldsIgnored(t *testing.T) {
	input := `
name: myapp
unknown_field: some value
another_unknown: 123
`
	type Config struct {
		Name string `nt:"name"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if config.Name != "myapp" {
		t.Errorf("Name = %q, want %q", config.Name, "myapp")
	}
}

// CustomType demonstrates the Unmarshaler interface.
type CustomType struct {
	Value string
}

func (c *CustomType) UnmarshalNT(data interface{}) error {
	s, ok := data.(string)
	if !ok {
		return errors.New("expected string")
	}
	c.Value = strings.ToUpper(s)
	return nil
}

func TestUnmarshalCustomUnmarshaler(t *testing.T) {
	input := "value: hello"
	type Config struct {
		Value CustomType `nt:"value"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if config.Value.Value != "HELLO" {
		t.Errorf("Value = %q, want %q", config.Value.Value, "HELLO")
	}
}

func TestDecoder(t *testing.T) {
	input := `
name: myapp
port: 8080
`
	type Config struct {
		Name string `nt:"name"`
		Port int    `nt:"port"`
	}

	decoder := NewDecoder(strings.NewReader(input))
	var config Config
	err := decoder.Decode(&config)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if config.Name != "myapp" {
		t.Errorf("Name = %q, want %q", config.Name, "myapp")
	}
	if config.Port != 8080 {
		t.Errorf("Port = %d, want %d", config.Port, 8080)
	}
}

func TestUnmarshalCompleteExample(t *testing.T) {
	input := `
name: myapp
port: 8080
debug: true
hosts:
    - localhost
    - 192.168.1.1
database:
    host: localhost
    port: 5432
`

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

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if config.Port != 8080 {
		t.Errorf("Port = %d, want %d", config.Port, 8080)
	}
	if config.Debug != true {
		t.Errorf("Debug = %v, want %v", config.Debug, true)
	}
	if config.Database.Port != 5432 {
		t.Errorf("Database.Port = %d, want %d", config.Database.Port, 5432)
	}
}

func TestUnmarshalEmptyInput(t *testing.T) {
	input := ""
	type Config struct {
		Name string `nt:"name"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Empty input should result in zero values
	if config.Name != "" {
		t.Errorf("Name = %q, want empty string", config.Name)
	}
}

func TestUnmarshalNestedSliceOfStructs(t *testing.T) {
	input := `
servers:
    -
        host: server1
        port: 8080
    -
        host: server2
        port: 9090
`
	type Server struct {
		Host string `nt:"host"`
		Port int    `nt:"port"`
	}

	type Config struct {
		Servers []Server `nt:"servers"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(config.Servers) != 2 {
		t.Fatalf("len(Servers) = %d, want 2", len(config.Servers))
	}

	if config.Servers[0].Host != "server1" {
		t.Errorf("Servers[0].Host = %q, want %q", config.Servers[0].Host, "server1")
	}
	if config.Servers[0].Port != 8080 {
		t.Errorf("Servers[0].Port = %d, want %d", config.Servers[0].Port, 8080)
	}
	if config.Servers[1].Host != "server2" {
		t.Errorf("Servers[1].Host = %q, want %q", config.Servers[1].Host, "server2")
	}
	if config.Servers[1].Port != 9090 {
		t.Errorf("Servers[1].Port = %d, want %d", config.Servers[1].Port, 9090)
	}
}

// --- Minimal mode tests ---

func TestMinimalModeRejectsInlineList(t *testing.T) {
	input := `[a, b, c]`
	_, err := Parse(strings.NewReader(input), Minimal())
	if err == nil {
		t.Error("expected error for inline list in minimal mode, got nil")
	}
}

func TestMinimalModeRejectsInlineDict(t *testing.T) {
	input := `{key: value}`
	_, err := Parse(strings.NewReader(input), Minimal())
	if err == nil {
		t.Error("expected error for inline dict in minimal mode, got nil")
	}
}

func TestMinimalModeRejectsMultilineKey(t *testing.T) {
	input := `: first line
: second line
    value
`
	_, err := Parse(strings.NewReader(input), Minimal())
	if err == nil {
		t.Error("expected error for multi-line key in minimal mode, got nil")
	}
}

func TestMinimalModeAcceptsBlockSyntax(t *testing.T) {
	input := `
name: test
items:
    - one
    - two
`
	result, err := Parse(strings.NewReader(input), Minimal())
	if err != nil {
		t.Fatalf("unexpected error in minimal mode with block syntax: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["name"] != "test" {
		t.Errorf("name = %q, want %q", m["name"], "test")
	}
}

func TestUnmarshalWithMinimalOption(t *testing.T) {
	input := `
name: myapp
port: 8080
`
	type Config struct {
		Name string `nt:"name"`
		Port int    `nt:"port"`
	}

	var config Config
	err := Unmarshal([]byte(input), &config, Minimal())
	if err != nil {
		t.Fatalf("Unmarshal with Minimal() failed: %v", err)
	}
	if config.Name != "myapp" {
		t.Errorf("Name = %q, want %q", config.Name, "myapp")
	}
}
