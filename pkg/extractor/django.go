package extractor

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Django, read from the URL configuration, not guessed from view names.
//
// A route is a `path()`/`re_path()` entry; `include()` joins another URLconf
// (a module, an inline list, or a DRF router) under a prefix. The URLconf that
// ROOT_URLCONF names is where it starts. Each view's HTTP methods come from what
// the code says (an `@api_view([...])` decorator, a `require_POST`, the
// `get`/`post` methods a class defines, a DRF ViewSet's base classes); a view
// whose methods can't be read is registered with no method, never a guessed one.

// ---------------------------------------------------------------------------
// Python classes and functions, project-wide
// ---------------------------------------------------------------------------

type pyClass struct {
	file    string
	bases   []string        // simple names: "ModelViewSet" for viewsets.ModelViewSet
	methods map[string]bool // names defined in the class body
	actions []drfAction
}

type drfAction struct {
	name    string
	detail  bool
	methods []string
	urlPath string
	read    bool // false when url_path or methods weren't plain literals
}

type pyFunc struct {
	file    string
	methods []string // an HTTP-method decorator restricts it to these; nil = unrestricted
}

// pyIndex is every top-level function and class the project defines in Python,
// by name. A name defined in several places is ambiguous and never resolved.
type pyIndex struct {
	classes map[string][]pyClass
	funcs   map[string][]pyFunc
}

func newPyIndex() *pyIndex {
	return &pyIndex{classes: map[string][]pyClass{}, funcs: map[string][]pyFunc{}}
}

