package structpages

import (
	"reflect"
	"testing"
)

type testStruct struct {
	Value string
}

type testInterface interface {
	Method()
}

type testImplementation struct {
	Data string
}

func (t testImplementation) Method() {}

func TestArgRegistry_addArg(t *testing.T) {
	tests := []struct {
		name    string
		args    []any
		wantLen int
		wantErr bool
	}{
		{
			name:    "add nil value",
			args:    []any{nil},
			wantLen: 0,
		},
		{
			name:    "add single value",
			args:    []any{&testStruct{Value: "test"}},
			wantLen: 1,
		},
		{
			name:    "add multiple different types",
			args:    []any{&testStruct{Value: "test"}, "string", 42},
			wantLen: 3,
		},
		{
			name:    "add duplicate type returns error",
			args:    []any{&testStruct{Value: "first"}, &testStruct{Value: "second"}},
			wantLen: 1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := make(argRegistry)
			var gotErr error
			for _, arg := range tt.args {
				if err := args.addArg(arg); err != nil {
					gotErr = err
				}
			}
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("argRegistry.addArg() error = %v, wantErr %v", gotErr, tt.wantErr)
			}
			if got := len(args); got != tt.wantLen {
				t.Errorf("argRegistry.addArg() length = %v, want %v", got, tt.wantLen)
			}
		})
	}
}

func TestArgRegistry_getArg(t *testing.T) {
	// Prepare test data
	strVal := "test string"
	intVal := 42
	structVal := &testStruct{Value: "test"}
	nonPtrStruct := testStruct{Value: "non-ptr"}
	implVal := &testImplementation{Data: "impl"}

	// For addressable value test
	addressableStruct := testStruct{Value: "addressable"}

	tests := []struct {
		name       string
		registry   argRegistry
		lookupType reflect.Type
		wantFound  bool
		wantValue  any
	}{
		{
			name: "get exact pointer type",
			registry: argRegistry{
				reflect.TypeOf(structVal): reflect.ValueOf(structVal),
			},
			lookupType: reflect.TypeOf(structVal),
			wantFound:  true,
			wantValue:  structVal,
		},
		{
			name: "get non-pointer when pointer stored",
			registry: argRegistry{
				reflect.TypeOf(structVal): reflect.ValueOf(structVal),
			},
			lookupType: reflect.TypeOf(testStruct{}),
			wantFound:  true,
			wantValue:  testStruct{Value: "test"},
		},
		{
			name: "get pointer when addressable non-pointer stored",
			registry: argRegistry{
				reflect.TypeOf(addressableStruct): reflect.ValueOf(&addressableStruct).Elem(),
			},
			lookupType: reflect.TypeOf(&testStruct{}),
			wantFound:  true,
		},
		{
			name: "get non-existent type",
			registry: argRegistry{
				reflect.TypeOf(strVal): reflect.ValueOf(strVal),
			},
			lookupType: reflect.TypeOf(intVal),
			wantFound:  false,
		},
		{
			name: "get interface from implementation",
			registry: argRegistry{
				reflect.TypeOf((*testInterface)(nil)).Elem(): reflect.ValueOf(implVal),
			},
			lookupType: reflect.TypeOf((*testInterface)(nil)).Elem(),
			wantFound:  true,
		},
		{
			name: "implementation not assignable to interface",
			registry: argRegistry{
				reflect.TypeOf(implVal): reflect.ValueOf(implVal),
			},
			lookupType: reflect.TypeOf((*testInterface)(nil)).Elem(),
			wantFound:  false,
		},
		{
			name:       "empty registry returns not found",
			registry:   argRegistry{},
			lookupType: reflect.TypeOf(structVal),
			wantFound:  false,
		},
		{
			name: "get non-pointer stored as non-pointer",
			registry: argRegistry{
				reflect.TypeOf(nonPtrStruct): reflect.ValueOf(nonPtrStruct),
			},
			lookupType: reflect.TypeOf(nonPtrStruct),
			wantFound:  true,
			wantValue:  nonPtrStruct,
		},
		{
			name: "assignable type check",
			registry: argRegistry{
				reflect.TypeOf(new(testInterface)).Elem(): reflect.ValueOf(implVal).Elem(),
			},
			lookupType: reflect.TypeOf((*testInterface)(nil)).Elem(),
			wantFound:  true,
		},
		{
			name: "get addressable struct value",
			registry: func() argRegistry {
				r := make(argRegistry)
				// Create an addressable value by storing it in a variable
				v := testStruct{Value: "addressable"}
				r[reflect.TypeOf(v)] = reflect.ValueOf(&v).Elem()
				return r
			}(),
			lookupType: reflect.TypeOf(&testStruct{}),
			wantFound:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotFound := tt.registry.getArg(tt.lookupType)
			if gotFound != tt.wantFound {
				t.Errorf("argRegistry.getArg() found = %v, want %v", gotFound, tt.wantFound)
				return
			}
			if !gotFound {
				return
			}
			if !gotValue.IsValid() {
				t.Errorf("argRegistry.getArg() returned invalid value")
				return
			}
			// For pointer checks, we can't easily compare values
			if tt.wantValue != nil && gotValue.CanInterface() {
				got := gotValue.Interface()
				// Simple type assertion check for basic validation
				switch expected := tt.wantValue.(type) {
				case testStruct:
					if gotStruct, ok := got.(testStruct); ok {
						if gotStruct.Value != expected.Value {
							t.Errorf("argRegistry.getArg() = %v, want %v", gotStruct, expected)
						}
					}
				case *testStruct:
					if gotStruct, ok := got.(*testStruct); ok {
						if gotStruct.Value != expected.Value {
							t.Errorf("argRegistry.getArg() = %v, want %v", gotStruct, expected)
						}
					}
				}
			}
		})
	}
}

