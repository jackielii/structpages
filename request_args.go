package structpages

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
)

// requestArg is a provider registered with WithRequestArgs or returned from
// a page's RequestArgs method: a func(*http.Request) (T, error) or
// func(*http.Request, *PageNode) (T, error) whose result fills parameters
// of type T for one request.
type requestArg struct {
	typ      reflect.Type // T, as declared
	fn       reflect.Value
	withNode bool   // takes the *PageNode as its second parameter
	origin   string // where it was registered, for error messages
}

var (
	requestType  = reflect.TypeFor[*http.Request]()
	pageNodeType = reflect.TypeFor[*PageNode]()
	errorType    = reflect.TypeFor[error]()
)

// newRequestArg validates that v has one of the two provider shapes.
func newRequestArg(v any, origin string) (*requestArg, error) {
	fn := reflect.ValueOf(v)
	if !fn.IsValid() || fn.Kind() != reflect.Func || fn.IsNil() {
		return nil, fmt.Errorf("%s: got %T, want func(*http.Request) (T, error) or "+
			"func(*http.Request, *PageNode) (T, error)", origin, v)
	}
	ft := fn.Type()
	shapeOK := ft.NumOut() == 2 && ft.Out(1) == errorType && !ft.IsVariadic() &&
		(ft.NumIn() == 1 || (ft.NumIn() == 2 && ft.In(1) == pageNodeType)) &&
		ft.In(0) == requestType
	if !shapeOK {
		return nil, fmt.Errorf("%s: got %s, want func(*http.Request) (T, error) or "+
			"func(*http.Request, *PageNode) (T, error)", origin, ft)
	}
	return &requestArg{typ: ft.Out(0), fn: fn, withNode: ft.NumIn() == 2, origin: origin}, nil
}

// requestArgLevel holds the providers one page (or WithRequestArgs, for
// the whole mount) contributes, keyed by the type they provide.
type requestArgLevel map[reflect.Type]*requestArg

func (l requestArgLevel) add(a *requestArg) error {
	if prev, ok := l[a.typ]; ok {
		return fmt.Errorf("%s: %s is already provided by %s", a.origin, a.typ, prev.origin)
	}
	l[a.typ] = a
	return nil
}

// match finds the provider in l that fills a parameter of type pt, by the
// rules WithArgs uses: a provider of exactly pt; a provider of *pt for a
// value parameter; for an interface parameter, the one provider whose type
// implements it. deref reports that the provided pointer is dereferenced.
// More than one implementation is an error.
func (l requestArgLevel) match(pt reflect.Type) (a *requestArg, deref bool, err error) {
	if a, ok := l[pt]; ok {
		return a, false, nil
	}
	if pt.Kind() != reflect.Pointer {
		if a, ok := l[reflect.PointerTo(pt)]; ok {
			return a, true, nil
		}
	}
	if pt.Kind() != reflect.Interface {
		return nil, false, nil
	}
	var impls []*requestArg
	for t, a := range l {
		if t.Implements(pt) {
			impls = append(impls, a)
		}
	}
	switch len(impls) {
	case 0:
		return nil, false, nil
	case 1:
		return impls[0], false, nil
	}
	names := make([]string, len(impls))
	for i, a := range impls {
		names[i] = a.typ.String()
	}
	slices.Sort(names)
	return nil, false, fmt.Errorf(
		"%s is implemented by more than one request arg (%s); "+
			"declare the parameter as the concrete type, or narrow the interface",
		pt, strings.Join(names, ", "))
}

// requestArgFor finds the provider for pt among levels, nearest first: the
// first level with a match wins, so a page's own provider shadows one of
// the same type further up.
func requestArgFor(levels []requestArgLevel, pt reflect.Type) (*requestArg, bool, error) {
	for _, l := range levels {
		a, deref, err := l.match(pt)
		if err != nil || a != nil {
			return a, deref, err
		}
	}
	return nil, false, nil
}

