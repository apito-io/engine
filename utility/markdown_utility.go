package utility

import (
	"strings"

	"bytes"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/k3a/html2text"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// MarkdownProcessor handles markdown sanitization and conversion
type MarkdownProcessor struct {
	mdToHTML  goldmark.Markdown
	htmlToMD  *md.Converter
	sanitizer *bluemonday.Policy
}

// NewMarkdownProcessor creates a new markdown processor with proper sanitization
func NewMarkdownProcessor() *MarkdownProcessor {
	// Configure markdown to HTML converter with extensions
	mdToHTML := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown
			extension.DefinitionList,
			extension.Footnote,
			extension.Linkify,
			extension.Strikethrough,
			extension.Table,
			extension.TaskList,
			extension.Typographer,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)

	// Configure HTML to markdown converter
	htmlToMD := md.NewConverter("", true, nil)

	// Configure HTML sanitizer for user-generated content
	sanitizer := bluemonday.UGCPolicy()
	sanitizer.AllowAttrs("class").OnElements("div", "span", "p", "h1", "h2", "h3", "h4", "h5", "h6")
	sanitizer.AllowAttrs("style").OnElements("div", "span", "p", "h1", "h2", "h3", "h4", "h5", "h6")

	return &MarkdownProcessor{
		mdToHTML:  mdToHTML,
		htmlToMD:  htmlToMD,
		sanitizer: sanitizer,
	}
}

// ProcessMultilineField processes a multiline field with markdown as the source of truth
func (mp *MarkdownProcessor) ProcessMultilineField(input map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Priority: markdown > html > text
	var markdownContent string

	// Get markdown content (primary source)
	if md, ok := input["markdown"].(string); ok && md != "" {
		markdownContent = mp.SanitizeMarkdown(md)
	} else if html, ok := input["html"].(string); ok && html != "" {
		// Convert HTML to markdown if no markdown provided
		markdownContent = mp.HTMLToMarkdown(html)
	} else if text, ok := input["text"].(string); ok && text != "" {
		// Convert plain text to markdown if nothing else provided
		markdownContent = mp.TextToMarkdown(text)
	}

	if markdownContent == "" {
		return result
	}

	// Generate HTML from markdown
	htmlContent := mp.MarkdownToHTML(markdownContent)

	// Generate plain text from markdown (clean, no formatting)
	textContent := mp.MarkdownToText(markdownContent)

	result["markdown"] = markdownContent
	result["html"] = htmlContent
	result["text"] = textContent

	return result
}

// SanitizeMarkdown removes potentially dangerous content from markdown
func (mp *MarkdownProcessor) SanitizeMarkdown(markdown string) string {
	if markdown == "" {
		return ""
	}

	// Convert to HTML first to sanitize
	html := mp.MarkdownToHTML(markdown)

	// Sanitize the HTML
	sanitizedHTML := mp.sanitizer.Sanitize(html)

	// Convert back to markdown
	return mp.HTMLToMarkdown(sanitizedHTML)
}

// MarkdownToHTML converts markdown to sanitized HTML
func (mp *MarkdownProcessor) MarkdownToHTML(markdown string) string {
	if markdown == "" {
		return ""
	}

	var buf bytes.Buffer
	if err := mp.mdToHTML.Convert([]byte(markdown), &buf); err != nil {
		return ""
	}

	html := buf.String()

	// Sanitize the HTML
	return mp.sanitizer.Sanitize(html)
}

// HTMLToMarkdown converts HTML to markdown
func (mp *MarkdownProcessor) HTMLToMarkdown(html string) string {
	if html == "" {
		return ""
	}

	// Sanitize HTML first
	sanitizedHTML := mp.sanitizer.Sanitize(html)

	// Convert to markdown
	markdown, err := mp.htmlToMD.ConvertString(sanitizedHTML)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(markdown)
}

// MarkdownToText converts markdown to plain text (removes all formatting)
func (mp *MarkdownProcessor) MarkdownToText(markdown string) string {
	if markdown == "" {
		return ""
	}

	// Convert markdown to HTML first
	html := mp.MarkdownToHTML(markdown)

	// Convert HTML to plain text
	return html2text.HTML2Text(html)
}

// TextToMarkdown converts plain text to basic markdown
func (mp *MarkdownProcessor) TextToMarkdown(text string) string {
	if text == "" {
		return ""
	}

	// Simple text to markdown conversion
	// Split into paragraphs and wrap in markdown
	paragraphs := strings.Split(text, "\n\n")
	var markdownParagraphs []string

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			// Escape markdown special characters
			p = mp.escapeMarkdown(p)
			markdownParagraphs = append(markdownParagraphs, p)
		}
	}

	return strings.Join(markdownParagraphs, "\n\n")
}

// escapeMarkdown escapes markdown special characters
func (mp *MarkdownProcessor) escapeMarkdown(text string) string {
	// Escape backticks, asterisks, underscores, and other markdown special characters
	escaped := text
	escaped = strings.ReplaceAll(escaped, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "`", "\\`")
	escaped = strings.ReplaceAll(escaped, "*", "\\*")
	escaped = strings.ReplaceAll(escaped, "_", "\\_")
	escaped = strings.ReplaceAll(escaped, "#", "\\#")
	escaped = strings.ReplaceAll(escaped, "+", "\\+")
	escaped = strings.ReplaceAll(escaped, "-", "\\-")
	escaped = strings.ReplaceAll(escaped, ".", "\\.")
	escaped = strings.ReplaceAll(escaped, "!", "\\!")
	escaped = strings.ReplaceAll(escaped, "[", "\\[")
	escaped = strings.ReplaceAll(escaped, "]", "\\]")
	escaped = strings.ReplaceAll(escaped, "(", "\\(")
	escaped = strings.ReplaceAll(escaped, ")", "\\)")
	escaped = strings.ReplaceAll(escaped, "|", "\\|")

	return escaped
}

// Global instance for easy access
var DefaultMarkdownProcessor = NewMarkdownProcessor()

// Convenience functions using the default processor
func ProcessMultilineField(input map[string]interface{}) map[string]interface{} {
	return DefaultMarkdownProcessor.ProcessMultilineField(input)
}

func SanitizeMarkdown(markdown string) string {
	return DefaultMarkdownProcessor.SanitizeMarkdown(markdown)
}

func MarkdownToHTML(markdown string) string {
	return DefaultMarkdownProcessor.MarkdownToHTML(markdown)
}

func HTMLToMarkdown(html string) string {
	return DefaultMarkdownProcessor.HTMLToMarkdown(html)
}

func MarkdownToText(markdown string) string {
	return DefaultMarkdownProcessor.MarkdownToText(markdown)
}
