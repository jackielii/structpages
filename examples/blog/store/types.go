package store

import "time"

type User struct {
	ID       int
	Username string
	Password string // demo only — plaintext (see README "not for production")
	IsAdmin  bool
}

type Category struct {
	ID   int
	Slug string
	Name string
}

type Post struct {
	ID         int
	Slug       string
	Title      string
	Body       string
	AuthorID   int
	CategoryID int
	Published  bool
	CreatedAt  time.Time
}

type Comment struct {
	ID        int
	PostID    int
	Author    string
	Body      string
	CreatedAt time.Time
}

// PostFilter narrows a ListPosts query. Zero value = all published posts.
type PostFilter struct {
	CategoryID   int    // 0 = any
	Search       string // case-insensitive title/body match; "" = any
	IncludeDraft bool   // false = published only
	Page         int    // 1-indexed; 0 treated as 1
	PageSize     int    // 0 treated as default
}
