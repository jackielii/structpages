package structpages

import "strings"

// This file handles the bidirectional conversion between component method names
// and HTML IDs/HTMX targets.
//
// Marshal:   Method name → HTML ID    (camelToKebab)
// Unmarshal: HTML ID → Method name    (kebabToPascal)
//
// Example round-trip:
//   TodoList → todo-list → TodoList
//   HTMLParser → html-parser → HtmlParser

// camelToKebab converts a CamelCase or camelCase string to kebab-case.
// Used by IDFor to generate HTML IDs from method names.
//
// Examples:
//   - "UserList" -> "user-list"
//   - "HTMLParser" -> "html-parser"
//   - "todoList" -> "todo-list"
func camelToKebab(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(s) + 5) // Preallocate with some extra space for hyphens

	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Add hyphen before uppercase letter (but not at start)
			// Check if previous char was not uppercase to avoid "HTML" -> "h-t-m-l"
			if i > 0 && s[i-1] >= 'a' && s[i-1] <= 'z' {
				result.WriteByte('-')
			} else if i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
				// Handle "HTMLParser" -> "html-parser" (uppercase sequence followed by lowercase)
				if i > 0 {
					result.WriteByte('-')
				}
			}
		}
		result.WriteRune(r | ('a' - 'A')) // Convert to lowercase using bit operation
	}

	return result.String()
}

// kebabToPascal converts kebab-case to PascalCase.
// Used by HTMXPageConfig to convert HX-Target values to method names.
//
// Examples:
//   - "todo-list" -> "TodoList"
//   - "html-parser" -> "HtmlParser"
//   - "content" -> "Content"
func kebabToPascal(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Split(s, "-")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}
