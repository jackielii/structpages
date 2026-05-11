package structpages

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// subpathRoot is the page tree we'll mount under a subpath like "/admin".
type subpathRoot struct {
	home  subpathHome  `route:"GET / Home"`
	users subpathUsers `route:"GET /users Users"`
	user  subpathUser  `route:"GET /users/{id} User"`
}

type subpathHome struct{}

func (subpathHome) Page() component { return testComponent{content: "home"} }

type subpathUsers struct{}

func (subpathUsers) Page() component { return testComponent{content: "users"} }

type subpathUser struct{}

func (subpathUser) Page() component { return testComponent{content: "user"} }

// TestMountAtSubpath verifies that passing a non-"/" base route to Mount
// registers all routes (root + children) under that subpath and that
// URLFor returns subpath-prefixed URLs.
func TestMountAtSubpath(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, subpathRoot{}, "/admin", "Admin")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		wantCode int
		wantBody string
	}{
		{"root mounts at subpath", "/admin", http.StatusOK, "home"},
		{"child route inherits subpath", "/admin/users", http.StatusOK, "users"},
		{"param route inherits subpath", "/admin/users/42", http.StatusOK, "user"},
		{"non-subpath path is unmatched", "/users", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("path %q: status = %d, want %d", tt.path, rec.Code, tt.wantCode)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("path %q: body = %q, want %q", tt.path, rec.Body.String(), tt.wantBody)
			}
		})
	}

	t.Run("URLFor includes subpath", func(t *testing.T) {
		cases := []struct {
			name string
			page any
			args []any
			want string
		}{
			{"root page", subpathHome{}, nil, "/admin"},
			{"child page", subpathUsers{}, nil, "/admin/users"},
			{"child page with param", subpathUser{}, []any{"id", "42"}, "/admin/users/42"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				got, err := sp.URLFor(c.page, c.args...)
				if err != nil {
					t.Fatalf("URLFor(%T): %v", c.page, err)
				}
				if got != c.want {
					t.Errorf("URLFor(%T) = %q, want %q", c.page, got, c.want)
				}
			})
		}
	})
}

// TestMountUnderOuterMux explores what happens when the structpages mux is
// itself mounted under a path by an outer router. Two integration shapes:
//
//  1. Outer router uses http.StripPrefix: inner mux is mounted at "/" and
//     sees requests with the prefix stripped.
//  2. Outer router dispatches without stripping: inner mux is mounted at the
//     subpath and sees requests with the prefix intact.
func TestMountUnderOuterMux(t *testing.T) {
	t.Run("outer mux with StripPrefix, inner mounted at /, WithURLPrefix", func(t *testing.T) {
		inner := http.NewServeMux()
		sp, err := Mount(inner, subpathRoot{}, "/", "App",
			WithURLPrefix("/admin"))
		if err != nil {
			t.Fatalf("Mount failed: %v", err)
		}

		outer := http.NewServeMux()
		outer.Handle("/admin/", http.StripPrefix("/admin", inner))

		// Routing: outer strips "/admin", inner sees "/users".
		req := httptest.NewRequest(http.MethodGet, "/admin/users", http.NoBody)
		rec := httptest.NewRecorder()
		outer.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("routing through StripPrefix: status = %d, want %d", rec.Code, http.StatusOK)
		}
		if rec.Body.String() != "users" {
			t.Errorf("routing through StripPrefix: body = %q, want %q", rec.Body.String(), "users")
		}

		// URLFor: WithURLPrefix tells structpages about the externally
		// visible "/admin" prefix, so generated URLs include it.
		cases := []struct {
			name string
			page any
			args []any
			want string
		}{
			{"root page", subpathHome{}, nil, "/admin"},
			{"child page", subpathUsers{}, nil, "/admin/users"},
			{"param route", subpathUser{}, []any{"id", "42"}, "/admin/users/42"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				got, err := sp.URLFor(c.page, c.args...)
				if err != nil {
					t.Fatalf("URLFor(%T): %v", c.page, err)
				}
				if got != c.want {
					t.Errorf("URLFor(%T) = %q, want %q", c.page, got, c.want)
				}
			})
		}
	})

	t.Run("outer mux without StripPrefix, inner mounted at /admin", func(t *testing.T) {
		// To make routing work without stripping, the inner Mount must use
		// the same prefix as the outer dispatch.
		inner := http.NewServeMux()
		sp, err := Mount(inner, subpathRoot{}, "/admin", "App")
		if err != nil {
			t.Fatalf("Mount failed: %v", err)
		}

		outer := http.NewServeMux()
		outer.Handle("/admin/", inner)

		req := httptest.NewRequest(http.MethodGet, "/admin/users", http.NoBody)
		rec := httptest.NewRecorder()
		outer.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("routing without StripPrefix: status = %d, want %d", rec.Code, http.StatusOK)
		}
		if rec.Body.String() != "users" {
			t.Errorf("routing without StripPrefix: body = %q, want %q", rec.Body.String(), "users")
		}

		// URLFor returns the correct prefixed URL because Mount was told.
		got, err := sp.URLFor(subpathUsers{})
		if err != nil {
			t.Fatalf("URLFor: %v", err)
		}
		if got != "/admin/users" {
			t.Errorf("URLFor = %q, want %q", got, "/admin/users")
		}
	})
}