var (
	pyClassLine    = regexp.MustCompile(`^class\s+(\w+)\s*(?:\(|:)`)
	pyIndexDefLine = regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(`)
)

// indexPython records the classes and functions of one Python file.
func (ix *pyIndex) indexPython(rel, src string) {
	cm := newCodeMap(src, pySyntax)
	lines := strings.Split(src, "\n")
	offs := make([]int, len(lines)+1)
	for i, l := range lines {
		offs[i+1] = offs[i] + len(l) + 1
	}

	var pending []string // decorators waiting for the def/class below them
	var cur *pyClass
	curName, curIndent := "", -1
	flush := func() {
		if cur != nil {
			ix.classes[curName] = append(ix.classes[curName], *cur)
			cur = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		indent := len(line) - len(trimmed)
		if !cm.codeAt(src, offs[i]+indent) {
			continue
		}
		if cur != nil && indent <= curIndent {
			flush()
		}
		switch {
		case strings.HasPrefix(trimmed, "@"):
			text, next := joinBalanced(lines, i)
			pending = append(pending, strings.TrimSpace(text))
			i = next
		case indent == 0 && pyClassLine.MatchString(trimmed):
			flush()
			text, next := joinBalanced(lines, i)
			m := pyClassLine.FindStringSubmatch(trimmed)
			cur = &pyClass{file: rel, bases: classBases(text), methods: map[string]bool{}}
			curName, curIndent = m[1], 0
			pending = nil
			i = next
		case pyIndexDefLine.MatchString(trimmed):
			name := pyIndexDefLine.FindStringSubmatch(trimmed)[1]
			switch {
			case cur != nil && indent > curIndent:
				cur.methods[name] = true
				for _, d := range pending {
					if act, ok := parseAction(d, name); ok {
						cur.actions = append(cur.actions, act)
					}
				}
			case indent == 0:
				ix.funcs[name] = append(ix.funcs[name], pyFunc{file: rel, methods: decoratorMethods(pending)})
			}
			pending = nil
		default:
			pending = nil
		}
	}
	flush()
}

// joinBalanced returns the text of line i joined with the following lines until
// its brackets close (a decorator or class header can span lines), and the index
// of its last line.
func joinBalanced(lines []string, i int) (string, int) {
	text, depth := "", 0
	for j := i; j < len(lines); j++ {
		l := strings.TrimRight(lines[j], "\r")
		text += l + "\n"
		var quote byte
		for k := 0; k < len(l); k++ {
			c := l[k]
			switch {
			case quote != 0:
				if c == '\\' {
					k++
				} else if c == quote {
					quote = 0
				}
			case c == '"' || c == '\'':
				quote = c
			case c == '#':
				k = len(l)
			case c == '(' || c == '[' || c == '{':
				depth++
			case c == ')' || c == ']' || c == '}':
				depth--
			}
		}
		if depth <= 0 {
			return text, j
		}
	}
	return text, len(lines) - 1
}

// classBases lists the base classes of a `class X(A, b.B, metaclass=M):` header
// by their last name.
func classBases(header string) []string {
	open := strings.IndexByte(header, '(')
	if open < 0 {
		return nil
	}
	var out []string
	for _, a := range splitTopLevelArgs(readArgsList(header, open)) {
		a = strings.TrimSpace(a)
		if a == "" || strings.Contains(a, "=") {
			continue
		}
		if i := strings.LastIndexByte(a, '.'); i >= 0 {
			a = a[i+1:]
		}
		out = append(out, a)
	}
	return out
}

var (
	pyListStrings = regexp.MustCompile(`["']([A-Za-z]+)["']`)
	boolKw        = regexp.MustCompile(`\bdetail\s*=\s*(True|False)`)
	urlPathKw     = regexp.MustCompile(`\burl_path\s*=\s*(["'][^"']*["'])`)
	methodsKw     = regexp.MustCompile(`\bmethods\s*=\s*([\[(][^\])]*[\])])`)
)

// parseAction reads `@action(detail=True, methods=["post"], url_path="x")`.
func parseAction(decorator, fn string) (drfAction, bool) {
	d := strings.TrimPrefix(decorator, "@")
	d = strings.TrimPrefix(d, "decorators.")
	if !strings.HasPrefix(d, "action") {
		return drfAction{}, false
	}
	args := ""
	if open := strings.IndexByte(d, '('); open >= 0 {
		args = readArgsList(d, open)
	}
	act := drfAction{name: fn, methods: []string{"GET"}, urlPath: fn, read: true}
	if m := boolKw.FindStringSubmatch(args); m != nil {
		act.detail = m[1] == "True"
	} else {
		act.read = false
	}
	if m := methodsKw.FindStringSubmatch(args); m != nil {
		act.methods = nil
		for _, s := range pyListStrings.FindAllStringSubmatch(m[1], -1) {
			act.methods = append(act.methods, strings.ToUpper(s[1]))
		}
	} else if strings.Contains(args, "methods") {
		act.read = false
	}
	if m := urlPathKw.FindStringSubmatch(args); m != nil {
		act.urlPath, _ = pyStr(m[1])
	} else if strings.Contains(args, "url_path") {
		act.read = false
	}
	return act, true
}

// decoratorMethods reads the HTTP methods a view function allows from the
// decorators above it; nil when none restricts it.
func decoratorMethods(decorators []string) []string {
	for _, d := range decorators {
		d = strings.TrimPrefix(d, "@")
		name := d
		if i := strings.IndexByte(name, '('); i >= 0 {
			name = name[:i]
		}
		name = strings.TrimSpace(name)
		if i := strings.LastIndexByte(name, '.'); i >= 0 {
			name = name[i+1:]
		}
		var list []string
		for _, s := range pyListStrings.FindAllStringSubmatch(argText(d), -1) {
			list = append(list, strings.ToUpper(s[1]))
		}
		switch name {
		case "api_view":
			if len(list) == 0 && !strings.Contains(d, "http_method_names") && strings.TrimSpace(argText(d)) == "" {
				return []string{"GET"} // DRF's default
			}
			if len(list) > 0 {
				return list
			}
		case "require_http_methods":
			if len(list) > 0 {
				return list
			}
		case "require_GET":
			return []string{"GET"}
		case "require_POST":
			return []string{"POST"}
		case "require_safe":
			return []string{"GET", "HEAD"}
		}
	}
	return nil
}

func argText(decorator string) string {
	if open := strings.IndexByte(decorator, '('); open >= 0 {
		return readArgsList(decorator, open)
	}
	return ""
}

// pyStr reads a plain Python string literal (r"x", 'x', """x"""). f-strings are not plain.
func pyStr(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	i := 0
	for i < len(raw) && strings.ContainsRune("rRuUbB", rune(raw[i])) {
		i++
	}
	if i >= len(raw) || (raw[i] != '"' && raw[i] != '\'') {
		return "", false
	}
	q, body := raw[i], raw[i:]
	if three := strings.Repeat(string(q), 3); len(body) >= 6 && strings.HasPrefix(body, three) && strings.HasSuffix(body, three) {
		return body[3 : len(body)-3], true
	}
	if len(body) >= 2 && body[len(body)-1] == q {
		return body[1 : len(body)-1], true
	}
	return "", false
}

// ---------------------------------------------------------------------------
// URLconf files
// ---------------------------------------------------------------------------

type djEntry struct {
	line    int
	route   string // the path template, ":name" for captures
	unread  bool   // a regex or expression that couldn't be turned into a path
	view    string // a leaf: the view expression as written
	name    string // name="..." (for {% url %} lookups)
	include *djInclude
	router  string // an entry that is a router's URLs (`router.urls`, `*router.urls`)
}

type djInclude struct {
	module  string // include("shop.urls")
	router  string // include(router.urls)
	entries []djEntry
	inline  bool // entries is an inline list
}

type djReg struct {
	line    int
	prefix  string
	unread  bool
	viewset string // as written: UserViewSet, views.UserViewSet
}

type djangoFile struct {
	entries []djEntry
	routers map[string][]djReg
}

var (
	djURLPatterns = regexp.MustCompile(`(?m)^[ \t]*urlpatterns\s*(\+?=)`)
	djNamedList   = regexp.MustCompile(`(?m)^(\w+)\s*=\s*\[`)
	djRouterDecl  = regexp.MustCompile(`(?m)^[ \t]*(\w+)\s*=\s*(?:\w+\.)*(?:Default|Simple)Router\s*\(`)
	djRegister    = regexp.MustCompile(`\b(\w+)\.register\s*\(`)
	djAngle       = regexp.MustCompile(`<(?:\w+:)?(\w+)>`)
	djRootURLConf = regexp.MustCompile(`(?m)^[ \t]*ROOT_URLCONF\s*=\s*["']([\w.]+)["']`)
)

// parseURLConf reads the URL patterns of one urls.py; nil when it has none.
func parseURLConf(src string) *djangoFile {
	src = maskComments(src, pySyntax) // a commented-out path() is not a route
	cm := newCodeMap(src, pySyntax)
	file := &djangoFile{routers: map[string][]djReg{}}
	named := map[string][]djEntry{}
	for _, m := range djNamedList.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) || src[m[2]:m[3]] == "urlpatterns" {
			continue
		}
		list := readBalanced(src, m[1]-1)
		if strings.Contains(list, "path(") || strings.Contains(list, "url(") {
			named[src[m[2]:m[3]]] = parseElems(src, m[1]-1, list, named)
		}
	}
	for _, m := range djRouterDecl.FindAllStringSubmatchIndex(src, -1) {
		if cm.codeAt(src, m[0]) {
			file.routers[src[m[2]:m[3]]] = nil
		}
	}
	for _, m := range djRegister.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		router := src[m[2]:m[3]]
		if _, ok := file.routers[router]; !ok {
			continue
		}
		args := splitTopLevelArgs(readArgsList(src, m[1]-1))
		if len(args) < 2 {
			continue
		}
		reg := djReg{line: lineOf(src, m[0]), viewset: strings.TrimSpace(args[1])}
		if p, ok := pyStr(args[0]); ok && !strings.ContainsAny(p, "()[]{}|?*+^$\\") {
			reg.prefix = strings.Trim(p, "/")
		} else {
			reg.unread = true
		}
		file.routers[router] = append(file.routers[router], reg)
	}
	found := false
	for _, m := range djURLPatterns.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		found = true
		stmt := readPyStatement(src, m[1])
		file.entries = append(file.entries, parseTerms(src, m[1], stmt, named, file)...)
	}
	if !found {
		return nil
	}
	return file
}

