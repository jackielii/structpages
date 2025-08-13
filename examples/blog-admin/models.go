package main

import (
	"database/sql"
	"time"
)

// User represents a blog user
type User struct {
	ID           int
	Username     string
	Email        string
	PasswordHash string
	Role         string // "admin", "author", "reader"
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Post represents a blog post
type Post struct {
	ID          int
	Slug        string
	Title       string
	Content     string
	Excerpt     string
	AuthorID    int
	Author      *User
	Status      string // "draft", "published"
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ViewCount   int
	Tags        []Tag
	Categories  []Category
}

// Category represents a blog category
type Category struct {
	ID          int
	Slug        string
	Name        string
	Description string
	PostCount   int
	CreatedAt   time.Time
}

// Tag represents a blog tag
type Tag struct {
	ID        int
	Name      string
	PostCount int
}

// Comment represents a blog comment
type Comment struct {
	ID        int
	PostID    int
	Post      *Post
	AuthorID  *int
	Author    *User
	Name      string // For anonymous comments
	Email     string // For anonymous comments
	Content   string
	Status    string // "pending", "approved", "spam"
	CreatedAt time.Time
}

// Media represents uploaded media files
type Media struct {
	ID          int
	Filename    string
	Path        string
	MimeType    string
	Size        int64
	UploadedBy  int
	UploadedAt  time.Time
	Alt         string
	Description string
}

// Analytics represents basic analytics data
type Analytics struct {
	Date      time.Time
	PageViews int
	Visitors  int
	NewUsers  int
	TopPosts  []PostAnalytics
}

type PostAnalytics struct {
	PostID    int
	PostTitle string
	Views     int
}

// Database schema
func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'reader',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT UNIQUE NOT NULL,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		excerpt TEXT,
		author_id INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'draft',
		published_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		view_count INTEGER DEFAULT 0,
		FOREIGN KEY (author_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL
	);

	CREATE TABLE IF NOT EXISTS post_categories (
		post_id INTEGER NOT NULL,
		category_id INTEGER NOT NULL,
		PRIMARY KEY (post_id, category_id),
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
		FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS post_tags (
		post_id INTEGER NOT NULL,
		tag_id INTEGER NOT NULL,
		PRIMARY KEY (post_id, tag_id),
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER NOT NULL,
		author_id INTEGER,
		name TEXT,
		email TEXT,
		content TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
		FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE SET NULL
	);

	CREATE TABLE IF NOT EXISTS media (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT NOT NULL,
		path TEXT NOT NULL,
		mime_type TEXT NOT NULL,
		size INTEGER NOT NULL,
		uploaded_by INTEGER NOT NULL,
		uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		alt TEXT,
		description TEXT,
		FOREIGN KEY (uploaded_by) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS page_views (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER,
		path TEXT NOT NULL,
		ip_hash TEXT NOT NULL,
		user_agent TEXT,
		referrer TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
	);

	-- Create indexes
	CREATE INDEX IF NOT EXISTS idx_posts_slug ON posts(slug);
	CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);
	CREATE INDEX IF NOT EXISTS idx_posts_published_at ON posts(published_at);
	CREATE INDEX IF NOT EXISTS idx_categories_slug ON categories(slug);
	CREATE INDEX IF NOT EXISTS idx_comments_post_id ON comments(post_id);
	CREATE INDEX IF NOT EXISTS idx_comments_status ON comments(status);
	CREATE INDEX IF NOT EXISTS idx_page_views_created_at ON page_views(created_at);

	-- Insert default admin user (password: admin123)
	INSERT OR IGNORE INTO users (id, username, email, password_hash, role) 
	VALUES (1, 'admin', 'admin@example.com', '$2a$10$i/CMm4KCafFhALzoVAqRxO8MpyllRakjq5gwOFH4KMdeIWodOjKyS', 'admin');

	-- Insert sample categories
	INSERT OR IGNORE INTO categories (id, slug, name, description) VALUES
	(1, 'technology', 'Technology', 'Posts about technology and programming'),
	(2, 'design', 'Design', 'Posts about design and UX'),
	(3, 'business', 'Business', 'Posts about business and entrepreneurship');

	-- Insert sample posts
	INSERT OR IGNORE INTO posts (id, slug, title, content, excerpt, author_id, status, published_at, view_count) VALUES
	(1, 'welcome-to-structpages', 'Welcome to Structpages Blog', '<p>This is a demo blog built with the structpages framework. It demonstrates advanced features like nested routing, authentication, and HTMX integration.</p>', 'A demo blog showcasing structpages features', 1, 'published', CURRENT_TIMESTAMP, 42),
	(2, 'building-with-go', 'Building Web Apps with Go', '<p>Go is an excellent choice for building web applications. Its simplicity and performance make it ideal for modern web development.</p>', 'Why Go is great for web development', 1, 'published', CURRENT_TIMESTAMP, 15),
	(3, 'draft-post', 'Work in Progress', '<p>This is a draft post that is not yet published.</p>', 'Coming soon...', 1, 'draft', NULL, 0);

	-- Link posts to categories
	INSERT OR IGNORE INTO post_categories (post_id, category_id) VALUES
	(1, 1), (1, 2),
	(2, 1),
	(3, 3);
	`

	_, err := db.Exec(schema)
	return err
}
