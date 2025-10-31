//lint:file-ignore U1000 Ignore unused code in test file
package structpages

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Test component for rendering
type genericTestComponent struct {
	content string
}

func (tc *genericTestComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte(tc.content))
	return err
}

// Minimal test page with handler for tests that just need to verify dependency injection
type testPageWithHandler struct{}

func (testPageWithHandler) Page() component {
	return &genericTestComponent{content: "test page"}
}

// Generic types for testing

// Basic generic type with single type parameter
type genericStore[T any] struct {
	data map[string]T
}

func newGenericStore[T any]() *genericStore[T] {
	return &genericStore[T]{
		data: make(map[string]T),
	}
}

func (s *genericStore[T]) Get(key string) (T, bool) {
	val, ok := s.data[key]
	return val, ok
}

func (s *genericStore[T]) Set(key string, val T) {
	s.data[key] = val
}

// Generic interface
type repository[T any] interface {
	FindByID(id string) (T, error)
	Save(item T) error
}

// Generic implementation
type memoryRepository[T any] struct {
	items map[string]T
}

func newMemoryRepository[T any]() *memoryRepository[T] {
	return &memoryRepository[T]{
		items: make(map[string]T),
	}
}

func (r *memoryRepository[T]) FindByID(id string) (T, error) {
	item, ok := r.items[id]
	if !ok {
		var zero T
		return zero, io.EOF
	}
	return item, nil
}

func (r *memoryRepository[T]) Save(item T) error {
	// In real implementation, you'd extract ID from item
	r.items["test"] = item
	return nil
}

// Generic type with constraints
type numeric interface {
	~int | ~int32 | ~int64 | ~float32 | ~float64
}

type calculator[T numeric] struct {
	name string
}

func (c *calculator[T]) Add(a, b T) T {
	return a + b
}

// Nested generic types
type result[T any] struct {
	value T
	err   error //nolint:unused
}

type asyncProcessor[T any] struct {
	results chan result[T]
}

func newAsyncProcessor[T any]() *asyncProcessor[T] {
	return &asyncProcessor[T]{
		results: make(chan result[T], 10),
	}
}

// Test structs using generics
type userModel struct {
	ID   string
	Name string
}

type productModel struct {
	ID    string
	Title string
	Price float64
}

// Page structs for testing generic injection
type genericTestPage struct {
	userList       `route:"/users Users"`
	productList    `route:"/products Products"`
	calculatorPage `route:"/calc Calculator"`
	nested         `route:"/nested Nested"`
}

type userList struct{}

func (u userList) Props(r *http.Request, store *genericStore[userModel]) ([]userModel, error) {
	// Simulate getting users
	store.Set("1", userModel{ID: "1", Name: "Alice"})
	store.Set("2", userModel{ID: "2", Name: "Bob"})

	users := make([]userModel, 0)
	for _, v := range store.data {
		users = append(users, v)
	}
	return users, nil
}

func (u userList) Page(users []userModel) component {
	return &genericTestComponent{content: "Users: " + formatUsers(users)}
}

func formatUsers(users []userModel) string {
	names := make([]string, len(users))
	for i, u := range users {
		names[i] = u.Name
	}
	return strings.Join(names, ", ")
}

type productList struct{}

func (p productList) Props(r *http.Request, repo *memoryRepository[productModel]) ([]productModel, error) {
	// Save a test product
	testProduct := productModel{ID: "1", Title: "Widget", Price: 9.99}
	if err := repo.Save(testProduct); err != nil {
		return nil, err
	}

	// Get the product back
	product, err := repo.FindByID("test")
	if err != nil {
		return nil, err
	}
	return []productModel{product}, nil
}

func (p productList) Page(products []productModel) component {
	return &genericTestComponent{content: "Products: " + formatProducts(products)}
}

func formatProducts(products []productModel) string {
	titles := make([]string, len(products))
	for i, p := range products {
		titles[i] = p.Title
	}
	return strings.Join(titles, ", ")
}

type calculatorPage struct{}

func (c calculatorPage) Props(r *http.Request, calc *calculator[float64]) (string, error) {
	result := calc.Add(10.5, 20.5)
	return formatFloat(result), nil
}

func (c calculatorPage) Page(result string) component {
	return &genericTestComponent{content: "Result: " + result}
}

