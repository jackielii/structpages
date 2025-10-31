package structpages

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// ============================================================================
// 1. PARSING BENCHMARKS
// ============================================================================

func BenchmarkParsing(b *testing.B) {
	b.Run("parseTag_Simple", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, _ = parseTag("/product")
		}
	})

	b.Run("parseTag_WithMethod", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, _ = parseTag("POST /api/users")
		}
	})

	b.Run("parseTag_WithMethodAndTitle", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, _ = parseTag("POST /api/users Create User")
		}
	})

	b.Run("parseSegments_NoParams", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = parseSegments("/api/v1/users")
		}
	})

	b.Run("parseSegments_OneParam", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = parseSegments("/api/users/{id}")
		}
	})

	b.Run("parseSegments_MultipleParams", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = parseSegments("/api/users/{userId}/posts/{postId}/comments/{commentId}")
		}
	})

	b.Run("parseSegments_Wildcard", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = parseSegments("/files/{path...}")
		}
	})

	b.Run("parsePageTree_Simple", func(b *testing.B) {
		type page struct{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = parsePageTree("/", "Test", page{})
		}
	})

	b.Run("parsePageTree_Medium", func(b *testing.B) {
		type product struct{}
		type team struct{}
		type contact struct{}
		type about struct{}
		type index struct {
			product `route:"/product Product"`
			team    `route:"/team Team"`
			contact `route:"/contact Contact"`
			about   `route:"/about About"`
		}
		p := index{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = parsePageTree("/", "Index", p)
		}
	})

	b.Run("parsePageTree_Complex", func(b *testing.B) {
		type user struct{}
		type userList struct{}
		type post struct{}
		type comment struct{}
		type admin struct {
			user     `route:"/users/{id} User"`
			userList `route:"/users Users"`
		}
		type blog struct {
			post    `route:"/posts/{id} Post"`
			comment `route:"/posts/{postId}/comments/{id} Comment"`
		}
		type index struct {
			admin `route:"/admin Admin"`
			blog  `route:"/blog Blog"`
		}
		p := index{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = parsePageTree("/", "Index", p)
		}
	})
}

// ============================================================================
// 2. MOUNT BENCHMARKS
// ============================================================================

func BenchmarkMount(b *testing.B) {
	b.Run("Mount_Simple_3Routes", func(b *testing.B) {
		type product struct{}
		type team struct{}
		type index struct {
			product `route:"/product Product"`
			team    `route:"/team Team"`
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			mux := http.NewServeMux()
			_, _ = Mount(mux, index{}, "/", "Index")
		}
	})

	b.Run("Mount_Medium_20Routes", func(b *testing.B) {
		type p1 struct{}
		type p2 struct{}
		type p3 struct{}
		type p4 struct{}
		type p5 struct{}
		type p6 struct{}
		type p7 struct{}
		type p8 struct{}
		type p9 struct{}
		type p10 struct{}
		type section1 struct {
			p1  `route:"/p1 P1"`
			p2  `route:"/p2 P2"`
			p3  `route:"/p3 P3"`
			p4  `route:"/p4 P4"`
			p5  `route:"/p5 P5"`
			p6  `route:"/p6 P6"`
			p7  `route:"/p7 P7"`
			p8  `route:"/p8 P8"`
			p9  `route:"/p9 P9"`
			p10 `route:"/p10 P10"`
		}
		type section2 struct {
			p1  `route:"/p1 P1"`
			p2  `route:"/p2 P2"`
			p3  `route:"/p3 P3"`
			p4  `route:"/p4 P4"`
			p5  `route:"/p5 P5"`
			p6  `route:"/p6 P6"`
			p7  `route:"/p7 P7"`
			p8  `route:"/p8 P8"`
			p9  `route:"/p9 P9"`
			p10 `route:"/p10 P10"`
		}
		type index struct {
			section1 `route:"/section1 Section1"`
			section2 `route:"/section2 Section2"`
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			mux := http.NewServeMux()
			_, _ = Mount(mux, index{}, "/", "Index")
		}
	})

	b.Run("Mount_WithMiddleware", func(b *testing.B) {
		type product struct{}
		type team struct{}
		type index struct {
			product `route:"/product Product"`
			team    `route:"/team Team"`
		}
		mw := func(h http.Handler, pn *PageNode) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.ServeHTTP(w, r)
			})
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			mux := http.NewServeMux()
			_, _ = Mount(mux, index{}, "/", "Index", WithMiddlewares(mw))
		}
	})
}

