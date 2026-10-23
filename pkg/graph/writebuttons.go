package graph

import (
	"sort"
	"strings"
)

// A WriteButton is a button or form whose handler, written in the same file,
// changes data: it inserts, updates or deletes in a table, or uploads to a storage
// bucket. These are the buttons worth telling a person about when the agent added
// one they did not ask for. A button that only changes what the page shows is not
// listed, and neither is one malveon cannot follow: a handler passed in from
// another file, a database function whose effect is unknown, signing out.
type WriteButton struct {
	Trigger Trigger
	Writes  []string // what the handler does to data, in words, sorted
	Targets []string // the tables and storage buckets it changes (lowercase, sorted)
}

// Key says what the button is and does, not where it sits, so moving it to another
// file or line does not make it new.
func (b WriteButton) Key() string {
	return strings.ToLower(b.Trigger.Tag) + "|" + b.Trigger.Event + "|" + b.Trigger.Label + "|" + strings.Join(b.Writes, ";")
}

// DataWrite says in words what a database call changes, and reports false for
// anything that is not provably a change (a read, an auth call, a database function
// whose effect is unknown). The same operations the plan-item check counts as
// changes, minus signing in and out, which every app has.
func DataWrite(n Node) (string, bool) {
	if n.Kind != DataCall {
		return "", false
	}
	switch n.Method {
	case "TABLE":
		switch n.Op {
		case "insert":
			return "inserts into table `" + n.Path + "`", true
		case "update":
			return "updates table `" + n.Path + "`", true
		case "upsert":
			return "upserts into table `" + n.Path + "`", true
		case "delete":
			return "deletes from table `" + n.Path + "`", true
		}
	case "STORAGE":
		switch n.Op {
		case "upload", "update", "move", "copy", "remove":
			return "changes storage bucket `" + n.Path + "` (`" + n.Op + "`)", true
		}
	case "SQL":
		switch n.Op {
		case "write":
			return "runs raw SQL that changes the database", true
		case "insert", "update", "delete":
			return "runs a raw SQL " + strings.ToUpper(n.Op) + " (table not named)", true
		}
	}
	return "", false
}

// isActionTag: a button, a form, or a component named like one (IconButton).
func isActionTag(tag string) bool {
	lower := strings.ToLower(tag)
	return lower == "form" || strings.HasSuffix(lower, "button")
}

// handlerLines is where the code a trigger runs sits in its own file: the function
// written in place, or the named function defined in the same file. false when it
// cannot be told, which is never a reason to list the button.
func (g *Graph) handlerLines(t Trigger) (from, to int, ok bool) {
	switch t.Handler.Kind {
	case HandlerInline:
		return t.Handler.From, t.Handler.To, t.Handler.To >= t.Handler.From && t.Handler.To > 0
	case HandlerNamed:
		for _, d := range g.Defs[t.Handler.Name] {
			if d.File == t.File && d.End >= d.Line && d.End > 0 {
				return d.Line, d.End, true
			}
		}
	}
	return 0, 0, false
}

// DataWriteButtons lists, in file and line order, the buttons and forms whose handler
// provably changes data.
func (g *Graph) DataWriteButtons() []WriteButton {
	var out []WriteButton
	for _, t := range g.Triggers {
		if !isActionTag(t.Tag) {
			continue
		}
		from, to, ok := g.handlerLines(t)
		if !ok {
			continue
		}
		seen := map[string]bool{}
		targets := map[string]bool{}
		var writes []string
		for _, n := range g.NodesInFile(t.File) {
			if n.Line < from || n.Line > to {
				continue
			}
			if w, ok := DataWrite(n); ok {
				if !seen[w] {
					seen[w] = true
					writes = append(writes, w)
				}
				if n.Path != "" {
					targets[strings.ToLower(n.Path)] = true
				}
			}
		}
		if len(writes) == 0 {
			continue
		}
		sort.Strings(writes)
		var names []string
		for name := range targets {
			names = append(names, name)
		}
		sort.Strings(names)
		out = append(out, WriteButton{Trigger: t, Writes: writes, Targets: names})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Trigger, out[j].Trigger
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return out
}
