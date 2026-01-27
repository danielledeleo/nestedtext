package nestedtext

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestReadmeExamples extracts NestedText examples from README.md and verifies they parse correctly.
func TestReadmeExamples(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}

	// Extract ```nestedtext code blocks
	nestedtextBlocks := extractCodeBlocks(string(readme), "nestedtext")
	for i, block := range nestedtextBlocks {
		_, err := Parse(strings.NewReader(block))
		if err != nil {
			t.Errorf("nestedtext block %d failed to parse:\n%s\nerror: %v", i+1, block, err)
		}
	}

	// Extract NestedText literals from Go code blocks (inside backtick strings)
	goBlocks := extractCodeBlocks(string(readme), "go")
	for _, block := range goBlocks {
		literals := extractNestedTextLiterals(block)
		for j, literal := range literals {
			_, err := Parse(strings.NewReader(literal))
			if err != nil {
				t.Errorf("NestedText literal %d in Go block failed to parse:\n%s\nerror: %v", j+1, literal, err)
			}
		}
	}
}

// extractCodeBlocks extracts fenced code blocks with the given language from markdown.
func extractCodeBlocks(markdown, language string) []string {
	pattern := "```" + language + "\n"
	var blocks []string

	for {
		start := strings.Index(markdown, pattern)
		if start == -1 {
			break
		}
		start += len(pattern)
		end := strings.Index(markdown[start:], "```")
		if end == -1 {
			break
		}
		blocks = append(blocks, markdown[start:start+end])
		markdown = markdown[start+end+3:]
	}

	return blocks
}

// extractNestedTextLiterals extracts NestedText content from Go backtick string literals.
// Looks for patterns like: []byte(`...`) or NewReader(`...`)
func extractNestedTextLiterals(goCode string) []string {
	var literals []string

	// Match backtick strings that look like NestedText (contain : or - at line start)
	re := regexp.MustCompile("`([^`]+)`")
	matches := re.FindAllStringSubmatch(goCode, -1)

	for _, match := range matches {
		content := match[1]
		// Check if it looks like NestedText (has key: value or - item patterns)
		if looksLikeNestedText(content) {
			literals = append(literals, content)
		}
	}

	return literals
}

// looksLikeNestedText returns true if the content appears to be NestedText.
func looksLikeNestedText(content string) bool {
	// Must have multiple lines to be a NestedText document
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return false
	}

	// Must not look like a struct tag (contains quotes around value)
	if strings.Contains(content, `:"`) {
		return false
	}

	dictEntries := 0
	listItems := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Count dict entries (key: value) and list items (- item)
		if strings.Contains(line, ": ") || strings.HasSuffix(trimmed, ":") {
			dictEntries++
		}
		if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
			listItems++
		}
	}

	// Must have at least 2 meaningful entries
	return dictEntries+listItems >= 2
}
