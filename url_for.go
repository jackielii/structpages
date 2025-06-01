package structpages

import (
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
func UrlFor(ctx context.Context, page any, args ...any) (string, error) {
	pc := pcCtx.Value(ctx)
	if pc == nil {
		return "", errors.New("parse context not found in context")
	}
	pattern, err := pc.urlFor(page)
	if err != nil {
		return "", err
	}
	return formatPathSegments(pattern, args...)
}

// TODO: see: go/src/net/http/pattern.go for more accurate path parsing
func formatPathSegments(pattern string, args ...any) (string, error) {
	segments := make([]string, 0)
	indicies := make([]int, 0)
	parts := strings.Split(pattern, "/")
	for i, part := range parts {
		if part == "{$}" {
			continue
		}
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			segments = append(segments, part[1:len(part)-1]) // remove { and }
			indicies = append(indicies, i)
		}
	}
	if len(args) == len(segments) {
		for i := range segments {
			parts[indicies[i]] = fmt.Sprint(args[i])
		}
	} else if len(args)/2 >= len(segments) {
		m := make(map[string]any)
		for i := 0; i < len(args); i += 2 {
			key, ok := args[i].(string)
			if !ok {
				return pattern, fmt.Errorf("pattern %s: argument %s not found in provided args", pattern, segments[i])
			}
			m[key] = args[i+1]
		}
		for i, segment := range segments {
			if value, ok := m[segment]; ok {
				parts[indicies[i]] = fmt.Sprint(value)
			} else {
				return pattern, fmt.Errorf("pattern %s: argument %s not found in provided args", pattern, segment)
			}
		}
	} else if len(args) == 1 {
		arg, ok := args[0].(map[string]any)
		if !ok {
			return pattern, fmt.Errorf("pattern %s: use map[string]any for single arg or provide the full args", pattern)
		}
		for i, segment := range segments {
			if value, ok := arg[segment]; ok {
				parts[indicies[i]] = fmt.Sprint(value)
			} else {
				return pattern, fmt.Errorf("pattern %s: argument %s not found in provided args", pattern, segment)
			}
		}
	} else if len(args) < len(segments) {
		return pattern, fmt.Errorf("pattern %s: not enough arguments provided for segment: %s", pattern, segments)
	} else if len(args) > len(segments) {
		// return pattern, fmt.Errorf("pattern %s: too many arguments provided for segment: %s", pattern, segments)
	}

	return strings.Join(parts, "/"), nil
}
