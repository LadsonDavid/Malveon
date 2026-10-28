// Package features loads whatever plan/checklist file the user already
// has, in whatever common format they already keep it in. The external
// contract never changes regardless of format: Load takes a path, returns
// a flat []Feature. Every format-specific parsing detail (JSON shape,
// checklist syntax, line splitting) is hidden inside this package — no
// caller anywhere in the tool knows or cares how many formats exist.
package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
	"github.com/LadsonDavid/Malveon/pkg/skipdirs"
)

type Feature struct {
	Citations []string `json:"citations,omitempty"` // Source-column text; retained, never demanded as a local file.
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	// Source is where the line came from: "path:line" for Markdown and
	// plain-text plans, the path alone for JSON plans (that format has no
	// lines). Lets an editor point at the exact plan line.
	Source string `json:"source,omitempty"`
	// Unticked is true for a Markdown "- [ ]" line: work the plan hasn't
	// claimed as done yet. It's still checked and reported, but only a
	// BROKEN verdict on it blocks the gate; not built / half built / too
	// vague is simply "not done yet" (2026-09-29: with a pre-commit hook,
	// every unticked to-do blocked every commit). A ticked "- [x]" line, a
	// plain bullet, and JSON/text plans carry no tick and stay due now.
	Unticked bool `json:"unticked,omitempty"`

	// InSession is true for a line written during this session — the user's
	// own prompt. A file unchanged since the session started is then really
	// not done; for a plan document it proves nothing (the plan may describe
	// work finished before the session).
	InSession bool `json:"-"`

	// TableRow is true for a line read from a Markdown table row. A row that
	// names no URL, file or code name is a table of notes or values, not a task.
	TableRow  bool   `json:"-"`
	Context   string `json:"context,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Reference bool   `json:"reference,omitempty"`
}

// Words returns the feature's name split into lowercase tokens for
// matching against graph node words — with the same tokenizer the code
// side uses (graph.SplitWords), so "`PostJobModal`" or "(`/api/jobs`)"
// in a plan line can actually meet the code it names.
func (f Feature) Words() []string {
	return graph.SplitWords(f.Name)
}

// connectivityIndicatorWords signal a plan line is about the screen
// talking to the server or database. Used by ClaimsConnection, and by
// Classify so a line with one of these is never skipped as "design".
// A heuristic, stated plainly and editable in one place.
var connectivityIndicatorWords = map[string]bool{
	"api": true, "endpoint": true, "endpoints": true, "backend": true,
	"database": true, "db": true, "persist": true, "persists": true,
	"persisted": true, "persistence": true, "save": true, "saves": true,
	"saved": true, "sync": true, "syncs": true, "fetch": true, "fetches": true,
	"request": true, "requests": true, "crud": true, "server": true,
	"supabase": true, "rpc": true, "query": true, "queries": true,
	"table": true, "webhook": true, "webhooks": true, "auth": true,
	"login": true, "signin": true, "signup": true, "upload": true,
	"uploads": true, "download": true, "downloads": true, "route": true,
	"routes": true, "connect": true, "connected": true, "integration": true,
	"wire": true, "wired": true, "load": true, "loads": true,
	"submit": true, "submission": true, "store": true, "stores": true,
}

// designOnlyIndicatorWords signal pure visual/animation/copy content —
// CSS properties, styling vocabulary, motion terms. Only counts when the
// line has no path and no connectivity word (see Classify).
var designOnlyIndicatorWords = map[string]bool{
	"stroke": true, "gradient": true, "gradients": true, "hex": true,
	"padding": true, "margin": true, "font": true, "fonts": true,
	"css": true, "tailwind": true, "classname": true,
	"animation": true, "animations": true, "animate": true, "animates": true,
	"ease": true, "easing": true, "duration": true, "fade": true, "fades": true,
	"translate": true, "translates": true, "headline": true, "wording": true,
	"copy": true, "color": true, "colors": true, "colour": true, "colours": true,
	"palette": true, "icon": true, "icons": true, "svg": true, "px": true,
	"rem": true, "opacity": true, "spacing": true, "layout": true,
	"typography": true, "rounded": true, "shadow": true, "hover": true,
	"transition": true, "transitions": true,
	"token": true, "tokens": true, "card": true, "cards": true, "cardless": true,
	"banner": true, "mockup": true, "accent": true, "recolor": true, "reskin": true,
	"divider": true, "dividers": true, "restructure": true, "wrapper": true,
	"style": true, "styles": true, "styling": true, "theme": true,
	"framing": true, "phrasing": true, "plural": true, "singular": true,
}

// PlanURL is one URL a plan line names, with the HTTP method written
// right before it if there is one ("POST /api/jobs").
type PlanURL struct {
	Path   string
	Method string
}

var (
	// A URL starts with "/" and a letter, at a boundary: never mid-word
	// (src/app/x.css), never right after ")" or "]" (a Next.js folder like
	// (landing)/layout.tsx), and never a fraction (1/2).
	// Segments may be placeholders in any common style: [id] (Next.js),
	// :id (Express), <id> / <int:id> (Flask, Django), {id} / {id:int}
	// (FastAPI, Spring, .NET, Laravel, OpenAPI). Stopping at < or { cut
	// `/api/users/<id>` to /api/users, which then matched the unrelated
	// list route (found 2026-09-29).
	planURLPattern   = regexp.MustCompile(`(?:^|[^A-Za-z0-9_./)\]\\])(/[A-Za-z][\w-]*(?:/[\w\[\].:<>{}-]+)*)`)
	methodBefore     = regexp.MustCompile(`(?i)\b(GET|POST|PUT|PATCH|DELETE)\s*$`)
	fileExtAfterPath = regexp.MustCompile(`^\.[A-Za-z]`)
)

// PlanURLs returns every URL a plan line names, normalized to route
// syntax: trailing punctuation dropped, Next.js [id]/[...slug] segments
// written as :id. A match that runs straight into a file extension
// ("/layout.tsx") is part of a file path, not a URL, and is skipped.
// Caught 2026-09-23 on a real plan: `src/app/(landing)/layout.tsx` was
// read as the URL /layout.
func PlanURLs(name string) []PlanURL {
	var out []PlanURL
	for _, m := range planURLPattern.FindAllStringSubmatchIndex(name, -1) {
		if fileExtAfterPath.MatchString(name[m[3]:]) {
			continue
		}
		u := PlanURL{Path: normalizeURL(name[m[2]:m[3]])}
		if mm := methodBefore.FindStringSubmatch(strings.TrimRight(name[:m[2]], "`'\" ")); mm != nil {
			u.Method = strings.ToUpper(mm[1])
		}
		out = append(out, u)
	}
	return out
}

