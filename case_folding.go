package structpages

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file converts component method names to the kebab-case form used in
// HTML ids and HTMX targets (camelToKebab).

// camelToKebab converts a CamelCase or camelCase name to kebab-case: its
// words lowercased and joined by single hyphens. Used by IDFor to generate
// HTML IDs from field, method and package names.
//
// A word starts at an upper-case letter that follows a lower-case one, or
// that is followed by one after an acronym ("HTMLParser" → "html-parser").
// Underscores and hyphens also separate words, so a Go name in snake_case
// or a hyphenated import path element kebab-cases too, and the result never
// starts or ends with a hyphen or holds two in a row, so a generated id
// never holds the "--" that separates a keyed id's key (see Key). Letters
// of any script are lowercased.
//
// Examples:
//   - "UserList" -> "user-list"
//   - "HTMLParser" -> "html-parser"
//   - "todoList" -> "todo-list"
//   - "Row_Edit" -> "row-edit"
//   - "ui-Kit" -> "ui-kit"
func camelToKebab(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(s) + 5) // Preallocate with some extra space for hyphens

	var prev rune
	sep := false // a hyphen is due before the next letter or digit
	for i, w := 0, 0; i < len(s); i += w {
		var r rune
		r, w = utf8.DecodeRuneInString(s[i:])
		if r == '_' || r == '-' {
			sep = true
			prev = r
			continue
		}
		if i > 0 && unicode.IsUpper(r) {
			// Not before every capital, to avoid "HTML" -> "h-t-m-l"
			next, _ := utf8.DecodeRuneInString(s[i+w:])
			if unicode.IsLower(prev) || unicode.IsLower(next) {
				sep = true
			}
		}
		if sep && result.Len() > 0 {
			result.WriteByte('-')
		}
		sep = false
		result.WriteRune(unicode.ToLower(r))
		prev = r
	}

	return result.String()
}
