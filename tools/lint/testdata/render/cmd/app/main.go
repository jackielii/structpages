package main

import (
	"net/http"

	"ex/pages"
	"ex/plain"
	"ex/tpl"

	"github.com/jackielii/structpages"
)

func appOptions() structpages.Option {
	return structpages.WithTargetSelector(structpages.HTMXRenderTarget)
}

// serve mounts whatever tree it is handed.
func serve(mux *http.ServeMux, page any) {
	structpages.Mount(mux, page, "/helper", "Helper") // want: via serve(mux, pages.Root{})
}

func main() {
	mux := http.NewServeMux()
	opts := []structpages.Option{appOptions()}

	structpages.Mount(mux, pages.Root{}, "/", "App") // want
	structpages.Mount(mux, pages.Root{}, "/v2", "App",
		structpages.WithTargetSelector(structpages.HTMXRenderTarget))
	structpages.Mount(mux, pages.Root{}, "/v4", "App",
		structpages.WithErrorHandler(nil),
		structpages.WithTargetSelector(structpages.HTMXv4RenderTarget))
	structpages.Mount(mux, pages.Root{}, "/mw", "App", structpages.WithMiddlewares()) // want
	structpages.Mount(mux, pages.Root{}, "/lit", "App",                               // want: literal spread
		[]structpages.Option{structpages.WithMiddlewares()}...)
	structpages.Mount(mux, pages.Root{}, "/var", "App", opts...)     // unresolvable: not flagged
	structpages.Mount(mux, pages.Root{}, "/fn", "App", appOptions()) // unresolvable: not flagged
	//structpages:lint:ignore selector
	structpages.Mount(mux, pages.Root{}, "/quiet", "App")
	structpages.Mount(mux, plain.Root{}, "/plain", "Plain") // no htmx
	structpages.Mount(mux, tpl.Root{}, "/tpl", "Tpl")       // want: templ htmx 4
	serve(mux, pages.Root{})
	structpages.Mount(mux, pages.Root{}, "/nil", "App", structpages.WithTargetSelector(nil)) // want: nil is ignored
}
