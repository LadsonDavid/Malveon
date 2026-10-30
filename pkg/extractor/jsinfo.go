package extractor

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/skipdirs"
)

// Project-wide facts about how the JavaScript/TypeScript code makes requests,
// found before the per-file scan so a call in one file can be understood
// through a helper defined in another (found by audit, 2026-10-10: only a
// literal fetch(...) and axios.x(...) were recognised, so a call through
// api('/a') or an axios.create() instance read as "no frontend call").
type jsInfo struct {
	clients  map[string]bool    // names bound to axios.create(...) / ky.create(...) / got.extend(...)
	wrappers map[string]wrapper // functions that pass one of their parameters to fetch
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
}

var (
	clientDecl = regexp.MustCompile(`(?:const|let|var)\s+(\w+)\s*=\s*(?:await\s+)?(?:axios|ky|got)\.(?:create|extend)\s*\(`)
	// function name(a, b) {   async function name(a) {   const name = (a, b) => {   const name = a => {
	fnWithParams = regexp.MustCompile(`(?:^|\W)(?:async\s+)?function\s+(\w+)\s*\(([^)]*)\)\s*(?::\s*[^{;]+)?\{|` +
		`\b(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:function\s*\(([^)]*)\)\s*(?::\s*[^{;]+)?\{|\(([^)]*)\)\s*(?::\s*[^=;{]+)?=>\s*\{?|([\w$]+)\s*=>\s*\{?)`)
	wordPattern = regexp.MustCompile(`[A-Za-z_$][\w$]*`)
)

// collectJSInfo reads every JavaScript/TypeScript file under root once.
func collectJSInfo(root string) *jsInfo {
	info := &jsInfo{clients: map[string]bool{}, wrappers: map[string]wrapper{}, mentions: map[string]int{}, uses: map[string]int{}}
	readFile := newReadAhead(root)
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && skipdirs.Names[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		switch {
		case strings.EqualFold(filepath.Base(path), "package.json"):
			if raw, err := readFile(path); err == nil {
				info.addPackage(filepath.ToSlash(filepath.Dir(rel)), raw)
			}
			return nil
		case ext == ".prisma":
			if raw, err := readFile(path); err == nil {
				info.addPrisma(string(raw))
			}
			return nil
		}
		if templateExts[ext] {
			// Templates call functions too (onclick="save()", @click="save"):
			// count what they mention, read nothing else from them.
			if raw, err := readFile(path); err == nil {
				countWords(string(raw), info.mentions)
				countWords(string(raw), info.uses) // plain words, imports included: errs toward "used"
			}
			return nil
		}
		switch ext {
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
		default:
			return nil
		}
		raw, err := readFile(path)
		if err != nil {
			return nil
		}
		src := string(raw)
		cm := newCodeMap(src, jsSyntax)
		countWords(src, info.mentions)
		countUses(src, cm, info.uses)
		info.addDrizzle(rel, src, cm)
		if strings.Contains(src, "NestFactory") || strings.Contains(src, "RouterModule") || strings.Contains(src, "enableVersioning") {
			info.readNest(src, cm)
		}
		for _, m := range clientDecl.FindAllStringSubmatchIndex(src, -1) {
			if cm.codeAt(src, m[0]) {
				info.clients[src[m[2]:m[3]]] = true
			}
		}
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
			if idx, method, ok := fetchesParam(body, params); ok {
				info.wrappers[name] = wrapper{param: idx, method: method}
			}
		}
		return nil
	})
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
				return i, fetchMethod(readArgsList(body, m[1]-1)), true
			}
		}
	}
	return 0, "", false
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
	for _, loc := range wordPattern.FindAllStringIndex(src, -1) {
		counts[src[loc[0]:loc[1]]]++
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
