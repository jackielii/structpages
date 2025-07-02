package main

import (
	"database/sql"
	"strings"
	"time"
)

// Store handles all database operations
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// User operations

func (s *Store) GetUserByID(id int) (*User, error) {
	var u User
	err := s.db.QueryRow(`
		SELECT id, username, email, password_hash, role, created_at, updated_at
		FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	var u User
	err := s.db.QueryRow(`
		SELECT id, username, email, password_hash, role, created_at, updated_at
		FROM users WHERE username = ?
	`, username).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateUser(user *User) error {
	result, err := s.db.Exec(`
		INSERT INTO users (username, email, password_hash, role)
		VALUES (?, ?, ?, ?)
	`, user.Username, user.Email, user.PasswordHash, user.Role)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = int(id)
	return nil
}

func (s *Store) ListUsers(limit, offset int) ([]*User, int, error) {
	// Get total count
	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get users
	rows, err := s.db.Query(`
		SELECT id, username, email, password_hash, role, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var u User
		err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, &u)
	}
	return users, total, nil
}

// Post operations

func (s *Store) GetPostBySlug(slug string) (*Post, error) {
	var p Post
	err := s.db.QueryRow(`
		SELECT id, slug, title, content, excerpt, author_id, status, 
		       published_at, created_at, updated_at, view_count
		FROM posts WHERE slug = ?
	`, slug).Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Excerpt,
		&p.AuthorID, &p.Status, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt, &p.ViewCount)
	if err != nil {
		return nil, err
	}

	// Load author
	p.Author, _ = s.GetUserByID(p.AuthorID)

	// Load tags and categories
	p.Tags, _ = s.GetPostTags(p.ID)
	p.Categories, _ = s.GetPostCategories(p.ID)

	return &p, nil
}

func (s *Store) GetPostByID(id int) (*Post, error) {
	var p Post
	err := s.db.QueryRow(`
		SELECT id, slug, title, content, excerpt, author_id, status, 
		       published_at, created_at, updated_at, view_count
		FROM posts WHERE id = ?
	`, id).Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Excerpt,
		&p.AuthorID, &p.Status, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt, &p.ViewCount)
	if err != nil {
		return nil, err
	}

	// Load author
	p.Author, _ = s.GetUserByID(p.AuthorID)

	// Load tags and categories
	p.Tags, _ = s.GetPostTags(p.ID)
	p.Categories, _ = s.GetPostCategories(p.ID)

	return &p, nil
}

func (s *Store) ListPosts(filter PostFilter) ([]*Post, int, error) {
	query := `SELECT id, slug, title, content, excerpt, author_id, status, 
	          published_at, created_at, updated_at, view_count FROM posts WHERE 1=1`
	args := []interface{}{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.AuthorID > 0 {
		query += " AND author_id = ?"
		args = append(args, filter.AuthorID)
	}
	if filter.CategoryID > 0 {
		query += " AND id IN (SELECT post_id FROM post_categories WHERE category_id = ?)"
		args = append(args, filter.CategoryID)
	}
	if filter.Search != "" {
		query += " AND (title LIKE ? OR content LIKE ?)"
		searchTerm := "%" + filter.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	// Get total count
	var total int
	// Build count query from scratch
	countQuery := "SELECT COUNT(*) FROM posts WHERE 1=1"
	countArgs := []interface{}{}

	if filter.Status != "" {
		countQuery += " AND status = ?"
		countArgs = append(countArgs, filter.Status)
	}
	if filter.AuthorID > 0 {
		countQuery += " AND author_id = ?"
		countArgs = append(countArgs, filter.AuthorID)
	}
	if filter.CategoryID > 0 {
		countQuery += " AND id IN (SELECT post_id FROM post_categories WHERE category_id = ?)"
		countArgs = append(countArgs, filter.CategoryID)
	}
	if filter.Search != "" {
		countQuery += " AND (title LIKE ? OR content LIKE ?)"
		searchTerm := "%" + filter.Search + "%"
		countArgs = append(countArgs, searchTerm, searchTerm)
	}

	err := s.db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Add ordering and pagination
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		var p Post
		err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Excerpt,
			&p.AuthorID, &p.Status, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt, &p.ViewCount)
		if err != nil {
			return nil, 0, err
		}

		// Load author
		p.Author, _ = s.GetUserByID(p.AuthorID)

		posts = append(posts, &p)
	}

	return posts, total, nil
}

func (s *Store) CreatePost(post *Post) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT INTO posts (slug, title, content, excerpt, author_id, status, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, post.Slug, post.Title, post.Content, post.Excerpt, post.AuthorID, post.Status, post.PublishedAt)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	post.ID = int(id)

	// Update tags and categories
	if err := s.updatePostTags(tx, post.ID, post.Tags); err != nil {
		return err
	}
	if err := s.updatePostCategories(tx, post.ID, post.Categories); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) UpdatePost(post *Post) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE posts 
		SET slug = ?, title = ?, content = ?, excerpt = ?, 
		    status = ?, published_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, post.Slug, post.Title, post.Content, post.Excerpt,
		post.Status, post.PublishedAt, post.ID)
	if err != nil {
		return err
	}

	// Update tags and categories
	if err := s.updatePostTags(tx, post.ID, post.Tags); err != nil {
		return err
	}
	if err := s.updatePostCategories(tx, post.ID, post.Categories); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) DeletePost(id int) error {
	_, err := s.db.Exec("DELETE FROM posts WHERE id = ?", id)
	return err
}

