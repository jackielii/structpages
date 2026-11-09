package main

import (
	"errors"
	"slices"
	"sync"
)

var errNotFound = errors.New("not found")

type Project struct{ ID, Name string }

type Task struct{ ID, ProjectID, Title string }

type Comment struct{ ID, TaskID, Author, Text string }

// A Mention points at a comment. Which project and task it belongs to, and
// whether the reader may see it, are only known from the record.
type Mention struct {
	ID, Who, ProjectID, TaskID, CommentID string
	Private                               bool
}

// Store is an in-memory stand-in for a database.
type Store struct {
	mu       sync.Mutex
	projects []Project
	tasks    []Task
	comments []Comment
	mentions []Mention
}

func NewStore() *Store {
	return &Store{
		projects: []Project{{ID: "42", Name: "Website relaunch"}},
		tasks: []Task{
			{ID: "7", ProjectID: "42", Title: "Write the launch post"},
			{ID: "8", ProjectID: "42", Title: "Move DNS"},
		},
		comments: []Comment{
			{ID: "1", TaskID: "7", Author: "ana", Text: "First draft is in the doc."},
			{ID: "2", TaskID: "7", Author: "sam", Text: "@ana looks good, ship it."},
			{ID: "3", TaskID: "8", Author: "ana", Text: "@sam TTL is lowered."},
		},
		mentions: []Mention{
			{ID: "1", Who: "ana", ProjectID: "42", TaskID: "7", CommentID: "2"},
			{ID: "2", Who: "sam", ProjectID: "42", TaskID: "8", CommentID: "3"},
			{ID: "3", Who: "lee", ProjectID: "42", TaskID: "8", CommentID: "3", Private: true},
		},
	}
}

func (s *Store) Project(id string) (Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.projects {
		if p.ID == id {
			return p, nil
		}
	}
	return Project{}, errNotFound
}

func (s *Store) Tasks(projectID string) []Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Task
	for _, t := range s.tasks {
		if t.ProjectID == projectID {
			out = append(out, t)
		}
	}
	return out
}

func (s *Store) Task(projectID, id string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tasks {
		if t.ProjectID == projectID && t.ID == id {
			return t, nil
		}
	}
	return Task{}, errNotFound
}

func (s *Store) RenameTask(projectID, id, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.tasks, func(t Task) bool { return t.ProjectID == projectID && t.ID == id })
	if i < 0 {
		return errNotFound
	}
	s.tasks[i].Title = title
	return nil
}

func (s *Store) Comments(taskID string) []Comment {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Comment
	for _, c := range s.comments {
		if c.TaskID == taskID {
			out = append(out, c)
		}
	}
	return out
}

func (s *Store) Comment(id string) (Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.comments {
		if c.ID == id {
			return c, nil
		}
	}
	return Comment{}, errNotFound
}

func (s *Store) Mentions() []Mention {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.mentions)
}

// Mention looks up a mention for the current reader. A private one is
// reported as not found, the same as a missing one.
func (s *Store) Mention(id string) (Mention, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.mentions {
		if m.ID == id && !m.Private {
			return m, nil
		}
	}
	return Mention{}, errNotFound
}
