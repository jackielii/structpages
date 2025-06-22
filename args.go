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
	if pt.Kind() != reflect.Ptr {
		needsElem = true
		pt = reflect.PointerTo(pt)
	}
	if st.Kind() == reflect.Ptr {
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
			// TODO: some values are not addressable
			return v.Addr(), true
		}
		return v, true
	}

	for t, v := range args {
		if pt.AssignableTo(t) {
			if needsPtr {
				return v.Addr(), true
			}
			return v, true
		}
		if st.AssignableTo(t) {
			if needsElem {
				return v.Elem(), true
			}
			return v, true
		}
	}

	return reflect.Value{}, false
}