// Category operations

func (s *Store) ListCategories() ([]*Category, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.slug, c.name, c.description, c.created_at, COUNT(pc.post_id) as post_count
		FROM categories c
		LEFT JOIN post_categories pc ON c.id = pc.category_id
		GROUP BY c.id
		ORDER BY c.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*Category
	for rows.Next() {
		var c Category
		err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.PostCount)
		if err != nil {
			return nil, err
		}
		categories = append(categories, &c)
	}
	return categories, nil
}

func (s *Store) GetCategoryBySlug(slug string) (*Category, error) {
	var c Category
	err := s.db.QueryRow(`
		SELECT c.id, c.slug, c.name, c.description, c.created_at, COUNT(pc.post_id) as post_count
		FROM categories c
		LEFT JOIN post_categories pc ON c.id = pc.category_id
		WHERE c.slug = ?
		GROUP BY c.id
	`, slug).Scan(&c.ID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.PostCount)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Tag operations

func (s *Store) GetPostTags(postID int) ([]Tag, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.name
		FROM tags t
		JOIN post_tags pt ON t.id = pt.tag_id
		WHERE pt.post_id = ?
		ORDER BY t.name
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		err := rows.Scan(&t.ID, &t.Name)
		if err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *Store) GetPostCategories(postID int) ([]Category, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.slug, c.name, c.description, c.created_at
		FROM categories c
		JOIN post_categories pc ON c.id = pc.category_id
		WHERE pc.post_id = ?
		ORDER BY c.name
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt)
		if err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

// Analytics operations

func (s *Store) IncrementPostViews(postID int) error {
	_, err := s.db.Exec("UPDATE posts SET view_count = view_count + 1 WHERE id = ?", postID)
	return err
}

func (s *Store) RecordPageView(postID *int, path, ipHash, userAgent, referrer string) error {
	_, err := s.db.Exec(`
		INSERT INTO page_views (post_id, path, ip_hash, user_agent, referrer)
		VALUES (?, ?, ?, ?, ?)
	`, postID, path, ipHash, userAgent, referrer)
	return err
}

func (s *Store) GetAnalytics(date time.Time) (*Analytics, error) {
	// This is a simplified version - in production you'd have more sophisticated analytics
	var analytics Analytics
	analytics.Date = date

	// Get daily stats
	err := s.db.QueryRow(`
		SELECT COUNT(DISTINCT ip_hash) as visitors, COUNT(*) as page_views
		FROM page_views
		WHERE DATE(created_at) = DATE(?)
	`, date).Scan(&analytics.Visitors, &analytics.PageViews)
	if err != nil {
		return nil, err
	}

	// Get top posts
	rows, err := s.db.Query(`
		SELECT p.id, p.title, COUNT(pv.id) as views
		FROM posts p
		JOIN page_views pv ON p.id = pv.post_id
		WHERE DATE(pv.created_at) = DATE(?)
		GROUP BY p.id
		ORDER BY views DESC
		LIMIT 10
	`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var pa PostAnalytics
		err := rows.Scan(&pa.PostID, &pa.PostTitle, &pa.Views)
		if err != nil {
			return nil, err
		}
		analytics.TopPosts = append(analytics.TopPosts, pa)
	}

	return &analytics, nil
}

// Helper methods

func (s *Store) updatePostTags(tx *sql.Tx, postID int, tags []Tag) error {
	// Delete existing tags
	_, err := tx.Exec("DELETE FROM post_tags WHERE post_id = ?", postID)
	if err != nil {
		return err
	}

	// Insert new tags
	for _, tag := range tags {
		// Get or create tag
		var tagID int
		err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", tag.Name).Scan(&tagID)
		if err == sql.ErrNoRows {
			result, err := tx.Exec("INSERT INTO tags (name) VALUES (?)", tag.Name)
			if err != nil {
				return err
			}
			id, err := result.LastInsertId()
			if err != nil {
				return err
			}
			tagID = int(id)
		} else if err != nil {
			return err
		}

		// Link tag to post
		_, err = tx.Exec("INSERT INTO post_tags (post_id, tag_id) VALUES (?, ?)", postID, tagID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) updatePostCategories(tx *sql.Tx, postID int, categories []Category) error {
	// Delete existing categories
	_, err := tx.Exec("DELETE FROM post_categories WHERE post_id = ?", postID)
	if err != nil {
		return err
	}

	// Insert new categories
	for _, cat := range categories {
		_, err = tx.Exec("INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)", postID, cat.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

// Filter types

type PostFilter struct {
	Status     string
	AuthorID   int
	CategoryID int
	Search     string
	Limit      int
	Offset     int
}

// Generate slug from title
func GenerateSlug(title string) string {
	// Simple slug generation - in production use a proper slugify library
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "'", "")
	slug = strings.ReplaceAll(slug, "\"", "")
	slug = strings.ReplaceAll(slug, ".", "")
	slug = strings.ReplaceAll(slug, ",", "")
	slug = strings.ReplaceAll(slug, "!", "")
	slug = strings.ReplaceAll(slug, "?", "")
	slug = strings.ReplaceAll(slug, "&", "and")
	return slug
}
