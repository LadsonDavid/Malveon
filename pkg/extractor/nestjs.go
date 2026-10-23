package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// NestJS: a controller class names a path prefix (`@Controller('cats')`), each
// method a verb and its own path (`@Get(':id')`); the real URL is also shaped by
// three project-wide choices — `app.setGlobalPrefix('api')`, URI versioning
// (`app.enableVersioning`, `@Version('2')`), and `RouterModule.register`. A choice
// whose value can't be read makes the route's URL unknown (Unmounted): it is
// then matched by the end of its path, never guessed.

// nestFacts are what main.ts and the modules decide for every route.
type nestFacts struct {
	prefix         string   // setGlobalPrefix("api") -> "api"
	prefixUnknown  bool     // setGlobalPrefix(variable) or with { exclude }
	uriVersioning  bool     // enableVersioning({ type: VersioningType.URI })
	versionPrefix  string   // "v" unless the options say otherwise
	defaultVersion []string // defaultVersion, "" meaning VERSION_NEUTRAL
	versionUnknown bool     // enableVersioning(...) whose type or defaults can't be read
	routerModule   bool     // RouterModule.register(...) puts modules under paths
}

var (
	nestSetPrefix    = regexp.MustCompile(`\.\s*setGlobalPrefix\s*\(`)
	nestEnableVers   = regexp.MustCompile(`\.\s*enableVersioning\s*\(`)
	nestRouterModule = regexp.MustCompile(`\bRouterModule\s*\.\s*register\s*\(`)
	nestVersionType  = regexp.MustCompile(`\btype\s*:\s*VersioningType\.(\w+)`)
	nestVersionPref  = regexp.MustCompile(`\bprefix\s*:\s*(false|'[^']*'|"[^"]*")`)
	nestDefaultVer   = regexp.MustCompile(`\bdefaultVersion\s*:\s*(\[[^\]]*\]|'[^']*'|"[^"]*"|VERSION_NEUTRAL)`)
	nestControllerAt = regexp.MustCompile(`@Controller\b`)
	nestHTTPDeco     = map[string]string{"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE", "Patch": "PATCH", "Options": "OPTIONS", "Head": "HEAD", "All": ""}
)

// readNest collects the project-wide Nest settings from one file.
func (in *jsInfo) readNest(src string, cm codeMap) {
	src = maskComments(src, jsSyntax)
	if in.nest.versionPrefix == "" {
		in.nest.versionPrefix = "v"
	}
	if strings.Contains(src, "RouterModule") {
		for _, m := range nestRouterModule.FindAllStringIndex(src, -1) {
			if cm.codeAt(src, m[0]) {
				in.nest.routerModule = true
			}
		}
	}
	for _, m := range nestSetPrefix.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		args := splitTopLevelArgs(readArgsList(src, m[1]-1))
		p, ok := jsStr(args[0])
		if !ok || (len(args) > 1 && strings.Contains(args[1], "exclude")) {
			in.nest.prefixUnknown = true
			continue
		}
		in.nest.prefix = strings.Trim(p, "/")
	}
	for _, m := range nestEnableVers.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		opts := readArgsList(src, m[1]-1)
		t := nestVersionType.FindStringSubmatch(opts)
		switch {
		case t == nil:
			in.nest.versionUnknown = true
		case t[1] == "URI":
			in.nest.uriVersioning = true
		}
		if p := nestVersionPref.FindStringSubmatch(opts); p != nil {
			if p[1] == "false" {
				in.nest.versionPrefix = "-" // none
			} else if s, ok := jsStr(p[1]); ok {
				in.nest.versionPrefix = s
			}
		}
		if d := nestDefaultVer.FindStringSubmatch(opts); d != nil {
			list, ok := nestVersions(d[1])
			if !ok {
				in.nest.versionUnknown = true
			}
			in.nest.defaultVersion = list
		} else if strings.Contains(opts, "defaultVersion") {
			in.nest.versionUnknown = true
		}
	}
}

// jsStr reads a plain JavaScript string literal (no interpolation).
func jsStr(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 {
		return "", false
	}
	q := raw[0]
	if (q != '\'' && q != '"' && q != '`') || raw[len(raw)-1] != q {
		return "", false
	}
	inner := raw[1 : len(raw)-1]
	if q == '`' && strings.Contains(inner, "${") {
		return "", false
	}
	return inner, true
}

