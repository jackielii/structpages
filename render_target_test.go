package structpages

import (
	"net/http"
	"net/http/httptest"
	"reflect"
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
func (selectionTestPage) Props(r *http.Request, sel *RenderTarget) (string, error) {
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
				"HX-Target":  "selection-test-todo-list",
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
	// Test that Selected() correctly distinguishes between methods on different types
	// even if they have the same name

	// Create RenderTarget for selectionTestPage.TodoList
	pageType := reflect.TypeOf(selectionTestPage{})
	method, _ := pageType.MethodByName("TodoList")
	sel := &RenderTarget{selectedMethod: method}

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
	sel := &RenderTarget{selectedMethod: method}

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
