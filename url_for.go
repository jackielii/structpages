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
	var argMap map[string]any
	if len(args) == 1 {
		if arg, ok := args[0].(map[string]any); ok {
			argMap = arg
		}
	}
	parts := strings.Split(pattern, "/")
	j := 0
	for i, part := range parts {
		if part == "{$}" {
			continue
		}
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			if argMap != nil {
				key := part[1 : len(part)-1] // remove { and }
				if value, ok := argMap[key]; ok {
					parts[i] = fmt.Sprint(value)
					continue
				} else {
					return pattern, fmt.Errorf("pattern %s: argument %s not found in provided args", pattern, key)
				}
			} else {
				if j >= len(args) {
					return pattern, fmt.Errorf("pattern %s: not enough arguments provided for segment: %s", pattern, part)
				}
				parts[i] = fmt.Sprint(args[j])
				j++
			}
		}
	}
	return strings.Join(parts, "/"), nil
}
