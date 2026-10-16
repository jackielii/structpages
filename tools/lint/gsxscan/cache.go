package gsxscan

import (
	"go/token"
	"os"

	"github.com/gsxhq/gsx/ast"
	"github.com/gsxhq/gsx/parser"
	"github.com/jackielii/structpages/tools/lint/internal/filecache"
)

// cache holds every .gsx file's parse and the values derived from it,
// so the url-attr scan, the directives, the htmx scan and the
// attribute index share one parse per file per process.
var cache filecache.Cache

// parsed is a .gsx file's source and parse.
type parsed struct {
	src  []byte
	fset *token.FileSet
	file *ast.File
	// readErr is an I/O error; err a parse error.
	readErr, err error
}

// parse returns filename's parse, from the cache when it is current.
func parse(filename string) parsed {
	return memo(filename, "parse", func() parsed {
		src, err := os.ReadFile(filename)
		if err != nil {
			return parsed{readErr: err}
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filename, src, 0)
		return parsed{src: src, fset: fset, file: f, err: err}
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
