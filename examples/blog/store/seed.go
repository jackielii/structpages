package store

import "time"

// SeedDemo populates the store with a small blog so the example is interactive
// from the first page load. The default admin credentials are admin/admin.
func SeedDemo(s *Store) {
	_, _ = s.CreateUser(User{Username: "admin", Password: "admin", IsAdmin: true})
	_, _ = s.CreateUser(User{Username: "alice", Password: "alice", IsAdmin: false})

	news, _ := s.CreateCategory(Category{Slug: "news", Name: "News"})
	guides, _ := s.CreateCategory(Category{Slug: "guides", Name: "Guides"})
	_, _ = s.CreateCategory(Category{Slug: "release-notes", Name: "Release Notes"})

	now := time.Now()
	posts := []Post{
		{
			Slug:       "welcome",
			Title:      "Welcome to the structpages blog example",
			Body:       "This blog is rendered by structpages, a struct-based router for Go that pairs with templ and HTMX. Read on for the patterns it demonstrates.",
			AuthorID:   1,
			CategoryID: news.ID,
			Published:  true,
			CreatedAt:  now.Add(-72 * time.Hour),
		},
		{
			Slug:       "module-organization",
			Title:      "Organizing templ components like a React app",
			Body:       "Each feature lives in its own Go package. Shared UI primitives live under ui/components and can be imported from any feature. Cross-feature links use Ref to avoid import cycles.",
			AuthorID:   1,
			CategoryID: guides.ID,
			Published:  true,
			CreatedAt:  now.Add(-48 * time.Hour),
		},
		{
			Slug:       "render-target-pattern",
			Title:      "The Props + RenderTarget pattern",
			Body:       "Inject RenderTarget into Props and call target.Is(MyWidget) to load only the data each HTMX target needs. The admin dashboard uses this for three independent widgets.",
			AuthorID:   1,
			CategoryID: guides.ID,
			Published:  true,
			CreatedAt:  now.Add(-24 * time.Hour),
		},
		{
			Slug:       "page-content-split",
			Title:      "Page() vs Content() — what's the difference?",
			Body:       "Page() wraps the layout for full document loads. Content() renders just the body so HTMX can swap it without refetching the shell. Most pages define both.",
			AuthorID:   2,
			CategoryID: guides.ID,
			Published:  true,
			CreatedAt:  now.Add(-12 * time.Hour),
		},
		{
			Slug:       "middleware-and-auth",
			Title:      "Page-level Middlewares() with dependency injection",
			Body:       "The admin Pages struct returns a RequireAdmin middleware that reads the session cookie. The middleware factory receives *auth.Service via the same DI registry handlers use.",
			AuthorID:   1,
			CategoryID: news.ID,
			Published:  true,
			CreatedAt:  now.Add(-6 * time.Hour),
		},
		{
			Slug:       "draft-post",
			Title:      "Draft: things still cooking",
			Body:       "Drafts only appear in the admin list. The public reader's PostFilter sets IncludeDraft=false.",
			AuthorID:   1,
			CategoryID: news.ID,
			Published:  false,
			CreatedAt:  now.Add(-3 * time.Hour),
		},
	}
	for _, p := range posts {
		_, _ = s.CreatePost(p)
	}

	welcome, _ := s.GetPostBySlug("welcome")
	_, _ = s.AddComment(welcome.ID, "alice", "Great intro — looking forward to the rest.")
	_, _ = s.AddComment(welcome.ID, "bob", "How does the Page/Content split work with non-HTMX clients?")
}