func normalizeURL(p string) string {
	p = strings.TrimRight(p, ".,:;)/")
	segs := strings.Split(p, "/")
	for i, s := range segs {
		switch {
		case strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"):
			inner := strings.Trim(s, "[]")
			if strings.HasPrefix(s, "[[") && strings.HasPrefix(inner, "...") {
				inner += "?" // [[...slug]]: also matches no segment
			}
			segs[i] = ":" + inner
		case strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">"): // <id>, <int:id>: the name is last
			inner := strings.Trim(s, "<>")
			segs[i] = ":" + inner[strings.LastIndex(inner, ":")+1:]
		case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"): // {id}, {id:int}: the name is first
			segs[i] = ":" + strings.TrimPrefix(strings.SplitN(strings.Trim(s, "{}"), ":", 2)[0], "*")
		}
	}
	return strings.Join(segs, "/")
}

// ClaimsConnection reports whether the line talks about the screen
// reaching the server or database (a URL, or a word like save/persist/
// fetch/api). A component that exists but never makes a call is only
// "half built" when its plan line claimed such a connection.
func ClaimsConnection(f Feature) bool {
	if len(PlanURLs(f.Name)) > 0 {
		return true
	}
	// What the line says, not what it points at: a word inside backticks is a
	// file or code name ("src/types/database.ts" lives in a folder called
	// types, it doesn't talk to a database), and "query parameters" is part of
	// a web address, not a database query.
	text := queryStringPattern.ReplaceAllString(codeSpanPattern.ReplaceAllString(f.Name, " "), " ")
	for _, w := range graph.SplitWords(text) {
		if connectivityIndicatorWords[w] {
			return true
		}
	}
	return false
}

