package structpages

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackielii/ctxkey"
)

var pcCtx = ctxkey.New[*parseContext]("structpages.parseContext", nil)

func withPcCtx(pc *parseContext) MiddlewareFunc {
	return func(next http.Handler, node *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := pcCtx.WithValue(r.Context(), pc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UrlFor returns the URL for a given page type. If args is provided, it'll replace
// the path segments. Supported format is similar to http.ServeMux
//
// If multiple page type matches are found, the first one is returned.
// In such situation, use a func(*PageNode) bool as page argument to match a specific page.
//
// Additionally, you can pass []any to page to join multiple path segments together.
// Strings will be joined as is. Example:
//
//	UrlFor(ctx, []any{ListProductPage{}, "?page={page}"}, "category", categoryId, "page", pageNumber)
func UrlFor(ctx context.Context, page any, args ...any) (string, error) {
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
	return formatPathSegments(pattern, args...)
}

// TODO: see: go/src/net/http/pattern.go for more accurate path parsing
func formatPathSegments(pattern string, args ...any) (string, error) {
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
	if len(args) == len(indicies) {
		for i, idx := range indicies {
			segments[idx].value = fmt.Sprint(args[i])
		}
	} else if len(args)/2 >= len(indicies) {
		m := make(map[string]any)
		for i := 0; i < len(args); i += 2 {
			key, ok := args[i].(string)
			if !ok {
				return pattern, fmt.Errorf("pattern %s: arg pairs should have string as key: %v", pattern, args[i])
			}
			m[key] = args[i+1]
		}
		for _, idx := range indicies {
			name := segments[idx].name
			if value, ok := m[name]; ok {
				segments[idx].value = fmt.Sprint(value)
			} else {
				return pattern, fmt.Errorf("pattern %s: argument %s not found in provided args: %v", pattern, name, args)
			}
		}
	} else if len(args) == 1 {
		arg, ok := args[0].(map[string]any)
		if !ok {
			return pattern, fmt.Errorf("pattern %s: use map[string]any for single arg or provide the full args", pattern)
		}
		for _, idx := range indicies {
			name := segments[idx].name
			if value, ok := arg[name]; ok {
				segments[idx].value = fmt.Sprint(value)
			} else {
				return pattern, fmt.Errorf("pattern %s: argument %s not found in provided args: %v", pattern, name, args)
			}
		}
	} else if len(args) < len(indicies) {
		return pattern, fmt.Errorf("pattern %s: not enough arguments provided, args: %v", pattern, args)
	} else if len(args) > len(segments) {
		// return pattern, fmt.Errorf("pattern %s: too many arguments provided for segment: %s", pattern, segments)
	}

	s := ""
	for _, segment := range segments {
		s += cmp.Or(segment.value, segment.name)
	}

	return s, nil
}

type segment struct {
	name  string
	param bool
	value string
}

func parseSegments(pattern string) (segments []segment, err error) {
	if pattern == "" {
		return
	}
	rest := pattern
	for i := 0; ; i++ {
		if len(rest) == 0 {
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
		segments = append(segments, segment{name: name, param: true})
	}
	return segments, nil
}
