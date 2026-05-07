package htmltemplate_test

import (
	"bytes"
	"context"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/htmltemplate"
)

var indexTmpl = template.Must(template.New("t").
	Funcs(htmltemplate.Funcs(nil)).
	Parse(`{{ urlFor "Product" }}|{{ urlFor "User" "id" 42 }}|{{ urlFor "/products" }}`))

type tpl struct{ name string }

func (p tpl) Render(ctx context.Context, w io.Writer) error {
	clone, err := indexTmpl.Clone()
	if err != nil {
		return err
	}
	clone.Funcs(htmltemplate.Funcs(ctx))
	return clone.ExecuteTemplate(w, p.name, nil)
}

type productPage struct{}
type userPage struct{}

type indexPage struct {
	Product productPage `route:"/products Product"`
	User    userPage    `route:"/users/{id} User"`
}

func (indexPage) Page() tpl { return tpl{name: "t"} }

func TestFuncs_NilCtx_ReturnsEmpty(t *testing.T) {
	tmpl := template.Must(template.New("nilctx").
		Funcs(htmltemplate.Funcs(nil)).
		Parse(`url={{ urlFor "X" }} id={{ idFor "Y" }} target={{ idTarget "Z" }}`))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatalf("execute with nil ctx: %v", err)
	}
	if !strings.Contains(buf.String(), "url= id= target=") {
		t.Errorf("expected empty values, got %q", buf.String())
	}
}

func TestFuncs_BoundCtx_ResolvesURLs(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, indexPage{}, "/", "Index",
		structpages.WithWarnEmptyRoute(func(*structpages.PageNode) {})); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rr.Code, rr.Body.String())
	}
	want := "/products|/users/42|/products"
	if got := strings.TrimSpace(rr.Body.String()); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