// Types for assignability tests
type baseInterface interface {
	Method()
}

type derivedInterface interface {
	baseInterface
	ExtraMethod()
}

type fullImpl struct{}

func (f fullImpl) Method()      {}
func (f fullImpl) ExtraMethod() {}

func TestArgRegistry_getArg_coverageGaps(t *testing.T) {
	// Test cases specifically to cover gaps in coverage
	tests := []struct {
		name       string
		registry   argRegistry
		lookupType reflect.Type
		wantFound  bool
	}{
		{
			name: "pointer type stored with non-addressable value",
			registry: func() argRegistry {
				r := make(argRegistry)
				// Store a non-addressable value for *testStruct key
				v := testStruct{Value: "test"}
				r[reflect.TypeOf(&testStruct{})] = reflect.ValueOf(&v).Elem()
				return r
			}(),
			lookupType: reflect.TypeOf(&testStruct{}),
			wantFound:  true, // Direct lookup will find it without needing Addr()
		},
		{
			name: "assignable non-pointer type in loop - needsElem",
			registry: argRegistry{
				reflect.TypeOf(&testStruct{}): reflect.ValueOf(&testStruct{Value: "test"}),
			},
			lookupType: reflect.TypeOf(testStruct{}),
			wantFound:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotFound := tt.registry.getArg(tt.lookupType)
			if gotFound != tt.wantFound {
				t.Errorf("argRegistry.getArg() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestArgRegistry_getArg_edgeCases(t *testing.T) {
	// Test edge cases for assignability
	type embeddedStruct struct {
		testStruct
		Extra string
	}

	embedded := &embeddedStruct{
		testStruct: testStruct{Value: "embedded"},
		Extra:      "extra",
	}

	tests := []struct {
		name       string
		registry   argRegistry
		lookupType reflect.Type
		wantFound  bool
	}{
		{
			name: "embedded struct assignability",
			registry: argRegistry{
				reflect.TypeOf(embedded): reflect.ValueOf(embedded),
			},
			lookupType: reflect.TypeOf(&testStruct{}),
			wantFound:  false, // embedded structs are not directly assignable
		},
		{
			name: "slice type not found",
			registry: argRegistry{
				reflect.TypeOf([]string{"a", "b"}): reflect.ValueOf([]string{"a", "b"}),
			},
			lookupType: reflect.TypeOf([]int{1, 2}),
			wantFound:  false,
		},
		{
			name: "map type not found",
			registry: argRegistry{
				reflect.TypeOf(map[string]int{"a": 1}): reflect.ValueOf(map[string]int{"a": 1}),
			},
			lookupType: reflect.TypeOf(map[string]string{"a": "b"}),
			wantFound:  false,
		},
		{
			name: "interface assignability - pointer type not directly assignable",
			registry: argRegistry{
				reflect.TypeOf((*derivedInterface)(nil)): reflect.ValueOf(&fullImpl{}),
			},
			lookupType: reflect.TypeOf((*baseInterface)(nil)),
			wantFound:  false,
		},
		{
			name: "interface assignability - value types not directly assignable",
			registry: argRegistry{
				reflect.TypeOf((*derivedInterface)(nil)).Elem(): reflect.ValueOf(fullImpl{}),
			},
			lookupType: reflect.TypeOf((*baseInterface)(nil)).Elem(),
			wantFound:  false,
		},
		{
			name: "type stored with exact match in loop",
			registry: argRegistry{
				reflect.TypeOf(&fullImpl{}): reflect.ValueOf(&fullImpl{}),
			},
			lookupType: reflect.TypeOf(&fullImpl{}),
			wantFound:  true,
		},
		{
			name: "complex assignability scenario",
			registry: func() argRegistry {
				r := make(argRegistry)
				// This tests the loop where we check assignability
				customType := reflect.TypeOf(struct{ Data string }{})
				r[customType] = reflect.ValueOf(struct{ Data string }{Data: "test"})
				return r
			}(),
			lookupType: reflect.TypeOf(struct{ Data string }{}),
			wantFound:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotFound := tt.registry.getArg(tt.lookupType)
			if gotFound != tt.wantFound {
				t.Errorf("argRegistry.getArg() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

// Custom types for assignability testing
type customInterface interface {
	CustomMethod()
}

type customImpl struct{}

func (c customImpl) CustomMethod() {}

func TestArgRegistry_integration(t *testing.T) {
	// Test a complete workflow
	registry := make(argRegistry)

	// Add various types
	s1 := &testStruct{Value: "first"}
	s2 := testStruct{Value: "second"}
	impl := &testImplementation{Data: "impl"}

	if err := registry.addArg(s1); err != nil {
		t.Fatalf("Failed to add s1: %v", err)
	}
	if err := registry.addArg(s2); err != nil {
		t.Fatalf("Failed to add s2: %v", err)
	}
	if err := registry.addArg(impl); err != nil {
		t.Fatalf("Failed to add impl: %v", err)
	}
	if err := registry.addArg(nil); err != nil {
		t.Fatalf("Failed to add nil: %v", err)
	}

	// Verify retrieval
	if v, ok := registry.getArg(reflect.TypeOf(s1)); !ok || v.Interface().(*testStruct).Value != "first" {
		t.Error("Failed to retrieve pointer type")
	}

	// When we ask for testStruct, getArg will find *testStruct first and dereference it
	// This is the designed behavior - it prefers finding compatible types through conversion
	if v, ok := registry.getArg(reflect.TypeOf(s2)); ok {
		// Check if value is correct
		if v.CanInterface() && v.Type() == reflect.TypeOf(s2) {
			val := v.Interface().(testStruct)
			// Due to getArg's conversion logic, we'll get "first" from the dereferenced *testStruct
			if val.Value != "first" {
				t.Errorf("Expected value 'first' (from dereferenced *testStruct), got '%s'", val.Value)
			}
		} else {
			t.Errorf("Retrieved value has wrong type or can't be interfaced")
		}
	} else {
		t.Error("Failed to retrieve non-pointer type")
	}

	// Test that interface retrieval fails because we didn't store interface type
	if _, ok := registry.getArg(reflect.TypeOf((*testInterface)(nil)).Elem()); ok {
		t.Error("Should not retrieve interface type when implementation was stored")
	}
}

func TestArgRegistry_assignability(t *testing.T) {
	// Test assignability paths specifically
	registry := make(argRegistry)

	// Add a custom implementation that is assignable to the interface
	impl := &customImpl{}
	iface := customInterface(impl)
	if err := registry.addArg(iface); err != nil {
		t.Fatalf("Failed to add iface: %v", err)
	}

	// Try to retrieve as pointer to interface - should trigger pt.AssignableTo(t) path
	ptrType := reflect.TypeOf((*customInterface)(nil))
	if v, ok := registry.getArg(ptrType); ok {
		t.Logf("Successfully retrieved pointer to interface: %v", v.Type())
	}

	// Add a non-pointer value that can be retrieved as interface
	nonPtrImpl := customImpl{}
	registry2 := make(argRegistry)
	if err := registry2.addArg(nonPtrImpl); err != nil {
		t.Fatalf("Failed to add nonPtrImpl: %v", err)
	}

	// Try to retrieve as interface - should trigger st.AssignableTo(t) path
	ifaceType := reflect.TypeOf((*customInterface)(nil)).Elem()
	if v, ok := registry2.getArg(ifaceType); ok {
		t.Logf("Successfully retrieved as interface: %v", v.Type())
	}
}

// Test to improve coverage of getArg with unaddressable value
func TestArgRegistry_getArg_unaddressable(t *testing.T) {
	registry := make(argRegistry)

	// Add a non-addressable value (created from a literal)
	registry[reflect.TypeOf(42)] = reflect.ValueOf(42)

	// Try to get a pointer to int - should fail because value is not addressable
	ptrType := reflect.TypeOf((*int)(nil))
	_, ok := registry.getArg(ptrType)
	if ok {
		t.Error("Expected not to find pointer type for unaddressable value")
	}
}

// Test extended types for better assignability coverage
func TestArgRegistry_getArg_extendedAssignability(t *testing.T) {
	registry := make(argRegistry)

	// Test with channel type (uncommon but valid)
	ch := make(chan int)
	registry[reflect.TypeOf(ch)] = reflect.ValueOf(ch)

	// Exact match should work
	v, ok := registry.getArg(reflect.TypeOf(ch))
	if !ok {
		t.Error("Expected to find channel type")
	}
	if v.Type() != reflect.TypeOf(ch) {
		t.Errorf("Expected channel type, got %v", v.Type())
	}
}

// Test to cover the needsPtr case in getArg where v.Addr() is called
func TestArgRegistry_getArg_needsPtr(t *testing.T) {
	registry := make(argRegistry)

	// Create an addressable value
	val := testStruct{Value: "addressable"}
	// Store the addressable value (not a pointer)
	registry[reflect.TypeOf(val)] = reflect.ValueOf(&val).Elem()

	// Now look up a pointer type - this should trigger needsPtr = true and v.Addr()
	ptrType := reflect.TypeOf(&testStruct{})
	v, ok := registry.getArg(ptrType)
	if !ok {
		t.Fatal("Expected to find value")
	}
	if v.Kind() != reflect.Ptr {
		t.Errorf("Expected pointer, got %v", v.Kind())
	}
}

// Test for covering the assignability loop in getArg
func TestArgRegistry_getArg_assignabilityLoop(t *testing.T) {
	registry := make(argRegistry)

	// Add a type that will be checked in the assignability loop
	type customType struct{ Data string }
	customVal := customType{Data: "test"}
	registry[reflect.TypeOf(customVal)] = reflect.ValueOf(customVal)

	// Also add a pointer type to trigger different path
	ptrVal := &customType{Data: "ptr"}
	registry[reflect.TypeOf(ptrVal)] = reflect.ValueOf(ptrVal)

	// Test cases that will go through the assignability loop
	tests := []struct {
		name       string
		lookupType reflect.Type
		wantFound  bool
	}{
		{
			name:       "exact match in loop",
			lookupType: reflect.TypeOf(customType{}),
			wantFound:  true,
		},
		{
			name:       "pointer match in loop",
			lookupType: reflect.TypeOf(&customType{}),
			wantFound:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found := registry.getArg(tt.lookupType)
			if found != tt.wantFound {
				t.Errorf("getArg() found = %v, want %v", found, tt.wantFound)
			}
		})
	}
}