func formatFloat(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	// Trim trailing zeros after decimal point
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

type nested struct{}

func (n nested) Props(r *http.Request, proc *asyncProcessor[string]) (string, error) {
	// Send a result
	proc.results <- result[string]{value: "async result", err: nil}

	// Receive it back
	select {
	case res := <-proc.results:
		if res.err != nil {
			return "", res.err
		}
		return res.value, nil
	default:
		return "no result", nil
	}
}

func (n nested) Page(result string) component {
	return &genericTestComponent{content: "Async: " + result}
}

// Tests for generic type injection

func TestGenerics_BasicInjection(t *testing.T) {
	// Create generic services
	userStore := newGenericStore[userModel]()
	productRepo := newMemoryRepository[productModel]()
	floatCalc := &calculator[float64]{name: "float-calc"}
	asyncProc := newAsyncProcessor[string]()

	// Create router with error handler for debugging and generic dependencies
	mux := http.NewServeMux()
	sp, err := Mount(mux, genericTestPage{}, "/", "Generic Test",
		WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
			t.Logf("Error handling request %s: %v", r.URL.Path, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}),
		WithArgs(
			userStore,
			productRepo, // Use concrete type
			floatCalc,
			asyncProc,
		),
	)
	if err != nil {
		t.Fatalf("Failed to mount pages: %v", err)
	}
	_ = sp

	// Test user list with generic store
	t.Run("generic store injection", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "Alice") || !strings.Contains(body, "Bob") {
			t.Errorf("Expected user names in response, got: %s", body)
		}
	})

	// Test product list with generic repository
	t.Run("generic repository injection", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "Widget") {
			t.Errorf("Expected product title in response, got: %s", body)
		}
	})

	// Test calculator with numeric constraint
	t.Run("generic with constraint injection", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/calc", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		body := rec.Body.String()
		// Expected result: 10.5 + 20.5 = 31
		if !strings.Contains(body, "31") {
			t.Errorf("Expected calculation result 31, got: %s", body)
		}
	})

	// Test nested generics
	t.Run("nested generic types injection", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/nested", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "async result") {
			t.Errorf("Expected async result in response, got: %s", body)
		}
	})
}

func TestGenerics_DuplicateTypeError(t *testing.T) {
	// Test that duplicate generic types with same type parameters cause error
	store1 := newGenericStore[string]()
	store2 := newGenericStore[string]()

	_, err := Mount(nil, genericTestPage{}, "/", "Test",
		WithArgs(
			store1,
			store2, // Same type as store1
		),
	)

	if err == nil {
		t.Error("Expected error for duplicate generic type, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate type") {
		t.Errorf("Expected duplicate type error, got: %v", err)
	}
}

func TestGenerics_DifferentTypeParameters(t *testing.T) {
	// Test that same generic type with different type parameters are treated as different types
	stringStore := newGenericStore[string]()
	intStore := newGenericStore[int]()
	floatStore := newGenericStore[float64]()

	// Create a minimal page struct for testing
	testPage := testPageWithHandler{}

	// This should work because they're different types
	mux := http.NewServeMux()
	_, err := Mount(mux, testPage, "/", "Test",
		WithArgs(
			stringStore,
			intStore,
			floatStore,
		),
	)
	if err != nil {
		t.Errorf("Different type parameters should create different types, got error: %v", err)
	}
}

// Test generic slices and maps
type collectionPage struct{}

func (c collectionPage) Props(r *http.Request,
	intSlice []int,
	stringMap map[string]string,
	genericSlice []genericStore[string],
) (string, error) {
	// Just verify we received the dependencies
	return fmt.Sprintf("intSlice: %d items, stringMap: %d items, genericSlice: %d items",
		len(intSlice), len(stringMap), len(genericSlice)), nil
}

func (c collectionPage) Page(info string) component {
	return &genericTestComponent{content: info}
}

func TestGenerics_SlicesAndMaps(t *testing.T) {
	intSlice := []int{1, 2, 3}
	stringMap := map[string]string{"key": "value"}
	genericSlice := []genericStore[string]{
		{data: map[string]string{"a": "1"}},
		{data: map[string]string{"b": "2"}},
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, collectionPage{}, "/", "Test",
		WithArgs(
			intSlice,
			stringMap,
			genericSlice,
		),
	)
	if err != nil {
		t.Fatalf("Failed to mount pages: %v", err)
	}
	_ = sp

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	expected := "intSlice: 3 items, stringMap: 1 items, genericSlice: 2 items"
	if !strings.Contains(rec.Body.String(), expected) {
		t.Errorf("Expected %q, got %q", expected, rec.Body.String())
	}
}

// Test type alias with generics
type stringStore = genericStore[string]

func TestGenerics_TypeAlias(t *testing.T) {
	// Type aliases should work the same as the original type
	var store stringStore
	store.data = make(map[string]string)
	store.Set("key", "value")

	// Create a minimal page struct for testing
	testPage := testPageWithHandler{}

	// This should work
	mux1 := http.NewServeMux()
	_, err := Mount(mux1, testPage, "/", "Test", WithArgs(&store))
	if err != nil {
		t.Errorf("Type alias should work, got error: %v", err)
	}

	// But adding the original type with same parameters should fail
	originalStore := newGenericStore[string]()
	mux2 := http.NewServeMux()
	_, err = Mount(mux2, testPage, "/", "Test", WithArgs(&store, originalStore))
	if err == nil {
		t.Error("Expected error for duplicate type (alias and original), got nil")
	}
}

// Test reflection with generic types
func TestGenerics_Reflection(t *testing.T) {
	// Test that we can properly reflect on generic types
	stringStore := newGenericStore[string]()
	intStore := newGenericStore[int]()

	stringType := reflect.TypeOf(stringStore)
	intType := reflect.TypeOf(intStore)

	if stringType == intType {
		t.Error("Generic types with different type parameters should have different reflect.Type")
	}

	// Verify the type names are different
	if stringType.String() == intType.String() {
		t.Errorf("Expected different type strings, both got: %s", stringType.String())
	}
}

// Test generic function types
type funcStore[T any] struct {
	transformer func(T) T
}

type funcPage struct{}

func (f funcPage) Props(r *http.Request, fs *funcStore[string]) (string, error) {
	if fs.transformer == nil {
		return "no transformer", nil
	}
	return fs.transformer("hello"), nil
}

func (f funcPage) Page(result string) component {
	return &genericTestComponent{content: result}
}

func TestGenerics_FunctionTypes(t *testing.T) {
	fs := &funcStore[string]{
		transformer: strings.ToUpper,
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, funcPage{}, "/", "Test", WithArgs(fs))
	if err != nil {
		t.Fatalf("Failed to mount pages: %v", err)
	}
	_ = sp

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "HELLO") {
		t.Errorf("Expected uppercase HELLO, got: %s", rec.Body.String())
	}
}

