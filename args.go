package structpages

import (
	"fmt"
	"reflect"
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

// note that p.args are always pointers
func (args argRegistry) getArg(pt reflect.Type) (reflect.Value, bool) {
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
