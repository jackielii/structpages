package structpages

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type raUser struct{ name string }
type raTenant string

func raCurrentUser(r *http.Request) (*raUser, error) {
	name := r.Header.Get("X-User")
	if name == "" {
		return nil, errors.New("not signed in")
	}
	return &raUser{name: name}, nil
}

type raAccountPage struct{}

func (raAccountPage) Props(u *raUser) (string, error) { return u.name, nil }
func (raAccountPage) Page(name string, u *raUser) component {
	return testComponent{name + "/" + u.name}
}

type raRoot struct {
	Account raAccountPage `route:"/account Account"`
}

func raGet(path, user string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	if user != "" {
		req.Header.Set("X-User", user)
	}
	return req
}

func TestRequestArgs_fillPropsAndComponents(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := Mount(mux, raRoot{}, "/", "App", WithRequestArgs(raCurrentUser)); err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, raGet("/account", "ana")); body != "ana/ana" {
		t.Errorf("rendered %q, want the user in Props and Page", body)
	}
}

func TestRequestArgs_runOncePerRequest(t *testing.T) {
	calls := 0
	counted := func(r *http.Request) (*raUser, error) {
		calls++
		return raCurrentUser(r)
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, raRoot{}, "/", "App", WithRequestArgs(counted)); err != nil {
		t.Fatal(err)
	}
	serve(t, mux, raGet("/account", "ana"))
	if calls != 1 {
		t.Errorf("provider ran %d times for Props and Page, want 1", calls)
	}
	serve(t, mux, raGet("/account", "bo"))
	if calls != 2 {
		t.Errorf("provider ran %d times over two requests, want 2", calls)
	}
}

func TestRequestArgs_errorStopsTheRequest(t *testing.T) {
	var got error
	mux := http.NewServeMux()
	_, err := Mount(mux, raRoot{}, "/", "App", WithRequestArgs(raCurrentUser),
		WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
			got = err
			w.WriteHeader(http.StatusUnauthorized)
		}))
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := serve(t, mux, raGet("/account", "")); code != http.StatusUnauthorized {
		t.Errorf("status %d, want the error handler's 401", code)
	}
	if got == nil || !strings.Contains(got.Error(), "not signed in") {
		t.Errorf("error = %v, want the provider's error", got)
	}
}

// raSkipProvider redirects and stops the request, like Props can.
func raSkipProvider(r *http.Request) (*raUser, error) {
	return nil, ErrSkipPageRender
}

func TestRequestArgs_errorsAreSignalsLikeProps(t *testing.T) {
	mux := http.NewServeMux()
	var handled bool
	_, err := Mount(mux, raRoot{}, "/", "App", WithRequestArgs(raSkipProvider),
		WithErrorHandler(func(http.ResponseWriter, *http.Request, error) { handled = true }))
	if err != nil {
		t.Fatal(err)
	}
	serve(t, mux, raGet("/account", "ana"))
	if handled {
		t.Error("ErrSkipPageRender from a provider reached the error handler; want it to stop the request quietly")
	}
}

// raAdminSection's RequestArgs provide a tenant to its pages, using a
// WithArgs value, and shadow the mount-wide user provider.
type raAdminSection struct {
	Settings raSettingsPage `route:"/settings Settings"`
}

type raTenantSource struct{ tenant raTenant }

func (raAdminSection) RequestArgs(src *raTenantSource) []any {
	return []any{
		func(*http.Request) (raTenant, error) { return src.tenant, nil },
		func(*http.Request) (*raUser, error) { return &raUser{name: "admin"}, nil },
	}
}

type raSettingsPage struct{}

func (raSettingsPage) Page(t raTenant, u *raUser) component {
	return testComponent{fmt.Sprintf("%s/%s", t, u.name)}
}

func TestRequestArgs_pageMethodScopesToSubtreeAndShadows(t *testing.T) {
	type root struct {
		Account raAccountPage  `route:"/account Account"`
		Admin   raAdminSection `route:"/admin Admin"`
	}
	mux := http.NewServeMux()
	_, err := Mount(mux, root{}, "/", "App",
		WithArgs(&raTenantSource{tenant: "acme"}),
		WithRequestArgs(raCurrentUser))
	if err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, raGet("/admin/settings", "ana")); body != "acme/admin" {
		t.Errorf("admin settings rendered %q, want the section's tenant and user", body)
	}
	if _, body := serve(t, mux, raGet("/account", "ana")); body != "ana/ana" {
		t.Errorf("account rendered %q; the admin section's providers must not reach it", body)
	}
}

type raValuePage struct{}

func (raValuePage) Page(u raUser) component { return testComponent{u.name} }

