package nestedtext

// Dict preserves insertion order of dictionary keys.
// It implements the NestedText spec requirement that dicts are ordered
// collections of name/value pairs.
//
// Use ParseOrdered to obtain *Dict values instead of map[string]interface{}.
type Dict struct {
	entries []DictEntry
}

// DictEntry is a single key-value pair in a Dict.
type DictEntry struct {
	Key   string
	Value interface{} // string, []interface{}, or *Dict
}

// NewDict creates an empty Dict.
func NewDict() *Dict {
	return &Dict{}
}

// Len returns the number of entries.
func (d *Dict) Len() int {
	return len(d.entries)
}

// Get returns the value for the given key and whether it was found.
func (d *Dict) Get(key string) (interface{}, bool) {
	for _, e := range d.entries {
		if e.Key == key {
			return e.Value, true
		}
	}
	return nil, false
}

// Keys returns all keys in insertion order.
func (d *Dict) Keys() []string {
	keys := make([]string, len(d.entries))
	for i, e := range d.entries {
		keys[i] = e.Key
	}
	return keys
}

// Entries returns all entries in insertion order.
// The returned slice is a copy; modifying it does not affect the Dict.
func (d *Dict) Entries() []DictEntry {
	out := make([]DictEntry, len(d.entries))
	copy(out, d.entries)
	return out
}

// ToMap recursively converts the Dict tree to plain Go types.
// *Dict becomes map[string]interface{}, and nested *Dict values within
// lists or dicts are also converted. Strings and []interface{} without
// nested *Dict values are returned as-is.
func (d *Dict) ToMap() map[string]interface{} {
	out := make(map[string]interface{}, len(d.entries))
	for _, e := range d.entries {
		out[e.Key] = toPlain(e.Value)
	}
	return out
}

// toPlain recursively converts *Dict values to map[string]interface{}.
func toPlain(v interface{}) interface{} {
	switch val := v.(type) {
	case *Dict:
		return val.ToMap()
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, item := range val {
			out[i] = toPlain(item)
		}
		return out
	default:
		return v
	}
}

// Set adds or updates a key-value pair. If the key already exists, its value
// is updated in place. Otherwise, the entry is appended.
func (d *Dict) Set(key string, value interface{}) {
	for i, e := range d.entries {
		if e.Key == key {
			d.entries[i].Value = value
			return
		}
	}
	d.entries = append(d.entries, DictEntry{Key: key, Value: value})
}