// Test error handling with nil generic types
func TestGenerics_NilHandling(t *testing.T) {
	var nilStore *genericStore[string]

	// Create a minimal page struct for testing
	testPage := testPageWithHandler{}

	// Nil values should be ignored by argRegistry
	mux := http.NewServeMux()
	_, err := Mount(mux, testPage, "/", "Test", WithArgs(nilStore))
	if err != nil {
		t.Errorf("Nil generic values should be ignored, got error: %v", err)
	}
}

// Test complex generic constraint
type ordered interface {
	~int | ~float64 | ~string
}

type sortableStore[T ordered] struct {
	items []T
}

func (s *sortableStore[T]) Len() int {
	return len(s.items)
}

func TestGenerics_ComplexConstraints(t *testing.T) {
	// Test with different ordered types
	intSorter := &sortableStore[int]{items: []int{3, 1, 2}}
	stringSorter := &sortableStore[string]{items: []string{"c", "a", "b"}}
	floatSorter := &sortableStore[float64]{items: []float64{3.14, 1.0, 2.5}}

	// Create a minimal page struct for testing
	testPage := testPageWithHandler{}

	// All three should be treated as different types
	mux := http.NewServeMux()
	_, err := Mount(mux, testPage, "/", "Test",
		WithArgs(
			intSorter,
			stringSorter,
			floatSorter,
		),
	)
	if err != nil {
		t.Errorf("Different constrained types should work, got error: %v", err)
	}
}

// Test pointer vs non-pointer generic types
func TestGenerics_PointerSemantics(t *testing.T) {
	store := newGenericStore[string]()                                // *genericStore[string]
	valueStore := genericStore[string]{data: make(map[string]string)} // genericStore[string]

	// Create a minimal page struct for testing
	testPage := testPageWithHandler{}

	// Both pointer and value should be allowed, but they're different types
	mux := http.NewServeMux()
	_, err := Mount(mux, testPage, "/", "Test",
		WithArgs(
			store,
			valueStore,
		),
	)
	if err != nil {
		t.Errorf("Pointer and value of generic type should both work, got error: %v", err)
	}
}

// Test method matching with generics
type methodStore[T any] struct{}

func (m *methodStore[T]) Get() T {
	var zero T
	return zero
}

type genericMethodPage struct{}

func (m genericMethodPage) Props(r *http.Request, ms *methodStore[int]) (int, error) {
	return ms.Get(), nil
}

func (m genericMethodPage) Page(val int) component {
	return &genericTestComponent{content: fmt.Sprintf("Value: %d", val)}
}

func TestGenerics_MethodMatching(t *testing.T) {
	ms := &methodStore[int]{}

	mux := http.NewServeMux()
	sp, err := Mount(mux, genericMethodPage{}, "/", "Test", WithArgs(ms))
	if err != nil {
		t.Fatalf("Failed to mount pages: %v", err)
	}
	_ = sp

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Value: 0") {
		t.Errorf("Expected zero value, got: %s", rec.Body.String())
	}
}

// Test interface injection limitations with generics
func TestGenerics_InterfaceInjection(t *testing.T) {
	// This test documents a known limitation: when you store an interface type
	// in the argRegistry, Go's reflection shows the concrete type, not the interface.
	// This means Props methods need to accept the concrete type, not the interface.

	t.Run("interface stored as concrete type", func(t *testing.T) {
		var repo repository[string] = &memoryRepository[string]{
			items: make(map[string]string),
		}

		// Even though repo is declared as an interface, reflect.TypeOf shows concrete type
		concreteType := reflect.TypeOf(repo).String()
		if !strings.Contains(concreteType, "memoryRepository") {
			t.Errorf("Expected concrete type memoryRepository, got %s", concreteType)
		}

		// This is why interface injection doesn't work as expected with generics
		t.Log("Interface variables show concrete type in reflection:", concreteType)
	})

	// Workaround: use concrete types in Props methods
	t.Run("workaround using concrete types", func(t *testing.T) {
		// Instead of accepting repository[T], accept *memoryRepository[T]
		// This is what we did in the productList.Props method
		t.Log("Workaround: Props methods should accept concrete types, not interface types")
	})
}