func TestRequestArgs_pointerFillsValueParameter(t *testing.T) {
	type root struct {
		P raValuePage `route:"/v V"`
	}
	mux := http.NewServeMux()
	var got error
	_, err := Mount(mux, root{}, "/", "App",
		WithRequestArgs(func(r *http.Request) (*raUser, error) {
			if r.URL.Query().Has("nil") {
				return nil, nil
			}
			return &raUser{name: "val"}, nil
		}),
		WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
			got = err
			w.WriteHeader(http.StatusInternalServerError)
		}))
	if err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, raGet("/v", "")); body != "val" {
		t.Errorf("rendered %q", body)
	}
	serve(t, mux, raGet("/v?nil", ""))
	if got == nil || !strings.Contains(got.Error(), "is nil") {
		t.Errorf("error = %v, want a nil pointer reported for a value parameter", got)
	}
}

type raNamerPage struct{}

func (raNamerPage) Page(n namer) component { return testComponent{n.Name()} }

func TestRequestArgs_interfaceParameter(t *testing.T) {
	type root struct {
		P raNamerPage `route:"/n N"`
	}
	provider := func(*http.Request) (*pgStore, error) { return &pgStore{name: "per-request"}, nil }

	mux := http.NewServeMux()
	// A request arg comes before WithArgs, so the registered memCache
	// doesn't make namer ambiguous.
	if _, err := Mount(mux, root{}, "/", "App", WithRequestArgs(provider), WithArgs(&memCache{})); err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, raGet("/n", "")); body != "per-request" {
		t.Errorf("rendered %q", body)
	}

	other := func(*http.Request) (*memCache, error) { return &memCache{}, nil }
	_, err := Mount(http.NewServeMux(), root{}, "/", "App", WithRequestArgs(provider, other))
	if err == nil || !strings.Contains(err.Error(), "implemented by more than one request arg") {
		t.Errorf("Mount error = %v, want two providers implementing namer reported", err)
	}
}

type raNodePage struct{}

func (raNodePage) Page(s string) component { return testComponent{s} }

func TestRequestArgs_providerCanTakeThePageNode(t *testing.T) {
	type root struct {
		P raNodePage `route:"/node Node"`
	}
	mux := http.NewServeMux()
	_, err := Mount(mux, root{}, "/", "App",
		WithRequestArgs(func(_ *http.Request, pn *PageNode) (string, error) { return pn.Name, nil }))
	if err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, raGet("/node", "")); body != "P" {
		t.Errorf("rendered %q, want the page node's name", body)
	}
}

type raHandler struct{}

func (raHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request, u *raUser) error {
	_, err := fmt.Fprint(w, "handler:"+u.name)
	return err
}

func TestRequestArgs_extendedServeHTTP(t *testing.T) {
	type root struct {
		H raHandler `route:"/h H"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App", WithRequestArgs(raCurrentUser)); err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, raGet("/h", "ana")); body != "handler:ana" {
		t.Errorf("rendered %q", body)
	}
}

func TestRequestArgs_mountRejectsBadProviders(t *testing.T) {
	cases := map[string]struct {
		providers []any
		want      string
	}{
		"not a func":  {[]any{"x"}, "WithRequestArgs argument 1: got string"},
		"no error":    {[]any{func(*http.Request) *raUser { return nil }}, "want func(*http.Request) (T, error)"},
		"wrong param": {[]any{func(string) (*raUser, error) { return nil, nil }}, "want func(*http.Request)"},
		"nil func":    {[]any{(func(*http.Request) (*raUser, error))(nil)}, "WithRequestArgs argument 1"},
		"duplicate type": {
			[]any{raCurrentUser, raCurrentUser},
			"*structpages.raUser is already provided by WithRequestArgs argument 1",
		},
	}
	for name, c := range cases {
		_, err := Mount(http.NewServeMux(), raRoot{}, "/", "App", WithRequestArgs(c.providers...))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: Mount error = %v, want it to contain %q", name, err, c.want)
		}
	}
}

type raBadSection struct {
	Settings raSettingsPage `route:"/settings Settings"`
}

func (raBadSection) RequestArgs() []any { return []any{42} }

func TestRequestArgs_mountRejectsBadPageRequestArgs(t *testing.T) {
	type root struct {
		Bad raBadSection `route:"/bad Bad"`
	}
	_, err := Mount(http.NewServeMux(), root{}, "/", "App")
	if err == nil || !strings.Contains(err.Error(), "Bad.RequestArgs element 1: got int") {
		t.Errorf("Mount error = %v", err)
	}
}

// raComponentOnlyPage needs the user only in Page, so the provider runs
// for the component, not Props.
type raComponentOnlyPage struct{}

func (raComponentOnlyPage) Page(u *raUser) component { return testComponent{u.name} }

func TestRequestArgs_componentProviderErrorsBreakTheFlow(t *testing.T) {
	type root struct {
		P raComponentOnlyPage `route:"/c C"`
	}
	login := func(*http.Request) (*raUser, error) {
		return nil, RenderComponent(testComponent{"please sign in"})
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App", WithRequestArgs(login)); err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, raGet("/c", "")); body != "please sign in" {
		t.Errorf("rendered %q, want the provider's RenderComponent", body)
	}
}