// jsStrList reads 'a' or ['a', 'b']; an empty argument list is one empty string.
func jsStrList(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{""}, true
	}
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		var out []string
		for _, a := range splitTopLevelArgs(raw[1 : len(raw)-1]) {
			if strings.TrimSpace(a) == "" {
				continue
			}
			s, ok := jsStr(a)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, len(out) > 0
	}
	s, ok := jsStr(raw)
	return []string{s}, ok
}

// nestVersions reads a version argument: '1', ['1', '2'], VERSION_NEUTRAL (an
// empty entry), or a list mixing them.
func nestVersions(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "VERSION_NEUTRAL" {
		return []string{""}, true
	}
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		var out []string
		for _, a := range splitTopLevelArgs(raw[1 : len(raw)-1]) {
			vs, ok := nestVersions(a)
			if !ok {
				return nil, false
			}
			out = append(out, vs...)
		}
		return out, len(out) > 0
	}
	s, ok := jsStr(raw)
	return []string{s}, ok
}

// nestMember is a class member with the decorators written above it.
type nestMember struct {
	decorators []nestDecorator
}

type nestDecorator struct {
	name string
	args string
	at   int // offset in src
}

// nestMembers walks a class body and groups each member with its decorators.
func nestMembers(src string, cm codeMap, open int) []nestMember {
	end := balancedCode(src, cm, open) // the closing brace
	var out []nestMember
	var decos []nestDecorator
	i := open + 1
	for i < end {
		c := src[i]
		if !cm.codeAt(src, i) || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ';' || c == ',' {
			i++
			continue
		}
		if c == '@' {
			j := i + 1
			for j < end && (isIdent(src[j]) || src[j] == '.') {
				j++
			}
			d := nestDecorator{name: lastName(src[i+1 : j]), at: i}
			k := j
			for k < end && (src[k] == ' ' || src[k] == '\t') {
				k++
			}
			if k < end && src[k] == '(' {
				d.args = readArgsList(src, k)
				j = k + len(readBalanced(src, k))
			}
			decos = append(decos, d)
			i = j
			continue
		}
		// A member: read to the end of its statement or its body block.
		depth := 0
		j := i
	member:
		for ; j < end; j++ {
			if !cm.codeAt(src, j) {
				continue
			}
			switch src[j] {
			case '(', '[':
				depth++
			case ')', ']':
				depth--
			case ';':
				if depth <= 0 {
					break member
				}
			case '{':
				if depth <= 0 {
					j = balancedCode(src, cm, j)
					break member
				}
				depth++
			case '}':
				depth--
			}
		}
		out = append(out, nestMember{decorators: decos})
		decos = nil
		i = j + 1
	}
	return out
}

// balancedCode returns the index of the bracket that closes the one at open,
// looking only at code: a bracket or an apostrophe in a comment or string
// doesn't count.
func balancedCode(src string, cm codeMap, open int) int {
	depth := 0
	for i := open; i < len(src) && i < len(cm); i++ {
		if !cm[i] {
			continue
		}
		switch src[i] {
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(src) - 1
}

// scanNest extracts the routes of every @Controller class in one file.
func scanNest(rel, src string, info *jsInfo) []callSite {
	if !strings.Contains(src, "@Controller") {
		return nil
	}
	if !strings.Contains(src, "@nestjs/") && !info.hasDep(rel, "@nestjs/core", "@nestjs/common") {
		return nil
	}
	src = maskComments(src, jsSyntax)
	cm := newCodeMap(src, jsSyntax)
	var sites []callSite
	for _, m := range nestControllerAt.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		after := m[1]
		var ctrlArgs string
		if k := skipSpaces(src, after); k < len(src) && src[k] == '(' {
			ctrlArgs = readArgsList(src, k)
			after = k + len(readBalanced(src, k))
		}
		cls := indexFrom(src, "class ", after)
		if cls < 0 {
			continue
		}
		open := indexByteFrom(src, '{', cls)
		if open < 0 || !cm.codeAt(src, cls) {
			continue
		}
		paths, version, hasVersion, readable := nestControllerOptions(ctrlArgs)
		for _, mem := range nestMembers(src, cm, open) {
			sites = append(sites, nestRoutes(src, info, mem, paths, readable, version, hasVersion)...)
		}
	}
	return sites
}

func skipSpaces(src string, i int) int {
	for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i++
	}
	return i
}

