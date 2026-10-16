package structpages

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

type argRegistry map[reflect.Type]reflect.Value

func (args argRegistry) addArg(v any) error {
	if v == nil {
		return nil
	}
	typ := reflect.TypeOf(v)
	pv := reflect.ValueOf(v)
	if _, ok := args[typ]; ok {
		return fmt.Errorf("duplicate type %s in args registry", typ)
	}
	args[typ] = pv
	return nil
}

// getArg finds the registered value for a parameter of type pt. Values
// are registered under their dynamic types, so the only way to register
// under an interface type is a pointer to an interface variable; that
// fills an interface parameter first. Otherwise an interface parameter
// takes the one registered value whose type implements it; more than one
// is an error, since picking one would be a guess. A concrete parameter
// goes through getConcreteArg. found is false when nothing matches.
func (args argRegistry) getArg(pt reflect.Type) (v reflect.Value, found bool, err error) {
	if pt.Kind() == reflect.Interface {
		if pv, ok := args[reflect.PointerTo(pt)]; ok {
			return pv.Elem(), true, nil
		}
		return args.implementationOf(pt)
	}
	v, found = args.getConcreteArg(pt)
	return v, found, nil
}

// implementationOf returns the single registered value implementing
// iface.
func (args argRegistry) implementationOf(iface reflect.Type) (reflect.Value, bool, error) {
	var impls []reflect.Type
	for t := range args {
		if t.Implements(iface) {
			impls = append(impls, t)
		}
	}
	switch len(impls) {
	case 0:
		return reflect.Value{}, false, nil
	case 1:
		return args[impls[0]], true, nil
	}
	names := make([]string, len(impls))
	for i, t := range impls {
		names[i] = t.String()
	}
	slices.Sort(names)
	return reflect.Value{}, false, fmt.Errorf(
		"%s is implemented by more than one registered value (%s); "+
			"declare the parameter as the concrete type, or narrow the interface",
		iface, strings.Join(names, ", "))
}

// getConcreteArg finds the registered value for a non-interface
// parameter type. Note that p.args are always pointers.
func (args argRegistry) getConcreteArg(pt reflect.Type) (reflect.Value, bool) {
	st := pt
	needsElem, needsPtr := false, false
	if pt.Kind() != reflect.Pointer {
		needsElem = true
		pt = reflect.PointerTo(pt)
	}
	if st.Kind() == reflect.Pointer {
		needsPtr = true
		st = st.Elem()
	}

	if v, ok := args[pt]; ok {
		if needsElem {
			return v.Elem(), true
		}
		return v, true
	}

	if v, ok := args[st]; ok {
		if needsPtr {
			// Check if the value is addressable before calling Addr()
			if v.CanAddr() {
				return v.Addr(), true
			}
			// If not addressable, we can't convert to pointer
			return reflect.Value{}, false
		}
		return v, true
	}

	// Check assignability for less common cases
	for t, v := range args {
		// If looking for pointer type and found something assignable
		if needsPtr && pt.AssignableTo(t) {
			// We need to return a pointer, but can only do so if addressable
			if v.CanAddr() {
				return v.Addr(), true
			}
			// Skip non-addressable values when we need a pointer
			continue
		}

		// If looking for non-pointer type and found something assignable
		if !needsPtr && st.AssignableTo(t) {
			// For interface types, we typically don't need Elem()
			// This handles concrete types being assignable to interface types
			return v, true
		}
	}

	return reflect.Value{}, false
}
