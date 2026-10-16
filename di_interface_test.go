package structpages

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// namer is a consumer-side interface: a Props method declares only what it
// needs, so a unit test can call it with a fake.
type namer interface{ Name() string }

type pgStore struct{ name string }

func (s *pgStore) Name() string { return s.name }

type memCache struct{}

func (*memCache) Name() string { return "cache" }

type namerPage struct{}

func (namerPage) Props(n namer) (string, error) { return n.Name(), nil }
func (namerPage) Page(name string) component    { return testComponent{name} }

type namerRoot struct {
	Namer namerPage `route:"/namer Namer"`
}

func TestInjection_interfaceParamTakesTheOneImplementation(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := Mount(mux, namerRoot{}, "/", "App", WithArgs(&pgStore{name: "pg"})); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/namer", http.NoBody))
	if rec.Code != http.StatusOK || rec.Body.String() != "pg" {
		t.Errorf("GET /namer = %d %q, want 200 %q", rec.Code, rec.Body.String(), "pg")
	}
}

func TestInjection_twoImplementationsFailMount(t *testing.T) {
	_, err := Mount(http.NewServeMux(), namerRoot{}, "/", "App",
		WithArgs(&pgStore{name: "pg"}, &memCache{}))
	if err == nil {
		t.Fatal("Mount succeeded; want an ambiguity error")
	}
	for _, want := range []string{"page Namer", "namer", "*structpages.memCache, *structpages.pgStore"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Mount error = %v, want it to contain %q", err, want)
		}
	}
}

// concreteNamerPage asks for the concrete type, which is registered
// exactly, so a second implementation of namer doesn't make it ambiguous.
type concreteNamerPage struct{}

func (concreteNamerPage) Props(s *pgStore) (string, error) { return s.Name(), nil }
func (concreteNamerPage) Page(name string) component       { return testComponent{name} }

func TestInjection_exactRegistrationWins(t *testing.T) {
	type root struct {
		Concrete concreteNamerPage `route:"/concrete Concrete"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App", WithArgs(&pgStore{name: "pg"}, &memCache{})); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/concrete", http.NoBody))
	if rec.Body.String() != "pg" {
		t.Errorf("GET /concrete = %q, want %q", rec.Body.String(), "pg")
	}
}

// TestInjection_pointerToInterfaceIsUsedFirst pins that registering a
// pointer to an interface variable fills that interface, ahead of (and
// without ambiguity against) the registered implementations.
func TestInjection_pointerToInterfaceIsUsedFirst(t *testing.T) {
	var n namer = &pgStore{name: "chosen"}
	mux := http.NewServeMux()
	_, err := Mount(mux, namerRoot{}, "/", "App",
		WithArgs(&n, &pgStore{name: "pg"}, &memCache{}))
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/namer", http.NoBody))
	if rec.Body.String() != "chosen" {
		t.Errorf("GET /namer = %q, want %q", rec.Body.String(), "chosen")
	}
}

// writerPropsPage asks for an io.Writer, which the per-request
// http.ResponseWriter fills before any registration is considered.
type writerPropsPage struct{}

func (writerPropsPage) Props(out io.Writer) (string, error) {
	_, isResponse := out.(http.ResponseWriter)
	return fmt.Sprint(isResponse), nil
}
func (writerPropsPage) Page(got string) component { return testComponent{got} }

func TestInjection_requestValueWinsOverRegistrations(t *testing.T) {
	type root struct {
		Writer writerPropsPage `route:"/w Writer"`
	}
	mux := http.NewServeMux()
	// Two registered io.Writers would be ambiguous, but the request's
	// writer fills the parameter first.
	_, err := Mount(mux, root{}, "/", "App", WithArgs(&bytes.Buffer{}, &strings.Builder{}))
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/w", http.NoBody))
	if rec.Body.String() != "true" {
		t.Errorf("GET /w: Props got the response writer = %q, want %q", rec.Body.String(), "true")
	}
}

// writerHandler's own w already takes the request's writer, so its extra
// io.Writer comes from the registry, where two candidates are ambiguous.
// Mount must reject it, as the request would fail.
type writerHandler struct{}

func (writerHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request, out io.Writer) error {
	_, err := io.WriteString(out, "")
	return err
}

func TestInjection_eachRequestValueFillsOneParameter(t *testing.T) {
	type root struct {
		Handler writerHandler `route:"/h Handler"`
	}
	_, err := Mount(http.NewServeMux(), root{}, "/", "App", WithArgs(&bytes.Buffer{}, &strings.Builder{}))
	if err == nil || !strings.Contains(err.Error(), "io.Writer is implemented by more than one registered value") {
		t.Errorf("Mount error = %v, want io.Writer ambiguity", err)
	}
}

// With nothing registered, namerPage's namer is missing. That stays a
// request-time error: a page may be mounted in a configuration that doesn't
// use it.
func TestInjection_missingDependencyFailsTheRequestNotMount(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := Mount(mux, namerRoot{}, "/", "App"); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/namer", http.NoBody))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("GET /namer = %d, want 500", rec.Code)
	}
}

// TestInjection_parseDoesNotCheck pins that Parse, used by tests that only
// need URLFor/ID, doesn't require Props dependencies to be registered.
func TestInjection_parseDoesNotCheck(t *testing.T) {
	sp, err := Parse(namerRoot{}, "/", "App")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := URLFor(sp.PageContext(context.Background()), namerPage{}); err != nil {
		t.Errorf("URLFor: %v", err)
	}
}
