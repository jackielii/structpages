package structpages

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackielii/ctxkey"
)

var (
	pcCtx        = ctxkey.New[*parseContext]("structpages.parseContext", nil)
	urlParamsCtx = ctxkey.New[map[string]string]("structpages.urlParams", nil)
)

// encodePathSegment converts a value to string and URL-encodes it for use in path segments.
// For wildcard segments ({path...}), slashes are preserved as they're part of the path.
func encodePathSegment(value any, isWildcard bool) string {
	s := fmt.Sprint(value)
	if isWildcard {
		// For wildcards, split on slashes, escape each part, then rejoin
		// This preserves slashes while properly encoding each path segment
		parts := strings.Split(s, "/")
		for i, part := range parts {
			parts[i] = url.PathEscape(part)
		}
		return strings.Join(parts, "/")
	}
	return url.PathEscape(s)
}

func withPcCtx(pc *parseContext) MiddlewareFunc {
	return func(next http.Handler, node *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := pcCtx.WithValue(r.Context(), pc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractURLParams extracts URL parameters from the request pattern and stores them in context
func extractURLParams(next http.Handler, node *PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := make(map[string]string)

		// Extract path values from the request
		pattern := r.Pattern
		if pattern != "" {
			// Parse the pattern to find parameter names
			segments, _ := parseSegments(pattern)
			for _, seg := range segments {
				if seg.param {
					// Get the actual value from the request
					value := r.PathValue(seg.name)
					if value != "" {
						params[seg.name] = value
					}
				}
			}
		}

		// Store params in context if any were found
		if len(params) > 0 {
			ctx := urlParamsCtx.WithValue(r.Context(), params)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// URLFor returns the URL for a given page type. If args is provided, it'll replace
// the path segments. Supported format is similar to http.ServeMux
//
// If multiple page type matches are found, the first one is returned.
// In such situation, use a func(*PageNode) bool as page argument to match a specific page.
//
// Additionally, you can pass []any to page to join multiple path segments together.
// Strings will be joined as is. Example:
//
//	URLFor(ctx, []any{Page{}, "?foo={bar}"}, "bar", "baz")
//
// It also supports a func(*PageNode) bool as the Page argument to match a specific page.
// It can be useful when you have multiple pages with the same type but different routes.
func URLFor(ctx context.Context, page any, args ...any) (string, error) {
	pc := pcCtx.Value(ctx)
	if pc == nil {
		return "", errors.New("parse context not found in context")
	}

	var pattern string
	parts, ok := page.([]any)
	if !ok {
		parts = []any{page}
	}
	for _, page := range parts {
		if s, ok := page.(string); ok {
			pattern += s
		} else {
			p, err := pc.urlFor(page)
			if err != nil {
				return "", err
			}
			pattern += p
		}
	}
	path, err := formatPathSegments(ctx, pattern, args...)
	if err != nil {
		return "", fmt.Errorf("urlfor: %w", err)
	}
	return strings.Replace(path, "{$}", "", 1), nil
}

// formatPathSegments formats URL pattern segments with provided arguments,
// using pre-extracted parameters from context if available.
// For more sophisticated path parsing, see Go's standard library implementation
// at go/src/net/http/pattern.go which handles edge cases like escaped braces.
//
//nolint:gocognit,gocyclo // This function handles multiple cases for flexible argument passing
func formatPathSegments(ctx context.Context, pattern string, args ...any) (string, error) {
	segments, err := parseSegments(pattern)
	if err != nil {
		return pattern, fmt.Errorf("pattern %s: %w", pattern, err)
	}
	indicies := make([]int, 0, len(segments)/2+1)
	for i, segment := range segments {
		if segment.param {
			indicies = append(indicies, i)
		}
	}
	if len(args) == 0 && len(indicies) == 0 {
		return pattern, nil // no args and no params, return the pattern as is
	}

	// Try to use pre-extracted parameters from context if no args provided
	if len(args) == 0 && len(indicies) > 0 {
		if params := urlParamsCtx.Value(ctx); params != nil {
			// Pre-fill segments with context parameters
			for _, idx := range indicies {
				name := segments[idx].name
				if value, ok := params[name]; ok {
					segments[idx].value = value
				}
			}
			// Check if all required params are filled
			allFilled := true
			for _, idx := range indicies {
				if segments[idx].value == "" {
					allFilled = false
					break
				}
			}
			if allFilled {
				s := ""
				for _, segment := range segments {
					s += cmp.Or(segment.value, segment.name)
				}
				return s, nil
			}
		}
		return pattern, fmt.Errorf("pattern %s: no arguments provided", pattern)
	}

	// Pre-fill segments with context parameters if available
	if params := urlParamsCtx.Value(ctx); params != nil {
		for _, idx := range indicies {
			name := segments[idx].name
			if value, ok := params[name]; ok {
				segments[idx].value = value
			}
		}
	}

	if arg, ok := args[0].(map[string]any); ok {
		for _, idx := range indicies {
			name := segments[idx].name
			if value, ok := arg[name]; ok {
				segments[idx].value = encodePathSegment(value, segments[idx].wildcard)
			}
			// If value not in args map, it should keep the pre-filled value from context
		}
		// Check if all params are filled
		for _, idx := range indicies {
			if segments[idx].value == "" {
				return pattern, fmt.Errorf("pattern %s: argument %s not found in provided args: %v",
					pattern, segments[idx].name, args)
			}
		}
	} else {
		switch {
		case len(args) == len(indicies):
			for i, idx := range indicies {
				// Always override with provided args when count matches exactly
				segments[idx].value = encodePathSegment(args[i], segments[idx].wildcard)
			}
		case len(args)%2 == 0 && len(args) >= 2:
			// Check if all even-indexed args are strings AND at least one matches a parameter name
			isPairs := true
			matchKey := false
			paramNames := make(map[string]bool)
			for _, idx := range indicies {
				paramNames[segments[idx].name] = true
			}

			for i := 0; i < len(args); i += 2 {
				key, ok := args[i].(string)
				if !ok {
					isPairs = false
					break
				}
				if paramNames[key] {
					matchKey = true
				}
			}

			// Only treat as key-value pairs if all even args are strings AND at least one matches
			if isPairs && matchKey {
				// If args are provided as key-value pairs, fill segments accordingly
				m := make(map[string]any)
				for i := 0; i < len(args); i += 2 {
					key := args[i].(string)
					m[key] = args[i+1]
				}
				for _, idx := range indicies {
					name := segments[idx].name
					if value, ok := m[name]; ok {
						segments[idx].value = encodePathSegment(value, segments[idx].wildcard)
					} else if segments[idx].value == "" {
						// Only error if no value from context either
						return pattern, fmt.Errorf("pattern %s: argument %s not found in provided args: %v", pattern, name, args)
					}
				}
				break
			}
			// If not valid key-value pairs, fall through to default
			fallthrough
		default:
			// Check if we have enough args considering pre-filled values
			unfilled := 0
			for _, idx := range indicies {
				if segments[idx].value == "" {
					unfilled++
				}
			}
			if len(args) < unfilled {
				return pattern, fmt.Errorf("pattern %s: not enough arguments provided, args: %v", pattern, args)
			}
			// Fill remaining unfilled params
			argIdx := 0
			for _, idx := range indicies {
				if segments[idx].value == "" && argIdx < len(args) {
					segments[idx].value = encodePathSegment(args[argIdx], segments[idx].wildcard)
					argIdx++
				}
			}
		}
	}

	s := ""
	for _, segment := range segments {
		s += cmp.Or(segment.value, segment.name)
	}

	return s, nil
}

type segment struct {
	name     string
	param    bool
	wildcard bool
	value    string
}

func parseSegments(pattern string) (segments []segment, err error) {
	if pattern == "" {
		return
	}
	rest := pattern
	for i := 0; ; i++ {
		if rest == "" {
			break
		}
		start := strings.Index(rest, "{")
		if start == -1 {
			segments = append(segments, segment{name: rest})
			break
		}
		if start > 0 {
			segments = append(segments, segment{name: rest[:start]})
		}
		rest = rest[start+1:] // move over the '{'
		end := strings.Index(rest, "}")
		if end == -1 {
			return nil, fmt.Errorf("pattern %s: unmatched {", pattern)
		}
		name := rest[:end]
		rest = rest[end+1:]
		if name == "$" { // skip {$} segments
			segments = append(segments, segment{name: "{$}"})
			continue
		}
		wildcard := strings.HasSuffix(name, "...")
		name = strings.TrimSuffix(name, "...")
		segments = append(segments, segment{name: name, param: true, wildcard: wildcard})
	}
	return segments, nil
}
