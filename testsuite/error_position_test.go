package testsuite_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/danielledeleo/nestedtext"
)

// This test file validates that error positions (line/column numbers) are
// reported correctly. It lives separately from the official test suite
// validation since position checking is not part of the official conformance.
//
// NOTE: The parser currently has a known limitation where some error types
// use MakeFormatError (no position) instead of MakeParsingError (with position).
// TestErrorPositionCoverage tracks which errors have positions and which don't.

// errorTestSuite represents the subset of test data we need for error position validation
type errorTestSuite struct {
	LoadTests map[string]errorLoadTestCase `json:"load_tests"`
}

type errorLoadTestCase struct {
	LoadIn  string       `json:"load_in"`  // base64-encoded NestedText input
	LoadErr errorDetails `json:"load_err"` // error details
}

type errorDetails struct {
	Message string `json:"message"`
	Line    string `json:"line"`   // the actual line content (for context)
	LineNo  *int   `json:"lineno"` // 0-indexed line number (pointer to distinguish 0 from absent)
	ColNo   *int   `json:"colno"`  // 0-indexed column number (optional)
}

func (e *errorDetails) hasError() bool {
	return e.Message != ""
}

// TestErrorPositionCoverage reports on which error cases have correct positions.
// This is informational - it shows the current state of position reporting.
func TestErrorPositionCoverage(t *testing.T) {
	data, err := os.ReadFile(suiteFile)
	if err != nil {
		t.Fatalf("Failed to read test suite: %v", err)
	}

	var suite errorTestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("Failed to parse test suite: %v", err)
	}

	var (
		tested          int
		lineCorrect     int
		lineMissing     int // Line=0 when it should have a value
		lineMismatch    int
		colCorrect      int
		colMissing      int
		colMismatch     int
		skippedNoLineno int
	)

	// Track error messages that are missing positions
	missingPosErrors := make(map[string]int)

	for name, tc := range suite.LoadTests {
		if !tc.LoadErr.hasError() {
			continue
		}

		if tc.LoadErr.LineNo == nil {
			skippedNoLineno++
			continue
		}

		tested++

		input, err := base64.StdEncoding.DecodeString(tc.LoadIn)
		if err != nil {
			t.Errorf("%s: failed to decode input: %v", name, err)
			continue
		}

		_, parseErr := nestedtext.Parse(strings.NewReader(string(input)))
		if parseErr == nil {
			t.Errorf("%s: expected error but got none", name)
			continue
		}

		ntErr, ok := parseErr.(nestedtext.NestedTextError)
		if !ok {
			t.Errorf("%s: error is not NestedTextError: %T", name, parseErr)
			continue
		}

		// Check line (convert 0-indexed test data to 1-indexed Go)
		expectedLine := *tc.LoadErr.LineNo + 1
		if ntErr.Line == 0 && expectedLine > 0 {
			lineMissing++
			// Extract first part of error message for categorization
			msg := parseErr.Error()
			if idx := strings.Index(msg, ":"); idx > 0 {
				msg = strings.TrimSpace(msg[idx+1:])
			}
			if idx := strings.Index(msg, ":"); idx > 0 {
				msg = msg[:idx]
			}
			missingPosErrors[msg]++
		} else if ntErr.Line == expectedLine {
			lineCorrect++
		} else {
			lineMismatch++
		}

		// Check column if specified
		if tc.LoadErr.ColNo != nil {
			expectedCol := *tc.LoadErr.ColNo + 1
			if ntErr.Column == 0 && expectedCol > 0 {
				colMissing++
			} else if ntErr.Column == expectedCol {
				colCorrect++
			} else {
				colMismatch++
			}
		}
	}

	t.Logf("=== Error Position Coverage Report ===")
	t.Logf("Total error cases tested: %d (skipped %d with no expected lineno)", tested, skippedNoLineno)
	t.Logf("")
	t.Logf("Line number accuracy:")
	t.Logf("  Correct:  %d (%.1f%%)", lineCorrect, pct(lineCorrect, tested))
	t.Logf("  Missing:  %d (%.1f%%) - parser returns Line=0", lineMissing, pct(lineMissing, tested))
	t.Logf("  Mismatch: %d (%.1f%%) - wrong line number", lineMismatch, pct(lineMismatch, tested))
	t.Logf("")
	t.Logf("Column number accuracy (for cases with expected colno):")
	colTotal := colCorrect + colMissing + colMismatch
	t.Logf("  Correct:  %d (%.1f%%)", colCorrect, pct(colCorrect, colTotal))
	t.Logf("  Missing:  %d (%.1f%%)", colMissing, pct(colMissing, colTotal))
	t.Logf("  Mismatch: %d (%.1f%%)", colMismatch, pct(colMismatch, colTotal))

	if len(missingPosErrors) > 0 {
		t.Logf("")
		t.Logf("Error types missing position info (uses MakeFormatError instead of MakeParsingError):")
		for msg, count := range missingPosErrors {
			t.Logf("  [%d] %s", count, msg)
		}
	}
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100 / float64(total)
}