var queryStringPattern = regexp.MustCompile(`(?i)\bquery[ -](?:string|param(?:eter)?s?)\b`)

// ExpectsOutsideCaller reports whether a line says the route it names is
// called from outside the app — a health check, a webhook, a cron job, a
// public API, "called by Stripe" — and doesn't also ask for an in-app
// caller. Only then is a route nothing in the app calls CONNECTED; every
// other uncalled route is HALF BUILT (a tester: "an endpoint that no UI
// or documented client calls is also a fail"; founder, 2026-09-25). The
// line's own words decide, never its URL: "add a GET `/api/health` route"
// names no outside caller, "add a `/api/health` health check" does.
func ExpectsOutsideCaller(f Feature) bool {
	text := strings.ToLower(codeSpanPattern.ReplaceAllString(f.Name, " "))
	text = urlishPattern.ReplaceAllString(text, " ")
	if !outsideCallerPattern.MatchString(text) {
		return false
	}
	// "called by Stripe" names an outside caller; its "called" isn't an
	// in-app one, so the outside phrases are dropped before that check.
	return !AsksForCaller(Feature{Name: outsideCallerPattern.ReplaceAllString(text, " ")})
}

var (
	codeSpanPattern      = regexp.MustCompile("`[^`]*`")
	urlishPattern        = regexp.MustCompile(`(?:^|\s)/[^\s]*`)
	outsideCallerPattern = regexp.MustCompile(`\b(?:health[ -]?checks?|healthchecks?|liveness|readiness|uptime|webhooks?|cron(?: jobs?)?|scheduled jobs?|public api|external(?:ly)?|third[- ]party|callbacks?|called by|probes?)\b`)
)

// WithoutOutsideCaller removes the words that name an outside caller, for
// text whose author doesn't get to decide that (an agent's claim about a
// user's prompt).
func WithoutOutsideCaller(s string) string {
	return outsideCallerAnyCase.ReplaceAllString(s, " ")
}

var outsideCallerAnyCase = regexp.MustCompile("(?i)" + outsideCallerPattern.String())

// AsksForCaller reports whether a line asks for something in the app to
// call what it names — a button, a form, a screen, "call it from X".
// "from" alone doesn't count — "served from route.ts" names where the
// route lives, not who calls it.
func AsksForCaller(f Feature) bool {
	for _, w := range f.Words() {
		if callerWords[w] {
			return true
		}
	}
	return false
}

var callerWords = map[string]bool{
	"call": true, "calls": true, "called": true, "calling": true, "caller": true, "callers": true,
	"button": true, "buttons": true, "click": true, "clicks": true, "clicking": true,
	"form": true, "forms": true, "submit": true, "submits": true, "submitted": true, "submitting": true,
	"fetch": true, "fetches": true, "fetched": true, "fetching": true,
	"wire": true, "wires": true, "wired": true, "wiring": true, "hook": true, "hooks": true, "hooked": true,
	"connect": true, "connects": true, "connected": true, "connecting": true,
	"frontend": true, "front": true, "client": true, "ui": true, "screen": true, "modal": true,
	"consume": true, "consumes": true, "consumed": true, "consumer": true,
}

