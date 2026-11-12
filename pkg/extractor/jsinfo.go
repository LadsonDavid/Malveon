package extractor

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

)

// Project-wide facts about how the JavaScript/TypeScript code makes requests,
// found before the per-file scan so a call in one file can be understood
// through a helper defined in another (found by audit, 2026-10-10: only a
// literal fetch(...) and axios.x(...) were recognised, so a call through
// api('/a') or an axios.create() instance read as "no frontend call").
type jsInfo struct {
	clients  map[string]bool    // names bound to axios.create(...) / ky.create(...) / got.extend(...)
	wrappers map[string]wrapper // functions that pass one of their parameters to fetch
	// candidates: small functions that may wrap a wrapper (jpost -> api -> fetch); see wrapperchain.go
	candidates []fnCandidate
	callOnce   sync.Once
	callRe     *regexp.Regexp
	// unreferenced: function names that appear exactly once in the whole
	// project (their own declaration) — nothing calls them.
	mentions map[string]int
	// uses: how often each name is used, apart from declaring, importing or
	// listing it in an export (see uses.go). Becomes Graph.Uses.
	uses map[string]int

	deps          map[string]map[string]bool  // folder (root-relative, slashes) -> every dependency its package.json lists
	workspace     map[string]workspacePackage // package name -> where that package lives in this repo
	prismaModels  map[string]string           // client accessor (userProfile) -> model name (UserProfile)
	drizzleTables map[string]drizzleTable     // JS variable (users) -> the table it declares
	nest          nestFacts                   // what main.ts and the modules decide for every NestJS route
}

type wrapper struct {
	param  int    // which argument is the URL
	method string // "GET" unless its fetch says otherwise
	prefix string // fetch("/companion/api" + path): the text the helper puts before its parameter
}

var (
	clientDecl = regexp.MustCompile(`(?:const|let|var)\s+(\w+)\s*=\s*(?:await\s+)?(?:axios|ky|got)\.(?:create|extend)\s*\(`)
	// function name(a, b) {   async function name(a) {   const name = (a, b) => {   const name = a => {
	fnWithParams = regexp.MustCompile(`(?:^|\W)(?:async\s+)?function\s+(\w+)\s*(?:<[^()]*>)?\s*\(([^)]*)\)\s*(?::\s*[^{;]+)?\{|` +
		`\b(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:function\s*\(([^)]*)\)\s*(?::\s*[^{;]+)?\{|(?:<[^()]*>\s*)?\(([^)]*)\)\s*(?::\s*[^=;{]+)?=>\s*\{?|([\w$]+)\s*=>\s*\{?)`)
)

// collectJSInfo reads every JavaScript/TypeScript file under root once.
func collectJSInfo(root string) *jsInfo {
	info := &jsInfo{clients: map[string]bool{}, wrappers: map[string]wrapper{}, mentions: map[string]int{}, uses: map[string]int{}}
	entries, _ := walkFiles(root, false) // an unreadable folder is skipped, as it always was
	var jsEntries []fileEntry
	for _, e := range entries {
		switch {
		case strings.EqualFold(filepath.Base(e.path), "package.json"), e.ext == ".prisma", templateExts[e.ext]:
			jsEntries = append(jsEntries, e)
		case e.ext == ".js", e.ext == ".jsx", e.ext == ".ts", e.ext == ".tsx", e.ext == ".mjs", e.ext == ".cjs":
			jsEntries = append(jsEntries, e)
		}
	}
	// Each file is read and counted on its own goroutine; the results are added in walk order.
	parallelOrdered(len(jsEntries), func(i int) func() { return scanInfoFile(info, root, jsEntries[i]) }, func(_ int, apply func()) {
		if apply != nil {
			apply()
		}
	})
	info.chainWrappers()
	return info
}

// functionBody is the text of the function whose signature ends at end: a
// {...} block, or for an expression-bodied arrow function the rest of the line.
func functionBody(src string, end int) string {
	if end > 0 && src[end-1] == '{' {
		return readBalanced(src, end-1)
	}
	nl := strings.IndexByte(src[end:], '\n')
	if nl < 0 {
		return src[end:]
	}
	return src[end : end+nl]
}

