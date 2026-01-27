package testsuite_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/nestext"
)

// This test runner tests the full NestedText test-suite, as proposed in the
// NestedText test proposal (https://github.com/kenkundert/nestedtext_tests).
// Current version tested against is 3.8
//
// Decoding-tests are checked via deep comparison. All tests pass.
//
// Encoding-tests are trickier, as for many structures there are more than one correct
// NT representations. Moreover, stability of map elements is a challenge: we sort
// them alphabetically, as Go does not make any guarantees about the sequence.
// All in all we are currently not testing encoding-cases to full depth, but in a
// sufficient manner.

var suiteFile = "official_tests/tests.json"

// TestSuite represents the v3.8 test suite JSON structure
type TestSuite struct {
	LoadTests map[string]LoadTestCase `json:"load_tests"`
	DumpTests map[string]DumpTestCase `json:"dump_tests"`
}

type LoadTestCase struct {
	LoadIn   string                 `json:"load_in"`   // base64-encoded NestedText input
	LoadOut  interface{}            `json:"load_out"`  // expected output (can be nil, string, list, or dict)
	LoadErr  map[string]interface{} `json:"load_err"`  // error details if error expected
	Encoding string                 `json:"encoding"`  // encoding (usually "utf-8")
	Types    map[string]int         `json:"types"`     // line type counts
}

type DumpTestCase struct {
	DumpIn  interface{}            `json:"dump_in"`  // input data to encode
	DumpOut string                 `json:"dump_out"` // base64-encoded expected NestedText output
	DumpErr map[string]interface{} `json:"dump_err"` // error details if error expected
}

func (tc *LoadTestCase) expectsError() bool {
	return len(tc.LoadErr) > 0
}

func (tc *LoadTestCase) decodeInput() ([]byte, error) {
	return base64.StdEncoding.DecodeString(tc.LoadIn)
}

func (tc *DumpTestCase) expectsError() bool {
	return len(tc.DumpErr) > 0
}

func (tc *DumpTestCase) decodeOutput() ([]byte, error) {
	return base64.StdEncoding.DecodeString(tc.DumpOut)
}

type ntTestCase struct {
	name    string
	load    *LoadTestCase
	dump    *DumpTestCase
	status  string
	statusD string
	statusE string
	isFail  bool
}

var skipped = []string{
	//"inline_dict_01",
}

func contains(l []string, s string) bool {
	for _, a := range l {
		if a == s {
			return true
		}
	}
	return false
}

func TestAll(t *testing.T) {
	suite := loadTestSuite(t)
	cases := buildTestCases(suite)

	min, max := 0, len(cases)-1
	for i := range cases[min : max+1] {
		c := &cases[min+i]
		if contains(skipped, c.name) {
			c.status = "skipped"
			continue
		}
		runTestCase(c, t)
	}
	failcnt := 0
	for i, c := range cases[min : max+1] {
		t.Logf("test (%03d) %-21q: %10s  [ %-5s , %-5s ]", min+i, c.name, c.status, c.statusD, c.statusE)
		if c.isFail {
			failcnt++
		}
	}
	t.Logf("%d out of %d tests failed", failcnt, len(cases))
}

func loadTestSuite(t *testing.T) *TestSuite {
	data, err := os.ReadFile(suiteFile)
	if err != nil {
		t.Fatalf("Failed to load test suite: %v", err)
	}

	var suite TestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("Failed to parse test suite JSON: %v", err)
	}

	return &suite
}

func buildTestCases(suite *TestSuite) []ntTestCase {
	// Collect all unique test names
	names := make(map[string]bool)
	for name := range suite.LoadTests {
		names[name] = true
	}
	for name := range suite.DumpTests {
		names[name] = true
	}

	cases := make([]ntTestCase, 0, len(names))
	for name := range names {
		c := ntTestCase{name: name}
		if load, ok := suite.LoadTests[name]; ok {
			c.load = &load
		}
		if dump, ok := suite.DumpTests[name]; ok {
			c.dump = &dump
		}
		cases = append(cases, c)
	}
	return cases
}

func runTestCase(c *ntTestCase, t *testing.T) {
	c.status = "loaded"
	testDecodeCase(c, t)
	testEncodeCase(c, t)
}

func testDecodeCase(c *ntTestCase, t *testing.T) {
	if c.load == nil {
		return
	}

	input, err := c.load.decodeInput()
	if err != nil {
		c.statusD = fmt.Sprintf("base64 error: %s", err.Error())
		c.isFail = true
		return
	}

	nt, parseErr := nestext.Parse(strings.NewReader(string(input)))

	if c.load.expectsError() {
		if parseErr == nil {
			c.statusD = "expected error"
			c.isFail = true
			return
		}
		c.statusD = "ok"
		return
	}

	if parseErr != nil {
		c.statusD = parseErr.Error()
		c.isFail = true
		return
	}

	c.statusD = "parsed"
	if deepEqual(nt, c.load.LoadOut) {
		c.statusD = "ok"
	} else {
		t.Logf("input:\n%s", string(input))
		t.Logf("nt   : %#v", nt)
		t.Logf("json : %#v", c.load.LoadOut)
		c.statusD = "mismatch"
		c.isFail = true
	}
}

func testEncodeCase(c *ntTestCase, t *testing.T) {
	if c.dump == nil {
		return
	}

	buf := &bytes.Buffer{}
	_, err := nestext.Encode(c.dump.DumpIn, buf, nestext.IndentBy(4))

	if c.dump.expectsError() {
		if err == nil {
			c.statusE = "expected error"
			c.isFail = true
			return
		}
		c.statusE = "ok"
		return
	}

	if err != nil {
		c.statusE = fmt.Sprintf("error: %s", err.Error())
		c.isFail = true
		return
	}

	c.statusE = "encoded"

	expected, decErr := c.dump.decodeOutput()
	if decErr != nil {
		c.statusE = fmt.Sprintf("base64 error: %s", decErr.Error())
		c.isFail = true
		return
	}

	// Compare output (trim trailing newlines for comparison)
	n := strings.TrimRight(string(expected), "\n")
	m := strings.TrimRight(buf.String(), "\n")
	if n == m {
		c.statusE = "ok"
	} else {
		c.statusE = "?"
	}
}

// deepEqual compares two values for equality, handling the JSON/Go type differences.
// JSON decodes numbers as float64, but NestedText only produces strings, lists, and maps.
func deepEqual(got, expected interface{}) bool {
	if got == nil && expected == nil {
		return true
	}
	if got == nil || expected == nil {
		return false
	}

	// Handle string comparison
	gotStr, gotIsStr := got.(string)
	expStr, expIsStr := expected.(string)
	if gotIsStr && expIsStr {
		return gotStr == expStr
	}

	// Handle slice comparison
	gotSlice, gotIsSlice := got.([]interface{})
	expSlice, expIsSlice := expected.([]interface{})
	if gotIsSlice && expIsSlice {
		if len(gotSlice) != len(expSlice) {
			return false
		}
		for i := range gotSlice {
			if !deepEqual(gotSlice[i], expSlice[i]) {
				return false
			}
		}
		return true
	}

	// Handle map comparison
	gotMap, gotIsMap := got.(map[string]interface{})
	expMap, expIsMap := expected.(map[string]interface{})
	if gotIsMap && expIsMap {
		if len(gotMap) != len(expMap) {
			return false
		}
		for k, v := range gotMap {
			expV, ok := expMap[k]
			if !ok {
				return false
			}
			if !deepEqual(v, expV) {
				return false
			}
		}
		return true
	}

	// Fallback to reflect.DeepEqual for other cases
	return reflect.DeepEqual(got, expected)
}
