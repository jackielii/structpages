package main

import (
	"log"
	"maps"
	"net/http"
	"slices"
	"sync"

	"github.com/jackielii/structpages"
)

type Todo struct {
	ID        int
	Text      string
	Completed bool
}

var (
	todoStore = make(map[int]*Todo)
	nextID    = 1
	mu        sync.RWMutex
)

func addTodo(text string) *Todo {
	mu.Lock()
	defer mu.Unlock()
	todo := &Todo{
		ID:        nextID,
		Text:      text,
		Completed: false,
	}
	todoStore[nextID] = todo
	nextID++
	return todo
}

func getTodos() []*Todo {
	mu.RLock()
	defer mu.RUnlock()
	todos := make([]*Todo, 0, len(todoStore))
	keys := maps.Keys(todoStore)
	for _, key := range slices.Backward(slices.Sorted(keys)) {
		todos = append(todos, todoStore[key])
	}
	return todos
}

func toggleTodo(id int) {
	mu.Lock()
	defer mu.Unlock()
	if todo, exists := todoStore[id]; exists {
		todo.Completed = !todo.Completed
	}
}

func removeTodo(id int) {
	mu.Lock()
	defer mu.Unlock()
	delete(todoStore, id)
}

func main() {
	// Add some sample todos
	addTodo("Learn Go")
	addTodo("Build a TODO app")
	addTodo("Deploy to production")

	sp := structpages.New(
		structpages.WithErrorHandler(errorHandler),
	)
	router := structpages.NewRouter(http.DefaultServeMux)
	if err := sp.MountPages(router, index{}, "/", "index"); err != nil {
		log.Fatalf("Failed to mount pages: %v", err)
	}
	log.Println("Starting TODO app on :8080")
	http.ListenAndServe(":8080", router)
}

func errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Error: %v", err)
	if r.Header.Get("Hx-Request") == "true" {
		errorComp(err).Render(r.Context(), w)
		return
	}
	errorPage(err).Render(r.Context(), w)
}