// templateExts are files that can call a JavaScript function by name.
var templateExts = map[string]bool{
	".html": true, ".htm": true, ".vue": true, ".svelte": true, ".astro": true, ".ejs": true, ".hbs": true,
	".njk": true, ".jinja": true, ".j2": true, ".php": true, ".erb": true, ".twig": true, ".mdx": true,
}

var fetchCall = regexp.MustCompile(`\bfetch\s*\(`)

// fetchesParam: the body calls fetch with something built from one of the
// parameters (fetch(p), fetch(BASE + p), fetch(`${BASE}${p}`)). It returns which
// parameter, and the method fetch uses.
func fetchesParam(body, params string) (idx int, method string, ok bool) {
	idx, method, _, ok = fetchesParamPrefix(body, params)
	return
}

// fetchesParamPrefix is fetchesParam plus the literal text the helper puts before the
// parameter: fetch("/companion/api" + path) has the prefix "/companion/api".
func fetchesParamPrefix(body, params string) (idx int, method string, prefix string, ok bool) {
	var names []string
	for _, p := range strings.Split(params, ",") {
		p = strings.TrimSpace(p)
		if i := strings.IndexAny(p, ":=?"); i >= 0 {
			p = strings.TrimSpace(p[:i])
		}
		names = append(names, p)
	}
	for _, m := range fetchCall.FindAllStringIndex(body, -1) {
		arg := readFirstArg(body, m[1]-1)
		if _, literal := literalPath(arg); literal {
			continue
		}
		for i, n := range names {
			if n != "" && containsWord(arg, n) {
				return i, fetchMethod(readArgsList(body, m[1]-1)), literalPrefix(arg, n), true
			}
		}
	}
	return 0, "", "", false
}

var (
	concatPrefix   = regexp.MustCompile(`^["']([^"'$]*)["']\s*\+\s*(\w+)\s*$`)
	templatePrefix = regexp.MustCompile("^`([^`$]*)" + `\$\{\s*(\w+)\s*\}` + "`$")
)

// literalPrefix: the plain text a fetch argument puts in front of the parameter, from
// "/api" + p or `/api${p}`. Anything else in front of it (a variable, a call) gives "".
func literalPrefix(arg, param string) string {
	arg = strings.TrimSpace(arg)
	for _, re := range []*regexp.Regexp{concatPrefix, templatePrefix} {
		if m := re.FindStringSubmatch(arg); m != nil && m[2] == param {
			return m[1]
		}
	}
	return ""
}

// unreferenced: a function name found exactly once in the project is only its
// declaration; nothing calls it.
func (in *jsInfo) unreferenced(name string) bool {
	return in != nil && name != "" && in.mentions[name] == 1
}

var constString = regexp.MustCompile("(?:const|let|var)\\s+(\\w+)\\s*=\\s*(?:'([^'$]*)'|\"([^\"$]*)\"|`([^`$]*)`)\\s*;?")

// stringConsts are the single-assignment string constants of a file
// (const BASE = ” or '/v1'), for resolving `${BASE}/x`.
func stringConsts(src string, cm codeMap) map[string]string {
	out := map[string]string{}
	for _, m := range constString.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		val := ""
		for g := 2; g <= 4; g++ {
			if m[2*g] >= 0 {
				val = src[m[2*g]:m[2*g+1]]
			}
		}
		out[src[m[2]:m[3]]] = val
	}
	return out
}

var templateExpr = regexp.MustCompile(`\$\{\s*([A-Za-z_$][\w$]*)\s*\}`)

// withConsts replaces ${NAME} in a template literal by the constant's value.
func withConsts(arg string, consts map[string]string) string {
	if len(consts) == 0 || !strings.HasPrefix(arg, "`") {
		return arg
	}
	return templateExpr.ReplaceAllStringFunc(arg, func(e string) string {
		if v, ok := consts[templateExpr.FindStringSubmatch(e)[1]]; ok {
			return v
		}
		return e
	})
}

// countWords adds every identifier-shaped word in src to counts, without
// allocating a string per word (the keys are slices of src).
func countWords(src string, counts map[string]int) {
	for s, e := nextWord(src, 0); s >= 0; s, e = nextWord(src, e) {
		counts[src[s:e]]++
	}
}

