package templscan

import (
	"fmt"
	"strings"
)

// isInternalPath reports whether s looks like a literal internal app
// URL: starts with "/" but not "//" (which would be a
// protocol-relative URL).
func isInternalPath(s string) bool {
	if !strings.HasPrefix(s, "/") {
		return false
	}
	if strings.HasPrefix(s, "//") {
		return false
	}
	return true
}

// literalDiagnostic formats the "hard-coded internal URL" message.
func literalDiagnostic(attrName, literal string) string {
	return fmt.Sprintf(
		"[%s] %s value %q is a hard-coded internal URL; use structpages.URLFor instead",
		categoryURLAttr, attrName, literal,
	)
}