var (
	headingPattern   = regexp.MustCompile(`^\*\*[^*]+\*\*\s*:?\s*$`)
	buildTestPattern = regexp.MustCompile(`(?i)\b(?:npm|pnpm|yarn|bun)\s+(?:run\s+)?(?:build|test|lint|typecheck|dev|install|test:\w+)\b|\b(?:run|rerun|execute)\s+(?:the\s+|all\s+|our\s+)?(?:unit\s+|full\s+|e2e\s+|integration\s+)?(?:tests?|test suite|build|linter|lint|type-?check(?:s|er)?)\b|\bunit tests?\b|\btest coverage\b|\b(?:vitest|jest|pytest|playwright|cypress)\b|\bgo test\b`)
	// allCapsSpanPattern: a `DATABASE_URL`-shaped name, which in an env line is the variable, not code.
	allCapsSpanPattern = regexp.MustCompile("`[A-Z][A-Z0-9_]*`")
	// envSetupPattern: a ".env" file, or an "environment variable" / "env var".
	envSetupPattern  = regexp.MustCompile("(?i)(?:^|[\\s`(])\\.env(?:\\.\\w+)?\\b|\\benv(?:ironment)?[ -]?(?:var(?:iable)?s?|file)\\b")
	notePattern      = regexp.MustCompile(`(?i)\b(?:stays|leaving|left alone|leave (?:it|them)|unchanged|untouched|out of scope|not part of|no changes?)\b`)
	firstWordPattern = regexp.MustCompile(`^[^A-Za-z]*([A-Za-z]+)(-[A-Za-z])?`)
)

// manualStepVerbs start a line that describes something a person does
// (open the page, check the console), not something built in code.
var manualStepVerbs = map[string]bool{
	"check": true, "open": true, "confirm": true, "verify": true, "visual": true,
	"visually": true, "review": true, "click": true, "try": true, "manually": true,
	"qa": true, "test": true, "inspect": true, "screenshot": true,
}

var (
	// metadataLabelPattern is the label opening a task's metadata bullet:
	// "**Task ID**: TASK-01", "**Priority:** P0", "`INPUT`: payload". Found
	// 2026-09-30 on an agent-written plan where these bullets were 100+ of
	// 184 plan lines.
	metadataLabelPattern = regexp.MustCompile("(?i)^[\\s*+-]*(?:\\*\\*)?`?(?:task id|agents?|skills|priority|dependencies|depends on|owner|assignee|estimate|effort|status|phase|task|type|inputs?|outputs?|verify|acceptance(?: criteria)?|deliverables?)`?(?:\\*\\*)?\\s*:")
	fileExtPattern       = regexp.MustCompile(`\.(?:tsx?|jsx?|mjs|cjs|py|go|rs|java|rb|php|cs|css|scss|less|sql|md|json|html|vue|svelte|prisma|ya?ml)$`)
	slashPathPattern     = regexp.MustCompile(`(?:^|\s)(?:[\w.()\[\]-]+/)+[\w.-]+\.(?:tsx?|jsx?|mjs|cjs|py|go|rs|java|rb|php|cs|css|scss|less|sql|md|json)\b`)
	identifierPattern    = regexp.MustCompile(`^[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*(?:\(\))?$`)
)

// namesCheckableThing reports whether a metadata line's value names
// something malveon can look up: a URL, a file with an extension, or a
// code-shaped name in backticks (camelCase, PascalCase, snake_case,
// dotted). An all-caps token (P0, TODO, WITHDRAWN) or a lowercase word
// after a label is a value, not a code name.
func namesCheckableThing(name string) bool {
	if len(PlanURLs(name)) > 0 || slashPathPattern.MatchString(name) {
		return true
	}
	for _, span := range codeSpanPattern.FindAllString(name, -1) {
		s := strings.Trim(span, "`")
		if fileExtPattern.MatchString(s) {
			return true
		}
		if identifierPattern.MatchString(s) &&
			(strings.ContainsAny(s, "_$.") || (strings.ToLower(s) != s && strings.ToUpper(s) != s)) {
			return true
		}
	}
	return false
}

// namesURLOrFile: a URL or a file path. A table cell mentioning `setTimeout` is a
// status note, not a code name to look up, so a table row needs one of these.
func namesURLOrFile(name string) bool {
	if len(PlanURLs(name)) > 0 || slashPathPattern.MatchString(name) {
		return true
	}
	for _, span := range codeSpanPattern.FindAllString(name, -1) {
		if fileExtPattern.MatchString(strings.Trim(span, "`")) {
			return true
		}
	}
	return false
}