// readPyStatement is the right-hand side of an assignment starting at from: up
// to the end of the line unless a bracket is open or the next line carries on
// with `+`.
func readPyStatement(src string, from int) string {
	depth := 0
	var quote byte
	i := from
	for ; i < len(src); i++ {
		c := src[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			i--
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == '\n' && depth <= 0:
			rest := strings.TrimLeft(src[i:], " \t\r\n")
			if !strings.HasPrefix(rest, "+") {
				return src[from:i]
			}
		}
	}
	return src[from:]
}

// parseTerms splits `[...] + router.urls + format_suffix_patterns(x)` and reads each term.
func parseTerms(src string, base int, stmt string, named map[string][]djEntry, file *djangoFile) []djEntry {
	var out []djEntry
	for _, term := range splitTop(stmt, '+') {
		term = strings.TrimSpace(term)
		switch {
		case term == "":
		case strings.HasPrefix(term, "[") || strings.HasPrefix(term, "("):
			out = append(out, parseElems(src, base, term, named)...)
		case strings.HasSuffix(term, ".urls") && !strings.Contains(term, "("):
			if r := strings.TrimSuffix(term, ".urls"); isPlainIdent(r) || plainIdent.MatchString(r) {
				out = append(out, djEntry{line: lineOf(src, base), router: r})
			}
		case strings.HasPrefix(term, "format_suffix_patterns") || strings.HasPrefix(term, "i18n_patterns"):
			if open := strings.IndexByte(term, '('); open >= 0 {
				inner := readArgsList(term, open)
				if strings.HasPrefix(term, "i18n_patterns") {
					out = append(out, parseElems(src, base, "["+inner+"]", named)...)
				} else {
					out = append(out, parseTerms(src, base, inner, named, file)...)
				}
			}
		default:
			if es, ok := named[term]; ok {
				out = append(out, es...)
			}
		}
	}
	return out
}

