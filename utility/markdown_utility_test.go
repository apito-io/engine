package utility

import (
	"strings"
	"testing"
)

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestMarkdownProcessor(t *testing.T) {
	processor := NewMarkdownProcessor()

	tests := []struct {
		name        string
		input       map[string]interface{}
		checkFields []string
	}{
		{
			name: "markdown to html and text",
			input: map[string]interface{}{
				"markdown": "# Hello World\n\nThis is **bold** and *italic* text.",
			},
			checkFields: []string{"markdown", "html", "text"},
		},
		{
			name: "html to markdown and text",
			input: map[string]interface{}{
				"html": "<h1>Hello World</h1><p>This is <strong>bold</strong> text.</p>",
			},
			checkFields: []string{"markdown", "html", "text"},
		},
		{
			name: "text to markdown",
			input: map[string]interface{}{
				"text": "Hello World\n\nThis is plain text.",
			},
			checkFields: []string{"markdown", "html", "text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.ProcessMultilineField(tt.input)

			// Check that all expected fields are present and not empty
			for _, field := range tt.checkFields {
				if actualValue, exists := result[field]; !exists {
					t.Errorf("Expected field %s to exist", field)
				} else if actualValue == "" {
					t.Errorf("Field %s should not be empty", field)
				} else {
					t.Logf("Field %s: %v", field, actualValue)
				}
			}
		})
	}
}

func TestSanitizeMarkdown(t *testing.T) {
	processor := NewMarkdownProcessor()

	// Test that potentially dangerous content is sanitized
	dangerousMarkdown := `<script>alert('xss')</script># Hello World`
	sanitized := processor.SanitizeMarkdown(dangerousMarkdown)

	if sanitized == dangerousMarkdown {
		t.Error("Expected markdown to be sanitized")
	}

	// Test that safe content is preserved
	safeMarkdown := "# Hello World\n\nThis is safe content."
	sanitizedSafe := processor.SanitizeMarkdown(safeMarkdown)

	if sanitizedSafe == "" {
		t.Error("Expected safe markdown to be preserved")
	}
}

func TestMarkdownToHTML(t *testing.T) {
	processor := NewMarkdownProcessor()

	markdown := "# Hello World\n\nThis is **bold** text."
	html := processor.MarkdownToHTML(markdown)

	// Check that HTML contains expected elements
	if html == "" {
		t.Error("Expected HTML output, got empty string")
	}
	if !containsString(html, "<h1") {
		t.Error("Expected HTML to contain h1 tag")
	}
	if !containsString(html, "<strong>") {
		t.Error("Expected HTML to contain strong tag")
	}
	t.Logf("Generated HTML: %s", html)
}

func TestHTMLToMarkdown(t *testing.T) {
	processor := NewMarkdownProcessor()

	html := "<h1>Hello World</h1><p>This is <strong>bold</strong> text.</p>"
	markdown := processor.HTMLToMarkdown(html)

	expectedMarkdown := "# Hello World\n\nThis is **bold** text."
	if markdown != expectedMarkdown {
		t.Errorf("Expected %s, got %s", expectedMarkdown, markdown)
	}
}

func TestMarkdownToText(t *testing.T) {
	processor := NewMarkdownProcessor()

	markdown := "# Hello World\n\nThis is **bold** and *italic* text."
	text := processor.MarkdownToText(markdown)

	// Check that text contains expected content
	if text == "" {
		t.Error("Expected text output, got empty string")
	}
	if !containsString(text, "Hello World") {
		t.Error("Expected text to contain 'Hello World'")
	}
	if !containsString(text, "bold") {
		t.Error("Expected text to contain 'bold'")
	}
	t.Logf("Generated text: %s", text)
}