// Classify decides whether a plan line is a task malveon checks against
// the code at all. skip=true means it isn't one, and reason says what it
// is instead (a heading, a manual step, ...) in plain words. Added
// 2026-09-23 with the CONNECTED/BROKEN/NOT BUILT/... verdicts: every line
// gets a direct answer, and "this line isn't a code task" is one of
// them — never a silent drop, never a vague "no proof".
func Classify(f Feature) (skip bool, reason string) {
	name := strings.TrimSpace(f.Name)
	switch {
	case f.Reference:
		return true, "an external reference, not a requested local implementation"
	case f.TableRow && !namesURLOrFile(name):
		return true, "a table row that names no URL, file or code name — a table of notes or values, not a task"
	case headingPattern.MatchString(name) || strings.HasSuffix(name, ":"):
		return true, "a heading — the lines under it are checked one by one"
	case buildTestPattern.MatchString(name):
		return true, "a build, test or install step — `malveon check --exec` runs your real build, lint and tests"
	case notePattern.MatchString(name):
		return true, "a note or decision, not something to build"
	// "Set `DATABASE_URL` in `.env`" is setup: `.env` is never committed, and the
	// variable is a value in a file, not a name in the code. A line that also
	// names code ("`getConfig` reads the env var") is still a code task.
	case envSetupPattern.MatchString(name) && !namesCheckableThing(allCapsSpanPattern.ReplaceAllString(name, " ")):
		return true, "an environment or setup step (a variable or a .env file), not something in the code"
	case metadataLabelPattern.MatchString(name) && !namesCheckableThing(name):
		return true, "a task's metadata (its ID, owner or inputs), not something to build"
	}
	// A manual step names no URL; a line that names one is a feature to look up,
	// and "Check-in", "Try-on" aren't verbs here.
	if m := firstWordPattern.FindStringSubmatch(name); m != nil && manualStepVerbs[strings.ToLower(m[1])] &&
		m[2] == "" && len(PlanURLs(name)) == 0 {
		return true, "a manual step for a person (open, check, confirm) — not something in the code"
	}
	// A design line: design words outnumber connection words, and it names
	// no URL. ("Tabler Icons — ... Settings, Palette, Database, Bot, ..."
	// lists icon names; one of them being "Database" doesn't make it a
	// database task.)
	if len(PlanURLs(name)) == 0 {
		design, connect := 0, 0
		// Words inside backticks name a file or code ("PostCard.tsx", "layout.tsx"):
		// they say where the change is, not what kind of change it is.
		for _, w := range graph.SplitWords(codeSpanPattern.ReplaceAllString(name, " ")) {
			if designOnlyIndicatorWords[w] {
				design++
			}
			if connectivityIndicatorWords[w] {
				connect++
			}
		}
		if design > connect {
			return true, "a design or styling change — malveon reads code, it can't see how the page looks"
		}
	}
	return false, ""
}

// Load reads path and returns its features, dispatching on file extension:
// .json is the original {id,name} array; .md/.markdown reads a checklist
// (- [ ] item / - item / 1. item); anything else (.txt or no extension)
// is treated as one feature name per line. IDs are generated from the
// name when a format doesn't supply one, de-duplicated if two lines slug
// to the same value.
func Load(path string) ([]Feature, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading features file: %w", err)
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		fs, err := parseJSON(raw)
		for i := range fs {
			fs[i].Source = path
		}
		return fs, err
	case ".md", ".markdown":
		return withGeneratedIDs(path, parseChecklistLines(string(raw))), nil
	default:
		return withGeneratedIDs(path, parsePlainTextLines(string(raw))), nil
	}
}

