package graph

import (
	"reflect"
	"testing"
)

// "The agent added a button you never asked for" is only worth saying when the button
// does something that matters. A button that changes local state is nothing to review;
// one whose handler writes to the database is. These tests pin which buttons count,
// and that none is reported on a guess.

const exportFile = "src/Export.tsx"

func writeGraph() *Graph {
	return &Graph{
		Nodes: []Node{
			{Kind: DataCall, File: exportFile, Line: 12, Method: "TABLE", Path: "exports", Op: "insert"},
			{Kind: DataCall, File: exportFile, Line: 30, Method: "TABLE", Path: "exports", Op: "select"},
			{Kind: DataCall, File: exportFile, Line: 45, Method: "TABLE", Path: "audit_log", Op: "delete"},
			{Kind: DataCall, File: exportFile, Line: 60, Method: "AUTH", Path: "signOut", Op: "signOut"},
			{Kind: DataCall, File: exportFile, Line: 70, Method: "RPC", Path: "do_thing"},
			{Kind: DataCall, File: exportFile, Line: 80, Method: "STORAGE", Path: "avatars", Op: "upload"},
			{Kind: DataCall, File: "src/Other.tsx", Line: 12, Method: "TABLE", Path: "elsewhere", Op: "insert"},
		},
		Defs: map[string][]Loc{
			"clearLog":  {{File: exportFile, Line: 44, End: 47}},
			"saveThere": {{File: "src/Other.tsx", Line: 10, End: 14}},
		},
	}
}

func writeButtons(t *testing.T, g *Graph, tr Trigger) []WriteButton {
	t.Helper()
	g.Triggers = []Trigger{tr}
	return g.DataWriteButtons()
}

func TestAButtonWhoseHandlerInsertsIsListed(t *testing.T) {
	got := writeButtons(t, writeGraph(), Trigger{File: exportFile, Line: 10, Tag: "button", Label: "Export CSV", Event: "onClick",
		Handler: Handler{Kind: HandlerInline, From: 10, To: 14}})
	if len(got) != 1 || got[0].Trigger.Label != "Export CSV" {
		t.Fatalf("got %+v", got)
	}
	if want := []string{"inserts into table `exports`"}; !reflect.DeepEqual(got[0].Writes, want) {
		t.Fatalf("writes = %q, want %q", got[0].Writes, want)
	}
	if want := []string{"exports"}; !reflect.DeepEqual(got[0].Targets, want) {
		t.Fatalf("targets = %q, want %q", got[0].Targets, want)
	}
}

func TestANamedHandlerIsFollowedWhenItIsDefinedInTheSameFile(t *testing.T) {
	got := writeButtons(t, writeGraph(), Trigger{File: exportFile, Line: 20, Tag: "button", Label: "Clear", Event: "onClick",
		Handler: Handler{Kind: HandlerNamed, Name: "clearLog"}})
	if len(got) != 1 || len(got[0].Writes) != 1 || got[0].Writes[0] != "deletes from table `audit_log`" {
		t.Fatalf("got %+v", got)
	}
}

func TestStorageUploadsCount(t *testing.T) {
	got := writeButtons(t, writeGraph(), Trigger{File: exportFile, Line: 78, Tag: "button", Label: "Upload", Event: "onClick",
		Handler: Handler{Kind: HandlerInline, From: 78, To: 82}})
	if len(got) != 1 {
		t.Fatalf("an upload to a storage bucket changes data: %+v", got)
	}
}

// Everything a button might do that malveon cannot prove is a change is left alone:
// listing a button on a guess is the false accusation this tool exists to avoid.
func TestNothingIsListedOnAGuess(t *testing.T) {
	cases := []struct {
		name string
		tr   Trigger
	}{
		{"it only reads", Trigger{Tag: "button", Label: "Refresh", Event: "onClick", Handler: Handler{Kind: HandlerInline, From: 28, To: 32}}},
		{"signing out is ordinary", Trigger{Tag: "button", Label: "Sign out", Event: "onClick", Handler: Handler{Kind: HandlerInline, From: 58, To: 62}}},
		{"a database function's effect is unknown", Trigger{Tag: "button", Label: "Run", Event: "onClick", Handler: Handler{Kind: HandlerInline, From: 68, To: 72}}},
		{"a button that submits its form is the form's business", Trigger{Tag: "button", Label: "Save", Event: "onClick", Handler: Handler{Kind: HandlerForm}}},
		{"no handler", Trigger{Tag: "button", Label: "Save", Handler: Handler{Kind: HandlerNone}}},
		{"a handler passed in from elsewhere cannot be followed", Trigger{Tag: "button", Label: "Save", Event: "onClick", Handler: Handler{Kind: HandlerUnknown}}},
		{"a named handler defined in another file", Trigger{Tag: "button", Label: "Save", Event: "onClick", Handler: Handler{Kind: HandlerNamed, Name: "saveThere"}}},
		{"a name defined nowhere", Trigger{Tag: "button", Label: "Save", Event: "onClick", Handler: Handler{Kind: HandlerNamed, Name: "ghost"}}},
		{"a link is not a button", Trigger{Tag: "a", Label: "Export", Event: "onClick", Handler: Handler{Kind: HandlerInline, From: 10, To: 14}}},
		{"an element that is not an action", Trigger{Tag: "div", Label: "Export", Event: "onClick", Handler: Handler{Kind: HandlerInline, From: 10, To: 14}}},
	}
	for _, c := range cases {
		c.tr.File, c.tr.Line = exportFile, 10
		if got := writeButtons(t, writeGraph(), c.tr); len(got) != 0 {
			t.Errorf("%s: listed %+v", c.name, got)
		}
	}
}

