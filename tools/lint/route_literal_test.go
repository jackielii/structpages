package lint

import "testing"

func TestConcreteRoutes(t *testing.T) {
	mk := func(name, full, method string, children ...*PageNode) *PageNode {
		return &PageNode{Name: name, FullRoute: full, Method: method, Children: children}
	}
	tree := &PageTree{Roots: []*PageNode{
		mk("Root", "/", "ALL",
			mk("PatientsIndex", "/receptionist/patients/{$}", "GET"),
			mk("Patients", "/receptionist/patients", "ALL"),
			mk("Schedules", "/receptionist/schedules", "GET"),
			mk("SubmitSchedule", "/receptionist/schedules", "POST"),
			mk("Detail", "/receptionist/patients/detail/{uuid}", "GET"),
		),
	}}

	got := concreteRoutes(tree)

	for _, excluded := range []string{
		"/",                                    // bare root
		"/receptionist/patients/{$}",           // index marker
		"/receptionist/patients/detail/{uuid}", // param segment
	} {
		if _, ok := got[excluded]; ok {
			t.Errorf("route %q should be excluded from concreteRoutes", excluded)
		}
	}

	if n := got["/receptionist/patients"]; n == nil || n.Name != "Patients" {
		t.Errorf("/receptionist/patients: got %v, want page Patients", n)
	}

	// GET wins over the POST sharing the same path.
	if n := got["/receptionist/schedules"]; n == nil || n.Method != "GET" {
		t.Errorf("/receptionist/schedules: got %v, want the GET page", n)
	}
}

func TestPreferAsSuggestion(t *testing.T) {
	get := &PageNode{Method: "GET"}
	post := &PageNode{Method: "POST"}
	all := &PageNode{Method: "ALL"}

	if !preferAsSuggestion(get, post) {
		t.Error("GET should be preferred over POST")
	}
	if preferAsSuggestion(post, get) {
		t.Error("POST should not displace a GET")
	}
	if !preferAsSuggestion(all, post) {
		t.Error("ALL should be preferred over POST")
	}
	if preferAsSuggestion(post, post) {
		t.Error("POST should not displace another POST (no improvement)")
	}
}