// LoadAll reads every path in paths (each dispatched through Load exactly
// as a single-doc run would be) and merges the results into one flat
// []Feature list — the same shape every check already consumes, so
// wiring/contract/overlap/etc. need no changes to support more than one
// plan doc per run. Merge order follows paths order; when two docs
// produce the same feature ID, the first doc to define it wins and the
// collision is reported back as a warning naming both files — never
// silently dropped, same as this package's other refuse-to-guess rules.
func LoadAll(paths []string) (features []Feature, warnings []string, err error) {
	seenFrom := map[string]string{} // feature ID -> path that first defined it
	for _, path := range paths {
		fs, loadErr := Load(path)
		if loadErr != nil {
			return nil, nil, loadErr
		}
		for _, f := range fs {
			if firstPath, dup := seenFrom[f.ID]; dup {
				warnings = append(warnings, fmt.Sprintf("skipped duplicate feature id %q from %s (already loaded from %s)", f.ID, path, firstPath))
				continue
			}
			seenFrom[f.ID] = path
			features = append(features, f)
		}
	}
	return features, warnings, nil
}

// aiToolDirs are the folders AI coding tools keep their own instructions and
// skills in. A plan.md or checklist.md in there is the tool's, not the project's.
var aiToolDirs = map[string]bool{
	".agent": true, ".agents": true, ".claude": true, ".cursor": true, ".codex": true,
	".gemini": true, ".windsurf": true, ".aider": true, ".continue": true,
}

var (
	candidateNamePattern = regexp.MustCompile(`(?i)(plan|feature|checklist|todo)`)
	candidateExts        = map[string]bool{".json": true, ".md": true, ".markdown": true, ".txt": true}
)

