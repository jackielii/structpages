package structpages

import (
	"net/http"
	"strings"
)

func HTMXPageConfig(r *http.Request) (string, error) {
	if isHTMX(r) {
		hxTarget := r.Header.Get("Hx-Target")
		if hxTarget != "" {
			return mixedCase(hxTarget), nil
		}
	}
	return "Page", nil
}

// MixedCase
func mixedCase(s string) string {
	if len(s) == 0 {
		return s
	}
	if strings.Contains(s, " ") {
		// TODO hx-target can't contain spaces, can we panic?
		return ""
	}
	parts := strings.Split(s, "-")
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("Hx-Request") == "true"
}