// containsWord: w appears in s as a whole word (not inside a longer identifier).
func containsWord(s, w string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], w)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(w)
		if (start == 0 || !isIdent(s[start-1])) && (end == len(s) || !isIdent(s[end])) {
			return true
		}
		i = start + 1
	}
}

// maxWrapperBody is the longest function body looked at when searching for a
// helper that hands its parameter to fetch.
const maxWrapperBody = 20_000

// fnFinding is what one function declaration told collectJSInfo: it either hands a parameter
// to fetch (a request helper), or it is a small function that might wrap one.
type fnFinding struct {
	name, params, body string
	helper             wrapper
	isHelper           bool
}

// scanInfoFile reads one file for collectJSInfo and returns the function that adds what it found
// to info. The reading and the counting are the slow part and touch nothing shared; the returned
// function is run in file order.
func scanInfoFile(info *jsInfo, root string, e fileEntry) func() {
	path, ext := e.path, e.ext
	rel := filepath.ToSlash(e.rel)
	switch {
	case strings.EqualFold(filepath.Base(path), "package.json"):
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		return func() { info.addPackage(filepath.ToSlash(filepath.Dir(rel)), raw) }
	case ext == ".prisma":
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		return func() { info.addPrisma(string(raw)) }
	}
	if templateExts[ext] {
		// Templates call functions too (onclick="save()", @click="save"):
		// count what they mention, read nothing else from them.
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		words := map[string]int{}
		countWords(string(raw), words)
		return func() { // plain words, imports included: errs toward "used"
			for w, n := range words {
				info.mentions[w] += n
				info.uses[w] += n
			}
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src := string(raw)
	cm := newCodeMap(src, jsSyntax)
	mentions := map[string]int{}
	countWords(src, mentions)
	uses := map[string]int{}
	countUses(src, cm, uses)

	var clients []string
	if containsAny(src, "axios.create", "axios.extend", "ky.create", "ky.extend", "got.create", "got.extend") {
		for _, m := range clientDecl.FindAllStringSubmatchIndex(src, -1) {
			if cm.codeAt(src, m[0]) {
				clients = append(clients, src[m[2]:m[3]])
			}
		}
	}
	var fns []fnFinding
	if containsAny(src, "function", "=>") { // both alternatives need one
		for _, m := range fnWithParams.FindAllStringSubmatchIndex(src, -1) {
			if !cm.codeAt(src, m[0]) {
				continue
			}
			name, params := "", ""
			switch {
			case m[2] >= 0:
				name, params = src[m[2]:m[3]], src[m[4]:m[5]]
			case m[8] >= 0: // const f = function(a) {
				name, params = src[m[6]:m[7]], src[m[8]:m[9]]
			case m[10] >= 0:
				name, params = src[m[6]:m[7]], src[m[10]:m[11]]
			case m[12] >= 0:
				name, params = src[m[6]:m[7]], src[m[12]:m[13]]
			default:
				continue
			}
			body := functionBody(src, m[1])
			if len(body) > maxWrapperBody {
				continue // a request helper is a few lines; a huge body (a bundle's module wrapper) is rescanned at every nesting level
			}
			if idx, method, prefix, ok := fetchesParamPrefix(body, params); ok {
				fns = append(fns, fnFinding{name: name, helper: wrapper{param: idx, method: method, prefix: prefix}, isHelper: true})
			} else {
				fns = append(fns, fnFinding{name: name, params: params, body: body})
			}
		}
	}
	nest := strings.Contains(src, "NestFactory") || strings.Contains(src, "RouterModule") || strings.Contains(src, "enableVersioning")
	return func() {
		for w, n := range mentions {
			info.mentions[w] += n
		}
		for w, n := range uses {
			info.uses[w] += n
		}
		info.addDrizzle(rel, src, cm)
		if nest {
			info.readNest(src, cm)
		}
		for _, c := range clients {
			info.clients[c] = true
		}
		for _, f := range fns {
			if f.isHelper {
				info.wrappers[f.name] = f.helper
			} else {
				info.noteCandidate(f.name, f.params, f.body)
			}
		}
	}
}