// nestControllerOptions reads @Controller('x'), @Controller(['x','y']) and
// @Controller({ path, version }).
func nestControllerOptions(args string) (paths []string, version []string, hasVersion, readable bool) {
	args = strings.TrimSpace(args)
	if strings.HasPrefix(args, "{") {
		paths, readable = []string{""}, true
		for _, kv := range splitTopLevelArgs(args[1 : len(args)-1]) {
			parts := strings.SplitN(strings.TrimSpace(kv), ":", 2)
			if len(parts) != 2 {
				continue
			}
			switch strings.TrimSpace(parts[0]) {
			case "path":
				paths, readable = jsStrList(parts[1])
			case "version":
				var ok bool
				version, ok = nestVersions(parts[1])
				hasVersion = true
				if !ok {
					readable = false
				}
			}
		}
		return paths, version, hasVersion, readable
	}
	paths, readable = jsStrList(args)
	return paths, nil, false, readable
}

// nestRoutes builds the route nodes of one member: one per verb decorator,
// controller path, method path and version.
func nestRoutes(src string, info *jsInfo, mem nestMember, ctrlPaths []string, ctrlReadable bool, ctrlVersion []string, ctrlHasVersion bool) []callSite {
	var methodVersion []string
	methodHasVersion, versionReadable := false, true
	for _, d := range mem.decorators {
		if d.name == "Version" {
			methodVersion, versionReadable = nestVersions(d.args)
			methodHasVersion = true
		}
	}
	var sites []callSite
	for _, d := range mem.decorators {
		verb, ok := nestHTTPDeco[d.name]
		if !ok {
			continue
		}
		line := lineOf(src, d.at)
		methodPaths, mReadable := jsStrList(d.args)
		if !ctrlReadable || !mReadable {
			sites = append(sites, callSite{kind: graph.RouteHandler, method: verb, line: line})
			continue
		}
		versions, vOK, hasAny := []string{""}, versionReadable, false
		switch {
		case methodHasVersion:
			versions, hasAny = methodVersion, true
		case ctrlHasVersion:
			versions, hasAny = ctrlVersion, true
		case len(info.nest.defaultVersion) > 0:
			versions, hasAny = info.nest.defaultVersion, true
		}
		unmounted := info.nest.prefixUnknown || info.nest.routerModule || info.nest.versionUnknown || !vOK ||
			(info.nest.uriVersioning && !hasAny) // unversioned routes are not served once versioning is on
		for _, cp := range ctrlPaths {
			for _, mp := range methodPaths {
				for _, v := range versions {
					seg := ""
					if info.nest.uriVersioning && v != "" {
						if info.nest.versionPrefix == "-" {
							seg = v
						} else {
							seg = info.nest.versionPrefix + v
						}
					}
					full := joinURL(joinURL(info.nest.prefix, seg), joinURL(cp, mp))
					for _, p := range nestPaths(full) {
						s := callSite{kind: graph.RouteHandler, method: verb, line: line, unmounted: unmounted}
						if p != "" {
							s.argRaw = `"` + p + `"`
						}
						sites = append(sites, s)
					}
				}
			}
		}
	}
	return sites
}

// nestPaths turns Nest/Express path syntax into paths: `*` and `{*name}` are
// catch-alls, `:name?` is two routes; anything else with pattern characters is
// unreadable ("").
func nestPaths(path string) []string {
	paths := []string{""}
	for _, seg := range strings.Split(strings.Trim(path, "/"), "/") {
		if seg == "" {
			continue
		}
		var add []string
		switch {
		case seg == "*":
			add = []string{":...splat?"}
		case strings.HasPrefix(seg, "{*") && strings.HasSuffix(seg, "}"):
			add = []string{":..." + seg[2:len(seg)-1]}
		case strings.HasPrefix(seg, ":") && strings.HasSuffix(seg, "?") && !strings.ContainsAny(seg[:len(seg)-1], "()[]|+*?\\"):
			paths2 := make([]string, 0, 2*len(paths))
			for _, p := range paths {
				paths2 = append(paths2, p, p+"/"+strings.TrimSuffix(seg, "?"))
			}
			paths = paths2
			continue
		case strings.ContainsAny(seg, "()[]|+*?\\"):
			return []string{""}
		default:
			add = []string{seg}
		}
		for i := range paths {
			paths[i] += "/" + add[0]
		}
	}
	for i := range paths {
		if paths[i] == "" {
			paths[i] = "/"
		}
	}
	return paths
}