// ============================================================================
// 3. REQUEST HANDLING BENCHMARKS (HOT PATH)
// ============================================================================

// Test component for benchmarking
type benchComp struct{}

func (benchComp) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte("<div>test</div>"))
	return err
}

// Benchmark test types
type benchIndex struct{}

func (benchIndex) Page() component { return benchComp{} }

type benchProduct struct{}

func (benchProduct) Page() component { return benchComp{} }

type benchIndexWithProps struct {
	data string
}

func (p *benchIndexWithProps) Props(r *http.Request) error {
	p.data = "test"
	return nil
}

func (p benchIndexWithProps) Page() component { return benchComp{} }

type benchIndexHTMX struct{}

func (benchIndexHTMX) Page() component    { return benchComp{} }
func (benchIndexHTMX) Content() component { return benchComp{} }

func BenchmarkRequestHandling(b *testing.B) {
	b.Run("ServeHTTP_SimpleGET", func(b *testing.B) {
		mux := http.NewServeMux()
		sp, _ := Mount(mux, benchIndex{}, "/", "Index")
		_ = sp

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			w.Code = 0
			mux.ServeHTTP(w, req)
		}
	})

	b.Run("ServeHTTP_WithParams", func(b *testing.B) {
		type index struct {
			benchProduct `route:"/product/{id} Product"`
		}

		mux := http.NewServeMux()
		sp, _ := Mount(mux, index{}, "/", "Index")
		_ = sp

		req := httptest.NewRequest("GET", "/product/123", nil)
		w := httptest.NewRecorder()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			w.Code = 0
			mux.ServeHTTP(w, req)
		}
	})

	b.Run("ServeHTTP_WithProps", func(b *testing.B) {
		mux := http.NewServeMux()
		sp, _ := Mount(mux, &benchIndexWithProps{}, "/", "Index")
		_ = sp

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			w.Code = 0
			mux.ServeHTTP(w, req)
		}
	})

	b.Run("ServeHTTP_HTMX_Partial", func(b *testing.B) {
		mux := http.NewServeMux()
		sp, _ := Mount(mux, benchIndexHTMX{}, "/", "Index")
		_ = sp

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Hx-Request", "true")
		req.Header.Set("Hx-Target", "content")
		w := httptest.NewRecorder()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			w.Code = 0
			mux.ServeHTTP(w, req)
		}
	})

	b.Run("ServeHTTP_POST_Form", func(b *testing.B) {
		mux := http.NewServeMux()
		sp, _ := Mount(mux, benchIndex{}, "/", "Index")
		_ = sp

		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w.Body.Reset()
			w.Code = 0
			mux.ServeHTTP(w, req)
		}
	})
}

// ============================================================================
// 4. URL GENERATION BENCHMARKS
// ============================================================================

func BenchmarkURLGeneration(b *testing.B) {
	type product struct{}
	type index struct {
		product `route:"/product/{id} Product"`
	}

	mux := http.NewServeMux()
	sp, _ := Mount(mux, index{}, "/", "Index")
	ctx := pcCtx.WithValue(context.Background(), sp.pc)

	b.Run("URLFor_NoParams", func(b *testing.B) {
		type simple struct{}
		type idx struct {
			simple `route:"/simple Simple"`
		}
		mux := http.NewServeMux()
		sp, _ := Mount(mux, idx{}, "/", "Index")
		ctxSimple := pcCtx.WithValue(context.Background(), sp.pc)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = URLFor(ctxSimple, simple{})
		}
	})

	b.Run("URLFor_OneParam_Positional", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = URLFor(ctx, product{}, "123")
		}
	})

	b.Run("URLFor_MultipleParams_Positional", func(b *testing.B) {
		type page struct{}
		type idx struct {
			page `route:"/users/{userId}/posts/{postId} Page"`
		}
		mux := http.NewServeMux()
		sp, _ := Mount(mux, idx{}, "/", "Index")
		ctxMulti := pcCtx.WithValue(context.Background(), sp.pc)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = URLFor(ctxMulti, page{}, "user1", "post2")
		}
	})

	b.Run("URLFor_WithMap", func(b *testing.B) {
		args := map[string]any{"id": "123"}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = URLFor(ctx, product{}, args)
		}
	})

	b.Run("URLFor_WithContext", func(b *testing.B) {
		params := map[string]string{"id": "123"}
		ctxWithParams := urlParamsCtx.WithValue(ctx, params)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = URLFor(ctxWithParams, product{})
		}
	})

	b.Run("URLFor_Repeated_10Times", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < 10; j++ {
				_, _ = URLFor(ctx, product{}, "123")
			}
		}
	})
}

