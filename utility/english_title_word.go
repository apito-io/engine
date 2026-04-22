package utility

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// englishTitleWord returns a title-cased word for Apito model-id segments (underscore-separated
// parts, camel splits, etc.). It avoids golang.org/x/text/cases which has panicked in production
// with transform.String slice bounds errors on some ASCII segment lengths / inputs.
func englishTitleWord(word string) string {
	word = strings.ToValidUTF8(strings.TrimSpace(word), "")
	if word == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(word)
	if r == utf8.RuneError && n == 1 {
		return word
	}
	rest := word[n:]
	return string(unicode.ToUpper(r)) + strings.ToLower(rest)
}
