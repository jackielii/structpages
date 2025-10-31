package structpages

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Test page types for RenderTarget
type selectionTestPage struct{}

func (selectionTestPage) Page(data string) component {
	return testComponent{content: "Page: " + data}
}

func (selectionTestPage) TodoList() component {
	return testComponent{content: "TodoList"}
}

func (selectionTestPage) Content(data string) component {
	return testComponent{content: "Content: " + data}
}

// Props that uses RenderTarget to return different data
func (selectionTestPage) Props(r *http.Request, sel RenderTarget) (string, error) {
	switch {
	case sel.Is(selectionTestPage.TodoList):
		return "todo data", nil
	case sel.Is(selectionTestPage.Content):
		return "content data", nil
	case sel.Is(selectionTestPage.Page):
		return "page data", nil
	default:
		return "default data", nil
	}
}

// Different page type with same method name (to test receiver type checking)
type anotherPage struct{}

func (anotherPage) TodoList() component {
	return testComponent{content: "Different TodoList"}
}

func TestRenderTarget_Selected(t *testing.T) {
	type pages struct {
		selectionTestPage `route:"/ SelectionTest"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	tests := []struct {
		name         string
		headers      map[string]string
		expectedBody string
		expectedData string // What Props should return based on selection
	}{
		{
			name:         "Non-HTMX request selects Page",
			headers:      map[string]string{},
			expectedBody: "Page: page data",
			expectedData: "page data",
		},
		{
			name: "HTMX request with simple target selects Content",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "content",
			},
			expectedBody: "Content: content data",
			expectedData: "content data",
		},
		{
			name: "HTMX request with IDFor-style target selects TodoList",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "selection-test-page-todo-list",
			},
			expectedBody: "TodoList",
			expectedData: "todo data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rec.Code)
			}

			body := rec.Body.String()
			if body != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, body)
			}

			// The body contains the data that Props returned based on RenderTarget
			// This proves that Props correctly used RenderTarget.Is()
		})
	}
}

func TestRenderTarget_DifferentReceiverTypes(t *testing.T) {
	// Test that Is() correctly distinguishes between methods on different types
	// even if they have the same name

	// Create RenderTarget for selectionTestPage.TodoList
	pageType := reflect.TypeOf(selectionTestPage{})
	method, _ := pageType.MethodByName("TodoList")
	sel := newMethodRenderTarget("TodoList", &method)

	// Should match selectionTestPage.TodoList
	if !sel.Is(selectionTestPage.TodoList) {
		t.Error("Should match selectionTestPage.TodoList")
	}

	// Should NOT match anotherPage.TodoList (different receiver type)
	if sel.Is(anotherPage.TodoList) {
		t.Error("Should NOT match anotherPage.TodoList (different type)")
	}
}

func TestRenderTarget_InvalidMethodExpression(t *testing.T) {
	pageType := reflect.TypeOf(selectionTestPage{})
	method, _ := pageType.MethodByName("Page")
	sel := newMethodRenderTarget("Page", &method)

	// Test with invalid inputs
	if sel.Is("not a method") {
		t.Error("Should not match non-method")
	}

	if sel.Is(nil) {
		t.Error("Should not match nil")
	}

	if sel.Is(123) {
		t.Error("Should not match non-function")
	}
}

// Standalone function components for testing
func StandaloneWidget(data string) component {
	return testComponent{content: "StandaloneWidget: " + data}
}

func AnotherStandaloneWidget() component {
	return testComponent{content: "AnotherStandaloneWidget"}
}

// Page that uses standalone function components
type standaloneTestPage struct{}

func (standaloneTestPage) Page(data string) component {
	return testComponent{content: "Page: " + data}
}

func (p standaloneTestPage) Props(r *http.Request, target RenderTarget) (string, error) {
	switch {
	case target.Is(AnotherStandaloneWidget):
		return "", RenderComponent(target)
	case target.Is(StandaloneWidget):
		return "standalone data", RenderComponent(target, "standalone data")
	case target.Is(p.Page):
		return "page data", nil
	default:
		return "default data", nil
	}
}

func TestRenderTarget_StandaloneFunctions(t *testing.T) {
	type pages struct {
		standaloneTestPage `route:"/ StandaloneTest"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	tests := []struct {
		name         string
		headers      map[string]string
		expectedBody string
	}{
		{
			name: "HTMX request targeting StandaloneWidget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "standalone-widget",
			},
			expectedBody: "StandaloneWidget: standalone data",
		},
		{
			name: "HTMX request targeting StandaloneWidget with page prefix",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "#standalone-test-page-standalone-widget",
			},
			expectedBody: "StandaloneWidget: standalone data",
		},
		{
			name: "HTMX request targeting AnotherStandaloneWidget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "another-standalone-widget",
			},
			expectedBody: "AnotherStandaloneWidget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
			}

			body := rec.Body.String()
			if body != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, body)
			}
		})
	}
}

