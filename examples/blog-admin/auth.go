package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles authentication
type AuthService struct {
	store   *Store
	session *scs.SessionManager
}

func NewAuthService(store *Store, session *scs.SessionManager) *AuthService {
	return &AuthService{
		store:   store,
		session: session,
	}
}

// Login authenticates a user
func (a *AuthService) Login(r *http.Request, username, password string) (*User, error) {
	user, err := a.store.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Store user ID in session
	a.session.Put(r.Context(), "userID", user.ID)

	return user, nil
}

// Logout logs out the current user
func (a *AuthService) Logout(r *http.Request) {
	a.session.Remove(r.Context(), "userID")
}

// GetUser returns the currently logged in user
func (a *AuthService) GetUser(r *http.Request) *User {
	userID := a.session.GetInt(r.Context(), "userID")
	if userID == 0 {
		return nil
	}

	user, err := a.store.GetUserByID(userID)
	if err != nil {
		return nil
	}

	return user
}

// IsAuthenticated checks if a user is logged in
func (a *AuthService) IsAuthenticated(r *http.Request) bool {
	return a.GetUser(r) != nil
}

// IsAdmin checks if the current user is an admin
func (a *AuthService) IsAdmin(r *http.Request) bool {
	user := a.GetUser(r)
	return user != nil && user.Role == "admin"
}

// IsAuthor checks if the current user is an author or admin
func (a *AuthService) IsAuthor(r *http.Request) bool {
	user := a.GetUser(r)
	return user != nil && (user.Role == "author" || user.Role == "admin")
}

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// Context keys for passing auth data
type contextKey string

const (
	userContextKey contextKey = "user"
)

// WithUser adds user to context
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves user from context
func UserFromContext(ctx context.Context) *User {
	user, ok := ctx.Value(userContextKey).(*User)
	if !ok {
		return nil
	}
	return user
}
