# nestedtext

A Go library for [NestedText](https://nestedtext.org/), a human-friendly data format.

This is a fork of [github.com/npillmayer/nestext](https://github.com/npillmayer/nestext) with an idiomatic Go API and compatibility with NestedText 3.8.

## Installation

```
go get github.com/danielledeleo/nestedtext
```

## Usage

### Unmarshaling into structs

```go
input := []byte(`
name: myapp
    port: 8080
debug: true
hosts:
    - localhost
    - 192.168.1.1
`)

type Config struct {
    Name  string   `nt:"name"`
    Port  int      `nt:"port"`
    Debug bool     `nt:"debug"`
    Hosts []string `nt:"hosts"`
}

var config Config
err := nestedtext.Unmarshal(input, &config)

fmt.Println(config.Name)  // myapp
fmt.Println(config.Port)  // 8080 (int, not string)
fmt.Println(config.Debug) // true (bool)
```

### Marshaling structs

```go
config := Config{
    Name:  "myapp",
    Port:  8080,
    Debug: true,
    Hosts: []string{"localhost", "192.168.1.1"},
}

data, err := nestedtext.Marshal(config)
```

### Struct tags

| Tag | Effect |
|-----|--------|
| `nt:"name"` | Use "name" as the key |
| `nt:"-"` | Ignore field |
| `nt:",omitempty"` | Omit if empty (marshal only) |

### Type coercion

NestedText values are always strings. When unmarshaling, string values are automatically converted to the target type:

- `int`, `int8`–`int64`, `uint`, `uint8`–`uint64`
- `float32`, `float64`
- `bool` (`"true"`, `"false"`, `"1"`, `"0"`)

### Low-level API

For dynamic data, use `Parse` which returns `interface{}`:

```go
result, err := nestedtext.Parse(strings.NewReader(input))
// result is string, []interface{}, or map[string]interface{}
```

For encoding without structs:

```go
data := map[string]interface{}{
    "name": "myapp",
    "port": "8080",
}
err := nestedtext.NewEncoder(os.Stdout).Encode(data)
```

## NestedText format

```nestedtext
# Comments start with #

# Dictionaries
server:
    host: localhost
    port: 8080

# Lists
users:
    - alice
    - bob

# Multiline strings
description:
    > This is a long
    > multiline string

# Inline syntax
point: {x: 1, y: 2}
tags: [dev, test, prod]
```

See [nestedtext.org](https://nestedtext.org/) for the full specification.

## Status

Passes the official NestedText 3.8 test suite (148 test cases).
