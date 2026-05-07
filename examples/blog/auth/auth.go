// Package auth provides a tiny cookie-session demo. Real apps should swap in
// hashed passwords (bcrypt/argon2) and a signed/encrypted session cookie.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/jackielii/structpages/examples/blog/store"
)

const cookieName = "blog_session"

var ErrInvalidCredentials = errors.New("invalid credentials")

// Service issues opaque session ids and resolves them back to users via the
// shared store. Sessions are kept in-process; restarting the server logs
// everyone out.
type Service struct {
	store    *store.Store
	mu       sync.RWMutex
	sessions map[string]int // sessionID -> userID
}

func New(s *store.Store) *Service {
	return &Service{store: s, sessions: map[string]int{}}
}

// Login validates the credentials, creates a session, and writes the cookie.
func (a *Service) Login(w http.ResponseWriter, username, password string) (store.User, error) {
	u, err := a.store.FindUserByUsername(username)
	if err != nil || u.Password != password {
		return store.User{}, ErrInvalidCredentials
	}
	id, err := newSessionID()
	if err != nil {
		return store.User{}, fmt.Errorf("create session id: %w", err)
	}
	a.mu.Lock()
	a.sessions[id] = u.ID
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return u, nil
}

// Logout invalidates the session for the current request and clears the cookie.
func (a *Service) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:   cookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// Current returns the logged-in user, or (zero, false) if there isn't one.
func (a *Service) Current(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return store.User{}, false
	}
	a.mu.RLock()
	uid, ok := a.sessions[c.Value]
	a.mu.RUnlock()
	if !ok {
		return store.User{}, false
	}
	u, err := a.store.GetUser(uid)
	if err != nil {
		return store.User{}, false
	}
	return u, true
}

func newSessionID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