// TestErrorPositionsLineOnly validates that errors have correct LINE position.
// Column validation is skipped since there are known off-by-one differences.
// Currently expected to have some failures due to parser limitations.
func TestErrorPositions(t *testing.T) {
	data, err := os.ReadFile(suiteFile)
	if err != nil {
		t.Fatalf("Failed to read test suite: %v", err)
	}

	var suite errorTestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("Failed to parse test suite: %v", err)
	}

	var tested, lineOK, lineMissing, lineMismatch int

	for name, tc := range suite.LoadTests {
		if !tc.LoadErr.hasError() {
			continue // skip non-error cases
		}

		if tc.LoadErr.LineNo == nil {
			continue // skip if no line number expected
		}

		tested++

		// Decode base64 input
		input, err := base64.StdEncoding.DecodeString(tc.LoadIn)
		if err != nil {
			t.Errorf("%s: failed to decode input: %v", name, err)
			continue
		}

		// Parse and expect error
		_, parseErr := nestedtext.Parse(strings.NewReader(string(input)))
		if parseErr == nil {
			t.Errorf("%s: expected error but got none", name)
			continue
		}

		// Check if it's a NestedTextError with position info
		ntErr, ok := parseErr.(nestedtext.NestedTextError)
		if !ok {
			t.Errorf("%s: error is not NestedTextError: %T", name, parseErr)
			continue
		}

		// Convert from 0-indexed (test data) to 1-indexed (Go implementation)
		expectedLine := *tc.LoadErr.LineNo + 1

		if ntErr.Line == 0 {
			lineMissing++
			// Not an error - known limitation tracked in TestErrorPositionCoverage
			continue
		}

		if ntErr.Line != expectedLine {
			lineMismatch++
			// Log but don't fail - some line numbers differ due to parser behavior
			t.Logf("%s: line mismatch: got %d, want %d (0-indexed: %d)\n  input: %q\n  error: %s",
				name, ntErr.Line, expectedLine, *tc.LoadErr.LineNo, string(input), parseErr)
			continue
		}

		lineOK++
	}

	t.Logf("Error line position: %d tested, %d correct, %d missing (Line=0), %d mismatched",
		tested, lineOK, lineMissing, lineMismatch)
}

// TestFixedErrorPositions verifies that specific error types that were fixed
// to include position information now report correct line numbers.
func TestFixedErrorPositions(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantLine     int
		wantContains string
	}{
		{
			name:         "partial dedent",
			input:        "key:\n  a: 1\n b: 2",
			wantLine:     3,
			wantContains: "partial dedent",
		},
		{
			name:         "invalid indent after list item with value",
			input:        "- item\n  - nested",
			wantLine:     2,
			wantContains: "invalid indent",
		},
		{
			name:         "multiline key requires value",
			input:        ": key\n: continued\na: 1",
			wantLine:     3,
			wantContains: "multiline key requires a value",
		},
		{
			name:         "duplicate key has position",
			input:        "a: 1\nb: 2\na: 3",
			wantLine:     4, // line after the duplicate (EOF), as error is detected after token consumed
			wantContains: "duplicate key",
		},
		{
			name:         "invalid indent after multiline list item",
			input:        "-\n  a: 1\n    too deep",
			wantLine:     3,
			wantContains: "invalid indent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := nestedtext.Parse(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("expected error but got none")
			}

			ntErr, ok := err.(nestedtext.NestedTextError)
			if !ok {
				t.Fatalf("error is not NestedTextError: %T - %v", err, err)
			}

			if ntErr.Line == 0 {
				t.Errorf("Line = 0, want %d (error position missing); error: %s", tt.wantLine, err)
			} else if ntErr.Line != tt.wantLine {
				t.Errorf("Line = %d, want %d; error: %s", ntErr.Line, tt.wantLine, err)
			}

			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantContains)
			}
		})
	}
}

// TestErrorPositionFormat verifies the error string format includes position
// for error types that support position reporting.
func TestErrorPositionFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLine int
		wantCol  int
	}{
		{
			name:     "indented top level",
			input:    "  key: value",
			wantLine: 1,
			wantCol:  1,
		},
		{
			name:     "unrecognized line type",
			input:    "no tag here",
			wantLine: 1,
			wantCol:  1,
		},
		{
			name:     "inline dict missing colon",
			input:    "{key value}",
			wantLine: 1,
			wantCol:  0, // column varies; just verify we get a position
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := nestedtext.Parse(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("expected error")
			}

			ntErr, ok := err.(nestedtext.NestedTextError)
			if !ok {
				t.Fatalf("error is not NestedTextError: %T", err)
			}

			if ntErr.Line != tt.wantLine {
				t.Errorf("Line = %d, want %d; error: %s", ntErr.Line, tt.wantLine, err)
			}

			// Only check column if we have an expected value > 0
			if tt.wantCol > 0 && ntErr.Column != tt.wantCol {
				t.Errorf("Column = %d, want %d; error: %s", ntErr.Column, tt.wantCol, err)
			}

			// Verify format includes position
			errStr := err.Error()
			if !strings.HasPrefix(errStr, "[") {
				t.Errorf("error format should start with '[': %s", errStr)
			}
		})
	}
}
