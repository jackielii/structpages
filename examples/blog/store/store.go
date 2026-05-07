package store

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

const DefaultPageSize = 5

var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("duplicate")
)

// Store is an in-memory, mutex-protected datastore. Every method takes the
// lock at most once and returns copies so callers can read concurrently.
type Store struct {
	mu         sync.RWMutex
	users      map[int]*User
	categories map[int]*Category
	posts      map[int]*Post
	comments   map[int][]*Comment // by PostID
	nextID     map[string]int
}

func New() *Store {
	return &Store{
		users:      map[int]*User{},
		categories: map[int]*Category{},
		posts:      map[int]*Post{},
		comments:   map[int][]*Comment{},
		nextID:     map[string]int{},
	}
}

func (s *Store) nextIDFor(kind string) int {
	s.nextID[kind]++
	return s.nextID[kind]
}

// --- Users ---

func (s *Store) FindUserByUsername(username string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Username == username {
			return *u, nil
		}
	}
	return User{}, ErrNotFound
}

func (s *Store) GetUser(id int) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return *u, nil
}

func (s *Store) ListUsers() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) CreateUser(u User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.users {
		if existing.Username == u.Username {
			return User{}, ErrDuplicate
		}
	}
	u.ID = s.nextIDFor("user")
	s.users[u.ID] = &u
	return u, nil
}

func (s *Store) DeleteUser(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return ErrNotFound
	}
	delete(s.users, id)
	return nil
}

// --- Categories ---

func (s *Store) ListCategories() []Category {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Category, 0, len(s.categories))
	for _, c := range s.categories {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) GetCategory(id int) (Category, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.categories[id]
	if !ok {
		return Category{}, ErrNotFound
	}
	return *c, nil
}

func (s *Store) GetCategoryBySlug(slug string) (Category, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.categories {
		if c.Slug == slug {
			return *c, nil
		}
	}
	return Category{}, ErrNotFound
}

func (s *Store) CreateCategory(c Category) (Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.ID = s.nextIDFor("category")
	s.categories[c.ID] = &c
	return c, nil
}

// --- Posts ---

func (s *Store) GetPost(id int) (Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.posts[id]
	if !ok {
		return Post{}, ErrNotFound
	}
	return *p, nil
}

func (s *Store) GetPostBySlug(slug string) (Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.posts {
		if p.Slug == slug {
			return *p, nil
		}
	}
	return Post{}, ErrNotFound
}

// ListPosts returns paginated posts and the total count matching the filter.
func (s *Store) ListPosts(f PostFilter) (posts []Post, total int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(f.Search)
	matched := make([]*Post, 0, len(s.posts))
	for _, p := range s.posts {
		if !f.IncludeDraft && !p.Published {
			continue
		}
		if f.CategoryID != 0 && p.CategoryID != f.CategoryID {
			continue
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(p.Title), q) &&
				!strings.Contains(strings.ToLower(p.Body), q) {
				continue
			}
		}
		matched = append(matched, p)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})
	total = len(matched)

	page := f.Page
	if page < 1 {
		page = 1
	}
	size := f.PageSize
	if size <= 0 {
		size = DefaultPageSize
	}
	start := (page - 1) * size
	end := start + size
	if start > total {
		return nil, total
	}
	if end > total {
		end = total
	}
	out := make([]Post, 0, end-start)
	for _, p := range matched[start:end] {
		out = append(out, *p)
	}
	return out, total
}

func (s *Store) CreatePost(p Post) (Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	if p.Slug == "" {
		p.Slug = slugify(p.Title)
	}
	for _, existing := range s.posts {
		if existing.Slug == p.Slug {
			return Post{}, ErrDuplicate
		}
	}
	p.ID = s.nextIDFor("post")
	s.posts[p.ID] = &p
	return p, nil
}

func (s *Store) UpdatePost(id int, mut func(*Post)) (Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.posts[id]
	if !ok {
		return Post{}, ErrNotFound
	}
	mut(p)
	return *p, nil
}

func (s *Store) DeletePost(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.posts[id]; !ok {
		return ErrNotFound
	}
	delete(s.posts, id)
	delete(s.comments, id)
	return nil
}

// --- Comments ---

func (s *Store) ListComments(postID int) []Comment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.comments[postID]
	out := make([]Comment, len(src))
	for i, c := range src {
		out[i] = *c
	}
	return out
}

func (s *Store) AddComment(postID int, author, body string) (Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.posts[postID]; !ok {
		return Comment{}, ErrNotFound
	}
	c := &Comment{
		ID:        s.nextIDFor("comment"),
		PostID:    postID,
		Author:    author,
		Body:      body,
		CreatedAt: time.Now(),
	}
	s.comments[postID] = append(s.comments[postID], c)
	return *c, nil
}

// --- Stats (used by admin dashboard widgets) ---

type Stats struct {
	Posts      int
	Drafts     int
	Comments   int
	Categories int
}

func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := Stats{Categories: len(s.categories)}
	for _, p := range s.posts {
		if p.Published {
			st.Posts++
		} else {
			st.Drafts++
		}
	}
	for _, cs := range s.comments {
		st.Comments += len(cs)
	}
	return st
}

func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}