// Detect recurses through root (deliberately skipping skipdirs.Names, so it
// never wanders into node_modules or a build output dir and calls
// something a plan file by accident) for files that look like a plan:
// name containing "plan"/"feature"/"checklist"/"todo" with a supported
// extension. Returns sorted candidate paths, never a guess about which
// one is "the" plan — that decision is always left to the caller.
//
// Was top-level-only until 2026-09-16: a real test run against a project
// keeping its plan in docs/PLAN.md found nothing here, fell through to
// DetectByContent, and that fallback's much weaker signal (a README.md
// that happened to have checklist-looking lines) won by default — the
// exact kind of confidently-wrong result this whole tool exists to
// catch, now happening to the tool's own plan detection. Recursing here
// is the real fix: a deliberately-named file should be found wherever it
// actually lives, not just at the root.
func Detect(root string) ([]string, error) {
	var candidates []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (skipdirs.Names[d.Name()] || aiToolDirs[d.Name()]) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !candidateExts[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		if candidateNamePattern.MatchString(name) {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("looking for a plan file in %s: %w", root, err)
	}
	sort.Strings(candidates)
	return candidates, nil
}

// DetectByContent is the fallback when Detect's name match finds nothing
// — the plan could be named anything. Recurses the same way Detect does
// (see skipdirs.Names), scanning every .json/.md/.markdown file (not .txt — a
// bare text file with no name hint has no reliable content signal
// either; asking the user is the honest move there, not guessing from "a
// file with some lines in it") and checking whether its *content*
// actually looks like a plan, not just its extension:
//   - .json: parses as an array of objects, each with a non-empty
//     "name" field — the real shape this tool expects, not just "is
//     it valid JSON."
//   - .md/.markdown: contains at least 2 real checklist-syntax lines
//     (see checklistItemPattern) — a strong, specific signal, not just
//     "the file has some bullet points somewhere."
//
// Same rule as everywhere else: this only ever proposes candidates for
// the caller to confirm or choose between, never picks one silently —
// callers must treat every result from this function (unlike Detect's)
// as needing confirmation even when there's only one, since a content
// match is a guess, not a deliberate name.
func DetectByContent(root string) ([]string, error) {
	var candidates []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (skipdirs.Names[d.Name()] || aiToolDirs[d.Name()]) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".json" && ext != ".md" && ext != ".markdown" {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if contentLooksLikePlan(ext, raw) {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("looking for a plan file in %s: %w", root, err)
	}
	sort.Strings(candidates)
	return candidates, nil
}

// LooksLikePlan reports whether the .md/.markdown/.json file at path has
// the shape of a plan by content — the same test DetectByContent uses.
// Used for files attached to a prompt (3.2.22); the human still confirms.
func LooksLikePlan(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".json" && ext != ".md" && ext != ".markdown" {
		return false
	}
	raw, err := os.ReadFile(path)
	return err == nil && contentLooksLikePlan(ext, raw)
}

func contentLooksLikePlan(ext string, raw []byte) bool {
	if ext == ".json" {
		var items []map[string]any
		if json.Unmarshal(raw, &items) != nil || len(items) == 0 {
			return false
		}
		for _, item := range items {
			name, ok := item["name"].(string)
			if !ok || strings.TrimSpace(name) == "" {
				return false
			}
		}
		return true
	}
	// .md / .markdown
	return len(parseChecklist(string(raw), false)) >= 2
}

func parseJSON(raw []byte) ([]Feature, error) {
	var fs []Feature
	if err := json.Unmarshal(raw, &fs); err != nil {
		return nil, fmt.Errorf("parsing features file: %w", err)
	}
	// An item without an "id" gets one from its name, like Markdown lines do.
	// Left empty they all shared the id "" and every item after the first was
	// dropped as a duplicate.
	seen := map[string]int{}
	for _, f := range fs {
		if f.ID != "" {
			seen[f.ID]++
		}
	}
	for i := range fs {
		if fs[i].ID != "" {
			continue
		}
		id := slugify(fs[i].Name)
		for seen[id] > 0 {
			id = fmt.Sprintf("%s-%d", id, seen[id]+1)
		}
		seen[id]++
		fs[i].ID = id
	}
	return fs, nil
}

var checklistItemPattern = regexp.MustCompile(`^\s*(?:[-*]\s*\[([ xX])\]|[-*]|\d+[.)])\s+(.+)$`)

// parseChecklistLines extracts item text from Markdown checklist/list
// syntax: "- [ ] item", "- [x] item", "- item", "* item", "1. item".
// Lines that don't match any of those forms (headings, prose, blank
// lines) are skipped rather than guessed at.
func parseChecklistLines(text string) []planLine { return parseChecklist(text, true) }

// parseChecklist is parseChecklistLines; tables says whether a table row counts.
// Detecting which files are plans counts bullets only: a README with a feature
// table is not a plan.
func parseChecklist(text string, tables bool) []planLine {
	var out []planLine
	lines := strings.Split(text, "\n")
	var headings []string
	active, indent := -1, 0
	fence := false
	var sourceColumns []int
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = !fence
			active = -1
			continue
		}
		if fence {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			if level <= 6 && len(trimmed) > level && trimmed[level] == ' ' {
				for len(headings) < level {
					headings = append(headings, "")
				}
				headings = headings[:level]
				headings[level-1] = strings.TrimSpace(trimmed[level:])
				active = -1
				sourceColumns = nil
				continue
			}
		}
		context := strings.Join(headings, " / ")
		reference := referenceHeading.MatchString(context)
		if m := checklistItemPattern.FindStringSubmatch(line); m != nil {
			name := strings.TrimSpace(m[2])
			if name != "" {
				out = append(out, planLine{name: name, line: i + 1, end: i + 1, unticked: m[1] == " ", context: context, reference: reference && !explicitDeliverable.MatchString(name)})
				active = len(out) - 1
				indent = len(line) - len(strings.TrimLeft(line, " \t"))
			}
			continue
		}
		// A table's declared Source column is provenance, independently of the
		// wording of its heading. Keep it for model review, but not as a local task anchor.
		if tables && tableRowPattern.MatchString(line) && i+1 < len(lines) && tableSepPattern.MatchString(lines[i+1]) {
			sourceColumns = nil
			for col, label := range strings.Split(strings.Trim(trimmed, "|"), "|") {
				switch strings.ToLower(strings.TrimSpace(label)) {
				case "source", "sources", "citation", "citations", "reference", "references":
					sourceColumns = append(sourceColumns, col)
				}
			}
		}
		// A plan written as a table: each row is a line (found 2026-10-12: every
		// row was dropped without a word). The header row, the row right above the
		// |---| separator, and the separator itself are not.
		if tables && tableRowPattern.MatchString(line) && !tableSepPattern.MatchString(line) &&
			!(i+1 < len(lines) && tableSepPattern.MatchString(lines[i+1])) {
			if name, unticked := tableRowText(line); name != "" {
				var citations []string
				cells := strings.Split(strings.Trim(trimmed, "|"), "|")
				for _, col := range sourceColumns {
					if col < len(cells) {
						citations = append(citations, strings.TrimSpace(cells[col]))
					}
				}
				out = append(out, planLine{name: name, line: i + 1, end: i + 1, unticked: unticked, table: true, context: context, reference: reference && !explicitDeliverable.MatchString(name), citations: citations})
			}
			active = -1
			continue
		}
		if !tableRowPattern.MatchString(line) {
			sourceColumns = nil
		}
		if active >= 0 && trimmed != "" && len(line)-len(strings.TrimLeft(line, " \t")) > indent {
			out[active].name += " " + trimmed
			out[active].end = i + 1
		} else if trimmed != "" {
			active = -1
		}
	}
	return out
}