func TestFunctionRenderTarget_Is(t *testing.T) {
	// Test functionRenderTarget.Is() directly
	frt := &functionRenderTarget{
		hxTarget: "#standalone-test-page-standalone-widget",
		pageName: "StandaloneTestPage",
	}

	// Should match StandaloneWidget
	if !frt.Is(StandaloneWidget) {
		t.Error("Should match StandaloneWidget")
	}

	// funcValue should be set after match
	if !frt.funcValue.IsValid() {
		t.Error("funcValue should be set after Is() returns true")
	}

	// Should NOT match AnotherStandaloneWidget
	frt2 := &functionRenderTarget{
		hxTarget: "standalone-widget",
		pageName: "StandaloneTestPage",
	}
	if !frt2.Is(StandaloneWidget) {
		t.Error("Should match StandaloneWidget without # prefix")
	}

	// Should NOT match page methods
	frt3 := &functionRenderTarget{
		hxTarget: "page",
		pageName: "StandaloneTestPage",
	}
	if frt3.Is(standaloneTestPage.Page) {
		t.Error("Should NOT match page method")
	}

	// Test with invalid input
	if frt3.Is("not a function") {
		t.Error("Should not match non-function")
	}
}

// Test methodRenderTarget.Is() with edge cases
func TestMethodRenderTarget_Is_EdgeCases(t *testing.T) {
	// Test with zero method (Type == nil)
	zeroMethod := reflect.Method{}
	mrt := &methodRenderTarget{
		name:   "Test",
		method: zeroMethod,
	}

	if mrt.Is(selectionTestPage.Page) {
		t.Error("Should not match when method.Type is nil")
	}

	// Test with bound method where selectedReceiverType is nil
	// Create a method with no inputs (NumIn() == 0)
	noInputMethod := reflect.Method{
		Name: "Test",
		Type: reflect.TypeOf(func() {}),
	}

	mrt2 := &methodRenderTarget{
		name:   "Test",
		method: noInputMethod,
	}

	// Create a bound method info (this would normally come from extractMethodInfo)
	// We need to test the path where info.isBound is true but selectedReceiverType is nil
	// This happens when the method has no receiver (NumIn == 0)

	// This will match the path where selectedReceiverType == nil for bound methods
	// The function should return false
	if mrt2.Is(selectionTestPage.Page) {
		t.Error("Should not match when bound method has no receiver type")
	}

	// Test that standalone functions don't match against method targets
	if mrt2.Is(StandaloneWidget) {
		t.Error("Should not match standalone function against method target")
	}
}

// Type with pointer receiver method for testing Is()
type ptrReceiverTestPage struct{}

func (*ptrReceiverTestPage) PointerMethod() component {
	return testComponent{content: "pointer method"}
}

// Test methodRenderTarget.Is() with actual bound methods
func TestMethodRenderTarget_Is_BoundMethods(t *testing.T) {
	// Create a methodRenderTarget for selectionTestPage.TodoList
	pageType := reflect.TypeOf(selectionTestPage{})
	method, _ := pageType.MethodByName("TodoList")
	mrt := &methodRenderTarget{
		name:   "TodoList",
		method: method,
	}

	// Create an actual bound method (instance.Method)
	instance := selectionTestPage{}
	boundMethod := instance.TodoList

	// Test with bound method from same type - should match
	if !mrt.Is(boundMethod) {
		t.Error("Should match bound method from same type")
	}

	// Test with bound method from different type - should not match
	anotherInstance := anotherPage{}
	anotherBoundMethod := anotherInstance.TodoList
	if mrt.Is(anotherBoundMethod) {
		t.Error("Should NOT match bound method from different type")
	}
}