// requestScope is the per-request state for request args: each provider
// runs at most once per request, the first time a parameter needs it.
type requestScope struct {
	r      *http.Request
	pn     *PageNode
	values map[*requestArg]requestArgResult
}

type requestArgResult struct {
	v   reflect.Value
	err error
}

func newRequestScope(r *http.Request, pn *PageNode) *requestScope {
	return &requestScope{r: r, pn: pn}
}

// get finds and evaluates the request arg for a parameter of type pt.
// found is false when no provider applies to pn.
func (rs *requestScope) get(pt reflect.Type) (v reflect.Value, found bool, err error) {
	a, deref, err := requestArgFor(rs.pn.requestArgs, pt)
	if err != nil || a == nil {
		return reflect.Value{}, false, err
	}
	res, ok := rs.values[a]
	if !ok {
		res = rs.call(a)
		if rs.values == nil {
			rs.values = make(map[*requestArg]requestArgResult)
		}
		rs.values[a] = res
	}
	if res.err != nil {
		return reflect.Value{}, true, res.err
	}
	if deref {
		if res.v.IsNil() {
			return reflect.Value{}, true, fmt.Errorf("request arg %s (%s) is nil; "+
				"declare the parameter as %s to accept a nil value", a.typ, a.origin, a.typ)
		}
		return res.v.Elem(), true, nil
	}
	return res.v, true, nil
}

func (rs *requestScope) call(a *requestArg) requestArgResult {
	in := []reflect.Value{reflect.ValueOf(rs.r)}
	if a.withNode {
		in = append(in, reflect.ValueOf(rs.pn))
	}
	out := a.fn.Call(in)
	if err, _ := out[1].Interface().(error); err != nil {
		return requestArgResult{err: fmt.Errorf("request arg %s: %w", a.typ, err)}
	}
	return requestArgResult{v: out[0]}
}

// resolveRequestArgs builds every page's request-arg levels: its own
// RequestArgs, then each ancestor's, then WithRequestArgs. A page's
// RequestArgs method is called once here, with WithArgs values injected.
func (sp *StructPages) resolveRequestArgs() error {
	global := requestArgLevel{}
	for i, v := range sp.requestArgs {
		a, err := newRequestArg(v, fmt.Sprintf("WithRequestArgs argument %d", i+1))
		if err != nil {
			return err
		}
		if err := global.add(a); err != nil {
			return err
		}
	}
	var base []requestArgLevel
	if len(global) > 0 {
		base = []requestArgLevel{global}
	}
	return sp.resolvePageRequestArgs(sp.pc.root, base)
}

func (sp *StructPages) resolvePageRequestArgs(pn *PageNode, outer []requestArgLevel) error {
	levels := outer
	if pn.requestArgsMethod != nil {
		own, err := sp.pageRequestArgs(pn)
		if err != nil {
			return err
		}
		if len(own) > 0 {
			levels = append([]requestArgLevel{own}, outer...)
		}
	}
	pn.requestArgs = levels
	var errs []error
	for _, c := range pn.Children {
		errs = append(errs, sp.resolvePageRequestArgs(c, levels))
	}
	return errors.Join(errs...)
}

func (sp *StructPages) pageRequestArgs(pn *PageNode) (requestArgLevel, error) {
	res, err := sp.pc.callMethod(pn, pn.requestArgsMethod)
	if err != nil {
		return nil, fmt.Errorf("error calling RequestArgs method on %s: %w", pn.Name, err)
	}
	if len(res) != 1 || res[0].Type() != reflect.TypeFor[[]any]() {
		return nil, fmt.Errorf("RequestArgs method on %s must return []any, got %s",
			pn.Name, pn.requestArgsMethod.Type)
	}
	vs := res[0].Interface().([]any)
	own := requestArgLevel{}
	for i, v := range vs {
		a, err := newRequestArg(v, fmt.Sprintf("%s.RequestArgs element %d", pn.Name, i+1))
		if err != nil {
			return nil, err
		}
		if err := own.add(a); err != nil {
			return nil, err
		}
	}
	return own, nil
}