func isPlainIdent(s string) bool { return plainIdent.MatchString(s) }

// splitTop splits s at sep wherever it is outside brackets and strings.
func splitTop(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == sep && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// parseElems reads the elements of a list/tuple literal: path(...), re_path(...),
// url(...), *router.urls.
func parseElems(src string, base int, list string, named map[string][]djEntry) []djEntry {
	if len(list) < 2 {
		return nil
	}
	var out []djEntry
	cursor := base
	for _, el := range splitTopLevelArgs(list[1 : len(list)-1]) {
		el = strings.TrimSpace(el)
		if el == "" {
			continue
		}
		line := lineOf(src, cursor)
		if at := strings.Index(src[cursor:], firstLine(el)); at >= 0 {
			cursor += at
			line = lineOf(src, cursor)
			cursor += len(firstLine(el))
		}
		if strings.HasPrefix(el, "*") && strings.HasSuffix(el, ".urls") {
			out = append(out, djEntry{line: line, router: strings.TrimSuffix(strings.TrimPrefix(el, "*"), ".urls")})
			continue
		}
		m := djCall.FindStringSubmatch(el)
		if m == nil {
			continue
		}
		isRegex := m[1] != "path"
		open := strings.IndexByte(el, '(')
		args := splitTopLevelArgs(readArgsList(el, open))
		if len(args) < 2 {
			continue
		}
		e := djEntry{line: line}
		route, ok := pyStr(args[0])
		switch {
		case !ok:
			e.unread = true
		case isRegex:
			if r, good := djRegexRoute(route); good {
				e.route = r
			} else {
				e.unread = true
			}
		default:
			e.route = djAngle.ReplaceAllString(route, ":$1")
		}
		view := strings.TrimSpace(args[1])
		for _, a := range args[2:] {
			if kv := strings.SplitN(strings.TrimSpace(a), "=", 2); len(kv) == 2 && strings.TrimSpace(kv[0]) == "name" {
				e.name, _ = pyStr(kv[1])
			}
		}
		if strings.HasPrefix(view, "include") {
			e.include = parseInclude(src, base, view, named)
		} else {
			e.view = view
		}
		out = append(out, e)
	}
	return out
}

var djCall = regexp.MustCompile(`^(path|re_path|url)\s*\(`)

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// parseInclude reads include("a.urls"), include(router.urls), include([...]),
// include((patterns, "app")) and include(some_list).
func parseInclude(src string, base int, expr string, named map[string][]djEntry) *djInclude {
	open := strings.IndexByte(expr, '(')
	args := splitTopLevelArgs(readArgsList(expr, open))
	if len(args) == 0 {
		return nil
	}
	first := strings.TrimSpace(args[0])
	inc := &djInclude{}
	switch {
	case strings.HasPrefix(first, "("): // (patterns, "app_name")
		inner := splitTopLevelArgs(first[1 : len(first)-1])
		if len(inner) == 0 {
			return nil
		}
		first = strings.TrimSpace(inner[0])
		fallthrough
	default:
		if s, ok := pyStr(first); ok {
			inc.module = s
		} else if strings.HasSuffix(first, ".urls") {
			inc.router = strings.TrimSuffix(first, ".urls")
		} else if strings.HasPrefix(first, "[") {
			inc.entries, inc.inline = parseElems(src, base, first, named), true
		} else if es, ok := named[first]; ok {
			inc.entries, inc.inline = es, true
		} else {
			return nil
		}
	}
	return inc
}

// djRegexRoute turns a re_path() pattern into a path when it only uses literal
// text and named groups; anything else (alternation, optional parts, classes)
// can't be turned into one URL and stays unread.
func djRegexRoute(re string) (string, bool) {
	s := strings.TrimPrefix(re, "^")
	s = strings.TrimSuffix(s, "$")
	s = strings.TrimSuffix(s, "/?")
	var b strings.Builder
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "(?P<") {
			end := strings.IndexByte(s[i:], '>')
			if end < 0 {
				return "", false
			}
			name := s[i+4 : i+end]
			depth, j := 0, i
		scan:
			for ; j < len(s); j++ {
				switch s[j] {
				case '\\':
					j++
				case '(':
					depth++
				case ')':
					depth--
					if depth == 0 {
						break scan
					}
				}
			}
			if depth != 0 {
				return "", false
			}
			b.WriteString(":" + name)
			i = j + 1
			continue
		}
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && strings.ContainsRune(".-/", rune(s[i+1])):
			b.WriteByte(s[i+1])
			i += 2
		case strings.ContainsRune(`()[]{}|?*+^$\`, rune(c)):
			return "", false
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), true
}

// ---------------------------------------------------------------------------
// Resolving the whole URL configuration
// ---------------------------------------------------------------------------

type djangoProject struct {
	urlconfs  map[string]*djangoFile // urls.py files by root-relative path
	rootConfs []string               // modules ROOT_URLCONF names
	index     *pyIndex
}

func newDjangoProject() *djangoProject {
	return &djangoProject{urlconfs: map[string]*djangoFile{}, index: newPyIndex()}
}

// readPython records one Python file's classes, functions, URL patterns and root URLconf.
func (p *djangoProject) readPython(rel, src string) {
	p.index.indexPython(rel, src)
	if strings.Contains(src, "urlpatterns") {
		if f := parseURLConf(src); f != nil {
			p.urlconfs[rel] = f
		}
	}
	if strings.Contains(src, "ROOT_URLCONF") {
		cm := newCodeMap(src, pySyntax)
		for _, m := range djRootURLConf.FindAllStringSubmatchIndex(src, -1) {
			if cm.codeAt(src, m[0]) {
				p.rootConfs = append(p.rootConfs, src[m[2]:m[3]])
			}
		}
	}
}

// moduleFile finds the file of a dotted module ("shop.urls"): the file that ends
// in shop/urls.py, nearest to the file that includes it.
func moduleFile(files []string, from, module string) (string, bool) {
	rest := strings.ReplaceAll(module, ".", "/")
	var best string
	bestScore := -1
	for _, f := range files {
		slash := filepath.ToSlash(f)
		if !(slash == rest+".py" || strings.HasSuffix(slash, "/"+rest+".py") || slash == rest+"/__init__.py" || strings.HasSuffix(slash, "/"+rest+"/__init__.py")) {
			continue
		}
		score := commonPrefixSegments(slash, filepath.ToSlash(from))
		if score > bestScore {
			best, bestScore = f, score
		}
	}
	return best, best != ""
}

func commonPrefixSegments(a, b string) int {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

// apply adds a RouteHandler node for every URL the project's configuration
// serves, and returns the URL names (for {% url %} in templates).
func (p *djangoProject) apply(g *graph.Graph) map[string][]string {
	names := map[string][]string{}
	if len(p.urlconfs) == 0 {
		return names
	}
	var roots []string
	for _, mod := range p.rootConfs {
		if f, ok := moduleFile(g.Files, "", mod); ok {
			if _, isConf := p.urlconfs[f]; isConf {
				roots = append(roots, f)
			}
		}
	}
	// The URLconfs something includes aren't roots.
	included := map[string]bool{}
	var mark func(f string, depth int)
	mark = func(f string, depth int) {
		if depth > 6 {
			return
		}
		for _, e := range p.urlconfs[f].allIncludes() {
			if tf, ok := moduleFile(g.Files, f, e); ok && tf != f {
				if !included[tf] {
					included[tf] = true
					if _, isConf := p.urlconfs[tf]; isConf {
						mark(tf, depth+1)
					}
				}
			}
		}
	}
	for f := range p.urlconfs {
		mark(f, 0)
	}
	rooted := len(roots) > 0
	if !rooted {
		for f := range p.urlconfs {
			if !included[f] {
				roots = append(roots, f)
			}
		}
	}
	sort.Strings(roots)

	reachable := map[string]bool{}
	r := &djResolver{g: g, p: p, names: names, reach: reachable, unmounted: !rooted && len(roots) > 1}
	for _, root := range roots {
		r.expand(root, "", 0)
	}
	// A URLconf nothing reaches (included in a way malveon can't follow): its URL is unknown.
	var orphans []string
	for f := range p.urlconfs {
		if !reachable[f] {
			orphans = append(orphans, f)
		}
	}
	sort.Strings(orphans)
	for _, f := range orphans {
		r.unmounted = true
		r.expand(f, "", 0)
	}
	g.Nodes = append(g.Nodes, r.out...)
	return names
}

// allIncludes lists every module name this URLconf includes (inline lists too).
func (f *djangoFile) allIncludes() []string {
	var mods []string
	var walk func(es []djEntry)
	walk = func(es []djEntry) {
		for _, e := range es {
			if e.include != nil {
				if e.include.module != "" {
					mods = append(mods, e.include.module)
				}
				walk(e.include.entries)
			}
		}
	}
	walk(f.entries)
	return mods
}

type djResolver struct {
	g         *graph.Graph
	p         *djangoProject
	names     map[string][]string
	reach     map[string]bool
	unmounted bool
	out       []graph.Node
}

func (r *djResolver) expand(file, prefix string, depth int) {
	if depth > 8 {
		return
	}
	r.reach[file] = true
	conf := r.p.urlconfs[file]
	if conf == nil {
		return
	}
	r.entries(file, conf, conf.entries, prefix, depth)
}

func (r *djResolver) entries(file string, conf *djangoFile, es []djEntry, prefix string, depth int) {
	for _, e := range es {
		if e.router != "" {
			r.router(file, conf, e.router, prefix)
			continue
		}
		if e.unread {
			if e.include == nil {
				r.emit(file, e.line, "", "", false)
			}
			continue
		}
		full := joinURL(prefix, e.route)
		switch {
		case e.include != nil:
			switch inc := e.include; {
			case inc.module != "":
				if tf, ok := moduleFile(r.g.Files, file, inc.module); ok && tf != file {
					r.expand(tf, full, depth+1)
				}
			case inc.router != "":
				r.router(file, conf, inc.router, full)
			case inc.inline:
				r.entries(file, conf, inc.entries, full, depth+1)
			}
		case e.view != "" && !strings.HasSuffix(e.view, ".urls"):
			methods, known := r.p.viewMethods(e.view, file, 0)
			if !known || len(methods) == 0 {
				methods = []string{""}
			}
			for _, m := range methods {
				r.emit(file, e.line, m, full, true)
			}
			if e.name != "" {
				r.names[e.name] = append(r.names[e.name], full)
			}
		}
	}
}

func (r *djResolver) emit(file string, line int, method, path string, read bool) {
	n := graph.Node{Kind: graph.RouteHandler, File: file, Line: line, Method: method, Unmounted: r.unmounted}
	if read {
		n.Path, n.Confidence = path, graph.Extracted
		n.Words = wordsFor(file, path)
	} else {
		n.Confidence = graph.Inferred
		n.Words = wordsFor(file, "")
	}
	r.out = append(r.out, n)
}

// router expands a DRF router's registrations under prefix.
func (r *djResolver) router(file string, conf *djangoFile, name, prefix string) {
	for _, reg := range conf.routers[name] {
		if reg.unread {
			continue
		}
		base := joinURL(prefix, reg.prefix)
		cls, known := r.p.index.lookupClass(reg.viewset, file)
		std, acts, ok := viewsetActions(r.p.index, cls, known, 0)
		list := []string{""}   // unknown: one route with no method
		detail := []string{""} // ditto
		if ok {
			list, detail = nil, nil
			for _, a := range std {
				switch a {
				case "list":
					list = append(list, "GET")
				case "create":
					list = append(list, "POST")
				case "retrieve":
					detail = append(detail, "GET")
				case "update":
					detail = append(detail, "PUT")
				case "partial_update":
					detail = append(detail, "PATCH")
				case "destroy":
					detail = append(detail, "DELETE")
				}
			}
		}
		for _, m := range list {
			r.emit(file, reg.line, m, base, true)
		}
		for _, m := range detail {
			r.emit(file, reg.line, m, joinURL(base, ":pk"), true)
		}
		for _, a := range acts {
			if !a.read {
				continue
			}
			p := joinURL(base, a.urlPath)
			if a.detail {
				p = joinURL(joinURL(base, ":pk"), a.urlPath)
			}
			for _, m := range a.methods {
				r.emit(file, reg.line, m, p, true)
			}
		}
	}
}

// lookupClass finds a class by the name a URLconf wrote (`views.UserViewSet`).
// A name defined more than once is ambiguous unless one is next to the file asking.
func (ix *pyIndex) lookupClass(expr, from string) (pyClass, bool) {
	name := expr
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(strings.TrimSuffix(name, ".as_view()"))
	cs := ix.classes[name]
	switch len(cs) {
	case 0:
		return pyClass{}, false
	case 1:
		return cs[0], true
	}
	best, bestScore, tie := cs[0], -1, false
	for _, c := range cs {
		s := commonPrefixSegments(filepath.ToSlash(c.file), filepath.ToSlash(from))
		if s > bestScore {
			best, bestScore, tie = c, s, false
		} else if s == bestScore {
			tie = true
		}
	}
	return best, !tie
}

var drfStandard = []string{"list", "create", "retrieve", "update", "partial_update", "destroy"}

// viewsetActions lists the standard actions a ViewSet class offers and its
// custom @action methods. ok is false when a base class is something malveon
// can't see into (then every method is unknown).
func viewsetActions(ix *pyIndex, cls pyClass, known bool, depth int) (std []string, acts []drfAction, ok bool) {
	if !known || depth > 6 {
		return nil, nil, false
	}
	set := map[string]bool{}
	for _, name := range drfStandard {
		if cls.methods[name] {
			set[name] = true
		}
	}
	acts = append(acts, cls.actions...)
	ok = true
	for _, b := range cls.bases {
		switch b {
		case "ModelViewSet":
			for _, n := range drfStandard {
				set[n] = true
			}
		case "ReadOnlyModelViewSet":
			set["list"], set["retrieve"] = true, true
		case "ListModelMixin":
			set["list"] = true
		case "CreateModelMixin":
			set["create"] = true
		case "RetrieveModelMixin":
			set["retrieve"] = true
		case "UpdateModelMixin":
			set["update"], set["partial_update"] = true, true
		case "DestroyModelMixin":
			set["destroy"] = true
		case "GenericViewSet", "ViewSet", "GenericAPIView", "object", "ViewSetMixin":
		default:
			parent, found := ix.lookupClass(b, cls.file)
			ps, pa, pok := viewsetActions(ix, parent, found, depth+1)
			if !pok {
				return nil, nil, false
			}
			for _, n := range ps {
				set[n] = true
			}
			acts = append(acts, pa...)
		}
	}
	for _, n := range drfStandard {
		if set[n] {
			std = append(std, n)
		}
	}
	return std, acts, ok
}

// ---------------------------------------------------------------------------
// Which methods a view answers
// ---------------------------------------------------------------------------

var drfGenerics = map[string][]string{
	"ListAPIView": {"GET"}, "CreateAPIView": {"POST"}, "RetrieveAPIView": {"GET"}, "DestroyAPIView": {"DELETE"},
	"UpdateAPIView": {"PUT", "PATCH"}, "ListCreateAPIView": {"GET", "POST"},
	"RetrieveUpdateAPIView": {"GET", "PUT", "PATCH"}, "RetrieveDestroyAPIView": {"GET", "DELETE"},
	"RetrieveUpdateDestroyAPIView": {"GET", "PUT", "PATCH", "DELETE"},
	"TemplateView":                 {"GET"}, "ListView": {"GET"}, "DetailView": {"GET"},
	"FormView": {"GET", "POST"}, "CreateView": {"GET", "POST"}, "UpdateView": {"GET", "POST"},
}

var plainViewBases = map[string]bool{"View": true, "APIView": true, "object": true, "ContextMixin": true, "LoginRequiredMixin": true,
	"PermissionRequiredMixin": true, "UserPassesTestMixin": true, "AccessMixin": true}

var httpHandlerNames = []string{"get", "post", "put", "patch", "delete"}

var wrapperView = regexp.MustCompile(`^(\w+)\s*\((.*)\)$`)

// viewMethods answers which HTTP methods a view expression accepts; known is
// false when that can't be read from the code.
func (p *djangoProject) viewMethods(view, from string, depth int) (methods []string, known bool) {
	view = strings.TrimSpace(view)
	if depth > 4 || view == "" {
		return nil, false
	}
	// login_required(views.x), csrf_exempt(views.X.as_view())
	if m := wrapperView.FindStringSubmatch(view); m != nil && !strings.Contains(m[1], ".") {
		args := splitTopLevelArgs(m[2])
		if len(args) > 0 {
			return p.viewMethods(args[0], from, depth+1)
		}
		return nil, false
	}
	// Class-based: Name.as_view(...)
	if i := strings.Index(view, ".as_view"); i >= 0 {
		cls, found := p.index.lookupClass(view[:i], from)
		if !found {
			// A well-known generic view used directly: TemplateView.as_view(...)
			if ms, ok := drfGenerics[lastName(view[:i])]; ok {
				return ms, true
			}
			return nil, false
		}
		return p.classMethods(cls, 0)
	}
	// A function: views.name or name
	fns := p.index.funcs[lastName(view)]
	if len(fns) != 1 {
		return nil, false
	}
	if fns[0].methods == nil {
		return nil, false
	}
	return fns[0].methods, true
}

func lastName(expr string) string {
	expr = strings.TrimSpace(expr)
	if i := strings.LastIndexByte(expr, '.'); i >= 0 {
		return expr[i+1:]
	}
	return expr
}

// classMethods: the methods a class-based view answers, from the handlers it
// defines plus what its base classes provide.
func (p *djangoProject) classMethods(cls pyClass, depth int) ([]string, bool) {
	if depth > 6 {
		return nil, false
	}
	set := map[string]bool{}
	for _, h := range httpHandlerNames {
		if cls.methods[h] {
			set[strings.ToUpper(h)] = true
		}
	}
	for _, b := range cls.bases {
		switch {
		case plainViewBases[b]:
		case drfGenerics[b] != nil:
			for _, m := range drfGenerics[b] {
				set[m] = true
			}
		default:
			parent, found := p.index.lookupClass(b, cls.file)
			if !found {
				return nil, false // a base malveon can't see into may add methods
			}
			ms, ok := p.classMethods(parent, depth+1)
			if !ok {
				return nil, false
			}
			for _, m := range ms {
				set[m] = true
			}
		}
	}
	if len(set) == 0 {
		return nil, false
	}
	var out []string
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		if set[m] {
			out = append(out, m)
		}
	}
	return out, true
}