// Test methodRenderTarget.Is() with pointer receiver methods (unbound)
func TestMethodRenderTarget_Is_PointerReceiverUnbound(t *testing.T) {
	// Create a methodRenderTarget for a pointer receiver method
	pageType := reflect.TypeOf(&ptrReceiverTestPage{})
	method, ok := pageType.MethodByName("PointerMethod")
	if !ok {
		t.Fatal("PointerMethod not found")
	}

	mrt := &methodRenderTarget{
		name:   "PointerMethod",
		method: method,
	}

	// Test with unbound pointer receiver method expression
	// This should trigger the pointer unwrapping path (lines 118-120)
	unboundPointerMethod := (*ptrReceiverTestPage).PointerMethod
	if !mrt.Is(unboundPointerMethod) {
		t.Error("Should match unbound pointer receiver method")
	}
}

// Page with Props that doesn't handle function targets
type forgotRenderComponentPage struct{}

func (forgotRenderComponentPage) Page(data string) component {
	return testComponent{content: "Page: " + data}
}

func (p forgotRenderComponentPage) Props(r *http.Request, target RenderTarget) (string, error) {
	// Intentionally doesn't call RenderComponent for function targets
	// This should trigger the error on line 348
	return "data", nil
}

// Test error when Props doesn't use RenderComponent for function target
func TestRenderTarget_PropsForgetRenderComponent(t *testing.T) {
	type pages struct {
		forgotRenderComponentPage `route:"/ ForgotPage"`
	}

	mux := http.NewServeMux()
	var capturedErr error
	sp, err := Mount(mux, &pages{}, "/", "Test", WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	// Make HTMX request targeting a standalone function
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "standalone-widget") // This will create a functionRenderTarget

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Should get error because Props didn't call RenderComponent
	if capturedErr == nil {
		t.Error("Expected error when Props doesn't use RenderComponent for function target")
	}
	// Verify new improved error message
	if capturedErr != nil {
		errMsg := capturedErr.Error()
		if !strings.Contains(errMsg, "Component function 'StandaloneWidget' is targeted but not handled") {
			t.Errorf("Expected improved error message about StandaloneWidget, got: %v", errMsg)
		}
		if !strings.Contains(errMsg, "target.Is(StandaloneWidget)") {
			t.Errorf("Expected error to mention target.Is(StandaloneWidget), got: %v", errMsg)
		}
	}
}

// Page that uses RenderComponent with method target
type methodTargetPage struct{}

func (methodTargetPage) Page(data string) component {
	return testComponent{content: "Page: " + data}
}

func (methodTargetPage) CustomComponent(data string) component {
	return testComponent{content: "Custom: " + data}
}

func (p methodTargetPage) Props(r *http.Request, target RenderTarget) (string, error) {
	// Check if target is CustomComponent and use RenderComponent with it
	if target.Is(methodTargetPage.CustomComponent) {
		// This will hit renderOpFromTarget with methodRenderTarget
		return "", RenderComponent(target, "method target data")
	}
	return "page data", nil
}

// Test RenderComponent with method target from Props
func TestRenderComponent_MethodTargetFromProps(t *testing.T) {
	type pages struct {
		methodTargetPage `route:"/ MethodTargetPage"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	// Make HTMX request targeting CustomComponent
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "custom-component")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if body != "Custom: method target data" {
		t.Errorf("Expected 'Custom: method target data', got %q", body)
	}
}

// Debug test to understand pointer unwrapping
type debugPtrReceiverPage struct{}

func (*debugPtrReceiverPage) PtrMethod() component {
	return testComponent{content: "test"}
}

func TestDebugPointerUnwrapping(t *testing.T) {
	// Create methodRenderTarget
	pageType := reflect.TypeOf(&debugPtrReceiverPage{})
	method, _ := pageType.MethodByName("PtrMethod")

	t.Logf("Method.Type.NumIn(): %d", method.Type.NumIn())
	if method.Type.NumIn() > 0 {
		t.Logf("Method.Type.In(0): %v (Kind: %v)", method.Type.In(0), method.Type.In(0).Kind())
	}

	// Create unbound method expression
	unboundMethod := (*debugPtrReceiverPage).PtrMethod

	info, err := extractMethodInfo(unboundMethod)
	if err != nil {
		t.Fatalf("extractMethodInfo failed: %v", err)
	}

	t.Logf("MethodInfo:")
	t.Logf("  isBound: %v", info.isBound)
	t.Logf("  receiverType: %v", info.receiverType)
	if info.receiverType != nil {
		t.Logf("  receiverType.Kind(): %v (Pointer=%v)",
			info.receiverType.Kind(), info.receiverType.Kind() == reflect.Pointer)
	}
}
