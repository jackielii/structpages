package templscan

import (
	parser "github.com/a-h/templ/parser/v2"
	"github.com/jackielii/structpages/tools/lint/internal/filecache"
)

// cache holds every .templ file's parse and the values derived from
// it, so the url-attr scan, the directives, the htmx scan and the
// attribute index share one parse (and one in-process generation) per
// file per process. The generator only reads the parsed file.
var cache filecache.Cache

// parsed is a .templ file's parse.
type parsed struct {
	tf  *parser.TemplateFile
	err error
}

// parse returns filename's parse, from the cache when it is current.
func parse(filename string) parsed {
	return memo(filename, "parse", func() parsed {
		tf, err := parser.Parse(filename)
		return parsed{tf, err}
	})
}

// memo returns the value key of filename, computing it with compute on
// first use.
func memo[T any](filename, key string, compute func() T) T {
	return cache.Value(filename, key, func() any { return compute() }).(T)
}

// result pairs a value with its error for memo.
type result[T any] struct {
	val T
	err error
}
