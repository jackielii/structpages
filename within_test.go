package structpages

import (
	"context"
	"io"
	"strings"
	"testing"
)

type withinLeaf struct{}

func (withinLeaf) Page() component {
	return fnComponent(func(context.Context, io.Writer) error { return nil })
}

type (
	withinDashboard struct{}
	withinInbox     struct{}
	withinTicket    struct{}
	withinOther     struct{}
)

func (withinDashboard) Page() component { return withinLeaf{}.Page() }
func (withinInbox) Page() component     { return withinLeaf{}.Page() }
func (withinTicket) Page() component    { return withinLeaf{}.Page() }
func (withinOther) Page() component     { return withinLeaf{}.Page() }

type withinManage struct {
	Inbox  withinInbox  `route:"/{$} Inbox"`
	Ticket withinTicket `route:"/tickets/{number} Ticket"`
}

type withinRoot struct {
	Dashboard withinDashboard `route:"/{$} Dashboard"`
	Manage    withinManage    `route:"/manage Manage"`
	Other     withinOther     `route:"/other Other"`
}

// withinCtx returns a context rendering the page that ref names, as
// structpages sets it up for a matched request.
func withinCtx(t *testing.T, ref string) context.Context {
	t.Helper()
	sp, err := Parse(&withinRoot{}, "/", "App")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	current, err := sp.pc.findPageNodeByRef(ref)
	if err != nil {
		t.Fatalf("current page %q: %v", ref, err)
	}
	return currentPageCtx.WithValue(sp.PageContext(context.Background()), current)
}

func TestWithin(t *testing.T) {
	cases := []struct {
		name    string
		current string
		target  any
		want    bool
	}{
		{"leaf is within itself", "Other", withinOther{}, true},
		{"leaf is not within a sibling", "Other", withinDashboard{}, false},
		{"leaf is within its group", "Ticket", withinManage{}, true},
		{"leaf is within the root", "Ticket", &withinRoot{}, true},
		{"index leaf is within its group", "Inbox", withinManage{}, true},
		{"index leaf is not within a sibling leaf", "Inbox", withinTicket{}, false},
		{"sibling leaf is not within the index leaf", "Ticket", withinInbox{}, false},
		{"root index leaf excludes other pages", "Ticket", withinDashboard{}, false},
		{"path params need no args", "Ticket", withinTicket{}, true},
		{"Ref by name", "Ticket", Ref("Manage"), true},
		{"qualified Ref", "Ticket", Ref("Manage.Ticket"), true},
		{"route Ref", "Ticket", Ref("/manage"), true},
		{"string is Ref sugar", "Inbox", "Manage.Inbox", true},
		{"chain", "Ticket", []any{withinManage{}, withinTicket{}}, true},
		{"predicate", "Other", func(n *PageNode) bool { return n.Name == "Other" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Within(withinCtx(t, tc.current), tc.target)
			if err != nil {
				t.Fatalf("Within: %v", err)
			}
			if got != tc.want {
				t.Errorf("Within(%s, %v) = %v, want %v", tc.current, tc.target, got, tc.want)
			}
		})
	}
}

func TestWithin_errors(t *testing.T) {
	cases := []struct {
		name    string
		target  any
		wantErr string
	}{
		{"unknown Ref", Ref("Manage.Nope"), `segment`},
		{"unmounted type", withinLeaf{}, "no page node found"},
		{"string fragment in chain", []any{withinManage{}, "?q=1"}, "string fragment"},
		{"empty chain", []any{}, "empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Within(withinCtx(t, "Ticket"), tc.target)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Within = %v, %v; want error containing %q", got, err, tc.wantErr)
			}
		})
	}
}

func TestWithin_noCurrentPage(t *testing.T) {
	sp, err := Parse(&withinRoot{}, "/", "App")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for name, ctx := range map[string]context.Context{
		"bare context": context.Background(),
		"PageContext":  sp.PageContext(context.Background()),
	} {
		got, err := Within(ctx, withinManage{})
		if got || err != nil {
			t.Errorf("%s: Within = %v, %v; want false, nil", name, got, err)
		}
	}
}