var (
	referenceHeading    = regexp.MustCompile(`(?i)\b(?:prior art|external references|references|sources|inspiration)\b`)
	explicitDeliverable = regexp.MustCompile("(?i)^(?:\\*\\*)?(?:add|create|implement|build|remove|delete|fix)\\b")
	tableRowPattern     = regexp.MustCompile(`^\s*\|.+\|\s*$`)
	tableSepPattern     = regexp.MustCompile(`^\s*\|?\s*:?-{2,}:?\s*(?:\|\s*:?-{2,}:?\s*)*\|?\s*$`)
	rowNumberCell       = regexp.MustCompile(`^#?\d*\.?$`)
)

// statusCells are the cells of a plan table that say how far a task is, not
// what it is. They are left out of the line's text; one that says "not done
// yet" makes the line unticked.
var statusCells = map[string]bool{
	"done": true, "complete": true, "completed": true, "finished": true, "✅": true, "✔": true, "✔️": true, "[x]": true, "x": true,
	"todo": true, "to do": true, "pending": true, "planned": true, "not started": true, "backlog": true, "open": true,
	"in progress": true, "wip": true, "tbd": true, "☐": true, "[ ]": true, "⬜": true,
}

var notDoneCells = map[string]bool{
	"todo": true, "to do": true, "pending": true, "planned": true, "not started": true, "backlog": true, "open": true,
	"in progress": true, "wip": true, "tbd": true, "☐": true, "[ ]": true, "⬜": true,
}

// tableRowText joins a row's task cells into one line.
func tableRowText(row string) (text string, unticked bool) {
	var cells []string
	for _, c := range strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|") {
		c = strings.TrimSpace(c)
		low := strings.ToLower(c)
		switch {
		case c == "" || rowNumberCell.MatchString(c):
		case statusCells[low]:
			unticked = unticked || notDoneCells[low]
		default:
			cells = append(cells, c)
		}
	}
	return strings.Join(cells, " — "), unticked
}

// planLine is one plan item's text and its 1-based line number.
type planLine struct {
	name      string
	line      int
	unticked  bool // a "- [ ]" checklist box
	table     bool // read from a table row
	end       int
	context   string
	reference bool
	citations []string
}

// parsePlainTextLines treats every non-empty, non-comment line as one
// feature name — the fallback for .txt or any unrecognized extension.
func parsePlainTextLines(text string) []planLine {
	var out []planLine
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, planLine{name: line, line: i + 1})
	}
	return out
}

func withGeneratedIDs(path string, lines []planLine) []Feature {
	seen := map[string]int{}
	fs := make([]Feature, 0, len(lines))
	for _, l := range lines {
		id := slugify(l.name)
		seen[id]++
		if n := seen[id]; n > 1 {
			id = fmt.Sprintf("%s-%d", id, n)
		}
		fs = append(fs, Feature{ID: id, Name: l.name, Source: fmt.Sprintf("%s:%d", path, l.line), Unticked: l.unticked, TableRow: l.table, Context: l.context, EndLine: l.end, Reference: l.reference, Citations: l.citations})
	}
	return fs
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := slugNonAlnum.ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(s, "-")
}
