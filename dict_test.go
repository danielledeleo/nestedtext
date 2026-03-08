package nestedtext

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseOrdered_PreservesKeyOrder(t *testing.T) {
	input := `
alpha: 1
beta: 2
gamma: 3
delta: 4
epsilon: 5
`
	result, err := ParseOrdered(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := result.(*Dict)
	if !ok {
		t.Fatalf("expected *Dict, got %T", result)
	}
	want := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	got := d.Keys()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestParseOrdered_NestedDicts(t *testing.T) {
	input := `
outer1:
    inner_a: 1
    inner_b: 2
outer2:
    inner_c: 3
    inner_d: 4
`
	result, err := ParseOrdered(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	d := result.(*Dict)
	if d.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", d.Len())
	}

	if got := d.Keys(); !reflect.DeepEqual(got, []string{"outer1", "outer2"}) {
		t.Errorf("outer keys = %v", got)
	}

	v1, ok := d.Get("outer1")
	if !ok {
		t.Fatal("missing key outer1")
	}
	inner1, ok := v1.(*Dict)
	if !ok {
		t.Fatalf("nested value: expected *Dict, got %T", v1)
	}
	if got := inner1.Keys(); !reflect.DeepEqual(got, []string{"inner_a", "inner_b"}) {
		t.Errorf("inner keys = %v", got)
	}
}

func TestParseOrdered_MixedNesting(t *testing.T) {
	// Dicts inside lists
	input := `
-
    key1: val1
    key2: val2
-
    key3: val3
`
	result, err := ParseOrdered(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	list, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
	d0, ok := list[0].(*Dict)
	if !ok {
		t.Fatalf("list[0]: expected *Dict, got %T", list[0])
	}
	if got := d0.Keys(); !reflect.DeepEqual(got, []string{"key1", "key2"}) {
		t.Errorf("list[0] keys = %v", got)
	}

	// Lists inside dicts
	input2 := `
fruits:
    - apple
    - banana
vegs:
    - carrot
`
	result2, err := ParseOrdered(strings.NewReader(input2))
	if err != nil {
		t.Fatal(err)
	}
	d2 := result2.(*Dict)
	if got := d2.Keys(); !reflect.DeepEqual(got, []string{"fruits", "vegs"}) {
		t.Errorf("keys = %v", got)
	}
	fruits, _ := d2.Get("fruits")
	fruitsList, ok := fruits.([]interface{})
	if !ok {
		t.Fatalf("fruits: expected []interface{}, got %T", fruits)
	}
	if len(fruitsList) != 2 || fruitsList[0] != "apple" || fruitsList[1] != "banana" {
		t.Errorf("fruits = %v", fruitsList)
	}
}

func TestParseOrdered_InlineDict(t *testing.T) {
	input := `{z: 1, a: 2, m: 3}`
	result, err := ParseOrdered(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := result.(*Dict)
	if !ok {
		t.Fatalf("expected *Dict, got %T", result)
	}
	want := []string{"z", "a", "m"}
	if got := d.Keys(); !reflect.DeepEqual(got, want) {
		t.Errorf("inline dict keys = %v, want %v", got, want)
	}
}

func TestParseOrdered_NestedInlineDict(t *testing.T) {
	input := `{z: 1, a: {nested: val}}`
	result, err := ParseOrdered(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	d := result.(*Dict)
	if got := d.Keys(); !reflect.DeepEqual(got, []string{"z", "a"}) {
		t.Errorf("keys = %v", got)
	}
	nested, _ := d.Get("a")
	nestedDict, ok := nested.(*Dict)
	if !ok {
		t.Fatalf("nested: expected *Dict, got %T", nested)
	}
	v, _ := nestedDict.Get("nested")
	if v != "val" {
		t.Errorf("nested value = %v, want val", v)
	}
}

func TestParseOrdered_EmptyDict(t *testing.T) {
	input := `{}`
	result, err := ParseOrdered(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := result.(*Dict)
	if !ok {
		t.Fatalf("expected *Dict, got %T", result)
	}
	if d.Len() != 0 {
		t.Errorf("empty dict Len() = %d", d.Len())
	}
}

func TestParseOrdered_CombinedWithMinimal(t *testing.T) {
	input := `
x: 1
y: 2
`
	result, err := ParseOrdered(strings.NewReader(input), Minimal())
	if err != nil {
		t.Fatal(err)
	}
	d, ok := result.(*Dict)
	if !ok {
		t.Fatalf("expected *Dict, got %T", result)
	}
	if got := d.Keys(); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("keys = %v", got)
	}
}

func TestParseOrdered_TopLevelString(t *testing.T) {
	input := `> hello world`
	result, err := ParseOrdered(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if s != "hello world" {
		t.Errorf("got %q, want %q", s, "hello world")
	}
}

func TestParseOrdered_TopLevelList(t *testing.T) {
	input := `
- one
- two
- three
`
	result, err := ParseOrdered(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	list, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	if len(list) != 3 || list[0] != "one" || list[1] != "two" || list[2] != "three" {
		t.Errorf("list = %v", list)
	}
}

func TestParseOrdered_EmptyInput(t *testing.T) {
	result, err := ParseOrdered(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v (%T)", result, result)
	}
}

func TestParseOrdered_MultilineKey(t *testing.T) {
	// Multi-line key syntax: ": key" lines followed by indented value
	input := `: key one
: continued
    > value1
: key two
    > value2
`
	result, err := ParseOrdered(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := result.(*Dict)
	if !ok {
		t.Fatalf("expected *Dict, got %T", result)
	}
	if got := d.Keys(); !reflect.DeepEqual(got, []string{"key one\ncontinued", "key two"}) {
		t.Errorf("keys = %v", got)
	}
}

func TestParse_DefaultReturnsMap(t *testing.T) {
	input := `
alpha: 1
beta: 2
`
	result, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	_, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}
}

func TestDict_Get(t *testing.T) {
	d := &Dict{entries: []DictEntry{
		{Key: "a", Value: "1"},
		{Key: "b", Value: "2"},
	}}

	v, ok := d.Get("a")
	if !ok || v != "1" {
		t.Errorf("Get(a) = %v, %v", v, ok)
	}

	v, ok = d.Get("b")
	if !ok || v != "2" {
		t.Errorf("Get(b) = %v, %v", v, ok)
	}

	_, ok = d.Get("missing")
	if ok {
		t.Error("Get(missing) should return false")
	}
}

func TestDict_Keys(t *testing.T) {
	d := &Dict{entries: []DictEntry{
		{Key: "z", Value: "1"},
		{Key: "a", Value: "2"},
		{Key: "m", Value: "3"},
	}}
	want := []string{"z", "a", "m"}
	if got := d.Keys(); !reflect.DeepEqual(got, want) {
		t.Errorf("Keys() = %v, want %v", got, want)
	}
}

func TestDict_Entries(t *testing.T) {
	entries := []DictEntry{
		{Key: "x", Value: "1"},
		{Key: "y", Value: "2"},
	}
	d := &Dict{entries: entries}
	got := d.Entries()
	if !reflect.DeepEqual(got, entries) {
		t.Errorf("Entries() = %v, want %v", got, entries)
	}
	// Verify it's a copy
	got[0].Key = "modified"
	if d.entries[0].Key == "modified" {
		t.Error("Entries() should return a copy")
	}
}

func TestDict_ToMap(t *testing.T) {
	inner := &Dict{entries: []DictEntry{
		{Key: "nested", Value: "val"},
	}}
	d := &Dict{entries: []DictEntry{
		{Key: "a", Value: "1"},
		{Key: "b", Value: inner},
	}}
	m := d.ToMap()
	want := map[string]interface{}{
		"a": "1",
		"b": map[string]interface{}{"nested": "val"},
	}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("ToMap() = %v, want %v", m, want)
	}
}

func TestDict_ToMap_WithLists(t *testing.T) {
	inner := &Dict{entries: []DictEntry{{Key: "x", Value: "1"}}}
	d := &Dict{entries: []DictEntry{
		{Key: "items", Value: []interface{}{"a", inner}},
	}}
	m := d.ToMap()
	want := map[string]interface{}{
		"items": []interface{}{"a", map[string]interface{}{"x": "1"}},
	}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("ToMap() = %v, want %v", m, want)
	}
}

func TestDict_EmptyMethods(t *testing.T) {
	d := NewDict()
	if d.Len() != 0 {
		t.Errorf("Len() = %d", d.Len())
	}
	if got := d.Keys(); len(got) != 0 {
		t.Errorf("Keys() = %v, want empty", got)
	}
	if got := d.Entries(); len(got) != 0 {
		t.Errorf("Entries() = %v, want empty", got)
	}
	if got := d.ToMap(); len(got) != 0 {
		t.Errorf("ToMap() = %v, want empty", got)
	}
	if _, ok := d.Get("anything"); ok {
		t.Error("Get on empty dict should return false")
	}
}

func TestDict_Len(t *testing.T) {
	d := NewDict()
	if d.Len() != 0 {
		t.Errorf("empty Len() = %d", d.Len())
	}
	d.Set("a", "1")
	d.Set("b", "2")
	if d.Len() != 2 {
		t.Errorf("Len() = %d, want 2", d.Len())
	}
}

func TestDict_Set(t *testing.T) {
	d := NewDict()
	d.Set("a", "1")
	d.Set("b", "2")
	d.Set("a", "updated") // update existing

	if d.Len() != 2 {
		t.Errorf("Len() = %d after Set update, want 2", d.Len())
	}
	v, _ := d.Get("a")
	if v != "updated" {
		t.Errorf("Get(a) = %v, want updated", v)
	}
	// Order should be preserved (a still first)
	if got := d.Keys(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("Keys() = %v after Set update", got)
	}
}
