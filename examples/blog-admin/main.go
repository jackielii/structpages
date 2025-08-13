package main

import (
	"database/sql"
	"embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/go-playground/form/v4"
	"github.com/jackielii/structpages"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed static/*
var staticFS embed.FS

var (
	flagPort = flag.String("port", "8080", "Port to listen on")
	flagDB   = flag.String("db", "blog.db", "Database file path")
)

func main() {
	flag.Parse()

	// Initialize database
	db, err := initDB(*flagDB)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Initialize services
	sessionManager := initSessionManager()
	store := NewStore(db)
	auth := NewAuthService(store, sessionManager)
	formDecoder := form.NewDecoder()
	formDecoder.SetTagName("form")
	config := &Config{
		SiteName:        "Structpages Blog",
		SiteDescription: "A powerful blog platform built with structpages",
		AdminEmail:      "admin@example.com",
	}

	// Initialize structpages
	sp := structpages.New(
		structpages.WithDefaultPageConfig(structpages.HTMXPageConfig),
		structpages.WithErrorHandler(customErrorHandler),
		structpages.WithMiddlewares(
			wrapMiddleware(sessionMiddleware(sessionManager)),
			wrapMiddleware(loggingMiddleware),
		),
	)

	// Create router
	mux := http.NewServeMux()
	r := structpages.NewRouter(mux)

	// Mount pages with dependency injection
	if err := sp.MountPages(r, pages{}, "/", "Structpages Blog",
		store,
		auth,
		sessionManager,
		formDecoder,
		config,
	); err != nil {
		log.Fatal("Failed to mount pages:", err)
	}

	// Serve static files
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", fileServer)

	// Print available routes
	fmt.Println("\nAvailable routes:")
	printRoutes(mux, "")

	// Start server
	addr := fmt.Sprintf(":%s", *flagPort)
	log.Printf("Starting blog server on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

func initDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}

	// Create tables
	if err := createTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

func initSessionManager() *scs.SessionManager {
	sessionManager := scs.New()
	sessionManager.Store = memstore.New()
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Name = "blog_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Persist = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Secure = false // Set to true in production with HTTPS
	return sessionManager
}

func customErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Error handling request %s: %v", r.URL.Path, err)

	// Check if it's an HTTP error
	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		w.WriteHeader(httpErr.Code)
		if httpErr.Code == http.StatusNotFound {
			fmt.Fprintf(w, "Page not found: %s", r.URL.Path)
		} else {
			fmt.Fprintf(w, "Error %d: %s", httpErr.Code, httpErr.Message)
		}
		return
	}

	// Default error response
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "Internal server error")
}

// Middleware

func sessionMiddleware(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return sm.LoadAndSave(next)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// wrapMiddleware converts a standard middleware to structpages MiddlewareFunc
func wrapMiddleware(mw func(http.Handler) http.Handler) structpages.MiddlewareFunc {
	return func(next http.Handler, pn *structpages.PageNode) http.Handler {
		return mw(next)
	}
}

// Print routes helper
func printRoutes(mux *http.ServeMux, prefix string) {
	// This is a simplified version - in production you'd use reflection
	// or a more sophisticated approach to list all routes
	routes := []string{
		"/",
		"/posts/{slug}",
		"/category/{slug}",
		"/search",
		"/login",
		"/logout",
		"/admin",
		"/admin/posts",
		"/admin/posts/new",
		"/admin/posts/{id}/edit",
		"/admin/users",
		"/admin/settings",
		"/api/posts/{id}/publish",
		"/static/",
	}

	for _, route := range routes {
		fmt.Printf("%s%s\n", prefix, route)
	}
}

// Config holds application configuration
type Config struct {
	SiteName        string
	SiteDescription string
	AdminEmail      string
}