func TestAComponentCalledSomethingButtonAndAFormCount(t *testing.T) {
	for _, tag := range []string{"Button", "IconButton", "form", "Form"} {
		got := writeButtons(t, writeGraph(), Trigger{File: exportFile, Line: 10, Tag: tag, Label: "Go", Event: "onClick",
			Handler: Handler{Kind: HandlerInline, From: 10, To: 14}})
		if len(got) != 1 {
			t.Errorf("<%s> with a writing handler should be listed: %+v", tag, got)
		}
	}
}

// Moving a button to another file or line must not make it new, so its key says what
// it is and what it does, not where it sits.
func TestAButtonKeepsItsKeyWhenItMoves(t *testing.T) {
	g := writeGraph()
	g.Nodes = append(g.Nodes, Node{Kind: DataCall, File: "src/Moved.tsx", Line: 33, Method: "TABLE", Path: "exports", Op: "insert"})
	before := writeButtons(t, g, Trigger{File: exportFile, Line: 10, Tag: "button", Label: "Export CSV", Event: "onClick",
		Handler: Handler{Kind: HandlerInline, From: 10, To: 14}})
	after := writeButtons(t, g, Trigger{File: "src/Moved.tsx", Line: 31, Tag: "button", Label: "Export CSV", Event: "onClick",
		Handler: Handler{Kind: HandlerInline, From: 31, To: 35}})
	if len(before) != 1 || len(after) != 1 || before[0].Key() != after[0].Key() {
		t.Fatalf("keys differ: %+v vs %+v", before, after)
	}
	other := writeButtons(t, g, Trigger{File: exportFile, Line: 20, Tag: "button", Label: "Clear", Event: "onClick",
		Handler: Handler{Kind: HandlerNamed, Name: "clearLog"}})
	if other[0].Key() == before[0].Key() {
		t.Fatal("two different buttons share a key")
	}
}

func TestEveryKindOfWriteIsSaidInWords(t *testing.T) {
	for _, c := range []struct {
		n    Node
		want string
		ok   bool
	}{
		{Node{Kind: DataCall, Method: "TABLE", Path: "t", Op: "insert"}, "inserts into table `t`", true},
		{Node{Kind: DataCall, Method: "TABLE", Path: "t", Op: "update"}, "updates table `t`", true},
		{Node{Kind: DataCall, Method: "TABLE", Path: "t", Op: "upsert"}, "upserts into table `t`", true},
		{Node{Kind: DataCall, Method: "TABLE", Path: "t", Op: "delete"}, "deletes from table `t`", true},
		{Node{Kind: DataCall, Method: "TABLE", Path: "t", Op: "select"}, "", false},
		{Node{Kind: DataCall, Method: "TABLE", Path: "t", Op: ""}, "", false}, // not chained: unknown, not a write
		{Node{Kind: DataCall, Method: "STORAGE", Path: "b", Op: "upload"}, "changes storage bucket `b` (`upload`)", true},
		{Node{Kind: DataCall, Method: "STORAGE", Path: "b", Op: "list"}, "", false},
		{Node{Kind: DataCall, Method: "SQL", Op: "write"}, "runs raw SQL that changes the database", true},
		{Node{Kind: DataCall, Method: "SQL", Op: "delete"}, "runs a raw SQL DELETE (table not named)", true},
		{Node{Kind: DataCall, Method: "SQL", Op: "select"}, "", false},
		{Node{Kind: NetworkCall, Method: "TABLE", Path: "t", Op: "insert"}, "", false}, // not a database call
	} {
		got, ok := DataWrite(c.n)
		if got != c.want || ok != c.ok {
			t.Errorf("%+v: got (%q, %v), want (%q, %v)", c.n, got, ok, c.want, c.ok)
		}
	}
}

// A button is its label and what it does. Two buttons that differ in either are two buttons.
func TestTwoButtonsDifferInTheirLabelOrInWhatTheyDo(t *testing.T) {
	g := writeGraph()
	key := func(label string, from, to int) string {
		got := writeButtons(t, g, Trigger{File: exportFile, Line: from, Tag: "button", Label: label, Event: "onClick",
			Handler: Handler{Kind: HandlerInline, From: from, To: to}})
		if len(got) != 1 {
			t.Fatalf("%s: %+v", label, got)
		}
		return got[0].Key()
	}
	base := key("Export", 10, 14) // inserts into exports
	if key("Download", 10, 14) == base {
		t.Error("a different label is a different button")
	}
	if key("Export", 43, 47) == base { // deletes from audit_log
		t.Error("a different effect is a different button")
	}
}

// The code a button runs is the lines of its handler and no others.
func TestAWriteOutsideTheHandlerIsNotTheButtonsWrite(t *testing.T) {
	g := writeGraph()
	// lines 28-32 hold only a read; a delete sits at 45 and an upload at 80, outside it
	if got := writeButtons(t, g, Trigger{File: exportFile, Line: 28, Tag: "button", Label: "Look", Event: "onClick",
		Handler: Handler{Kind: HandlerInline, From: 28, To: 32}}); len(got) != 0 {
		t.Errorf("writes after the handler ended were credited to it: %+v", got)
	}
	if got := writeButtons(t, g, Trigger{File: exportFile, Line: 46, Tag: "button", Label: "Look", Event: "onClick",
		Handler: Handler{Kind: HandlerInline, From: 46, To: 50}}); len(got) != 0 {
		t.Errorf("writes before the handler began were credited to it: %+v", got)
	}
}