// ============================================================================
// 5. ID GENERATION BENCHMARKS
// ============================================================================

type benchIndexWithUserList struct{}

func (benchIndexWithUserList) UserList() component { return benchComp{} }

func BenchmarkIDGeneration(b *testing.B) {
	mux := http.NewServeMux()
	sp, _ := Mount(mux, benchIndexWithUserList{}, "/", "Index")
	ctx := pcCtx.WithValue(context.Background(), sp.pc)

	b.Run("IDFor_UnboundMethod", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ID(ctx, benchIndexWithUserList.UserList)
		}
	})

	b.Run("IDFor_BoundMethod", func(b *testing.B) {
		inst := benchIndexWithUserList{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ID(ctx, inst.UserList)
		}
	})

	b.Run("IDFor_Ref", func(b *testing.B) {
		ref := Ref("benchIndexWithUserList.UserList")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ID(ctx, ref)
		}
	})

	b.Run("IDTarget_UnboundMethod", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = IDTarget(ctx, benchIndexWithUserList.UserList)
		}
	})
}

// ============================================================================
// 6. REFLECTION BENCHMARKS
// ============================================================================

type benchTestPage struct{}

func (benchTestPage) TestMethod() component { return benchComp{} }

type benchTestPageWithDI struct {
	injected string
}

func (benchTestPageWithDI) TestMethod(s string) component { return benchComp{} }

func BenchmarkReflection(b *testing.B) {
	inst := benchTestPage{}

	b.Run("extractMethodInfo_Unbound", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = extractMethodInfo(benchTestPage.TestMethod)
		}
	})

	b.Run("extractMethodInfo_Bound", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = extractMethodInfo(inst.TestMethod)
		}
	})

	b.Run("extractMethodInfo_Function", func(b *testing.B) {
		fn := func() component { return benchComp{} }
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = extractMethodInfo(fn)
		}
	})

	b.Run("callMethod_NoArgs", func(b *testing.B) {
		pc, err := parsePageTree("/", benchTestPage{})
		if err != nil {
			b.Fatal(err)
		}
		pn := pc.root
		method, ok := reflect.TypeOf(benchTestPage{}).MethodByName("TestMethod")
		if !ok {
			b.Fatal("method not found")
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pc.callMethod(pn, &method)
		}
	})

	b.Run("callMethod_WithDI", func(b *testing.B) {
		pc, err := parsePageTree("/", benchTestPageWithDI{}, "injected-value")
		if err != nil {
			b.Fatal(err)
		}
		pn := pc.root
		method, ok := reflect.TypeOf(benchTestPageWithDI{}).MethodByName("TestMethod")
		if !ok {
			b.Fatal("method not found")
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pc.callMethod(pn, &method)
		}
	})

	b.Run("isComponent", func(b *testing.B) {
		method, _ := reflect.TypeOf(benchTestPage{}).MethodByName("TestMethod")
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = isComponent(&method)
		}
	})
}

// ============================================================================
// 7. STRING CONVERSION BENCHMARKS
// ============================================================================

func BenchmarkStringConversion(b *testing.B) {
	b.Run("camelToKebab_Short", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = camelToKebab("UserList")
		}
	})

	b.Run("camelToKebab_Long", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = camelToKebab("VeryLongComponentMethodName")
		}
	})

	b.Run("camelToKebab_WithAcronym", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = camelToKebab("HTMLParser")
		}
	})

	b.Run("kebabToPascal_Short", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = kebabToPascal("user-list")
		}
	})

	b.Run("kebabToPascal_Long", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = kebabToPascal("very-long-component-method-name")
		}
	})
}

// ============================================================================
// 8. END-TO-END SCENARIO BENCHMARKS
// ============================================================================

func BenchmarkEndToEnd(b *testing.B) {
	b.Run("CompleteFlow_SimpleGET", func(b *testing.B) {
		// Simulate a complete request flow: parse, mount, handle request
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux := http.NewServeMux()
			_, _ = Mount(mux, benchIndex{}, "/", "Index")
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
		}
	})

	b.Run("CompleteFlow_WithURLGeneration", func(b *testing.B) {
		type index struct {
			benchProduct `route:"/product/{id} Product"`
		}

		mux := http.NewServeMux()
		sp, _ := Mount(mux, index{}, "/", "Index")
		ctx := pcCtx.WithValue(context.Background(), sp.pc)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			_, _ = URLFor(ctx, benchProduct{}, "123")
		}
	})
}
