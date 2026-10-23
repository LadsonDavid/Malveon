package extractor

import (
	"regexp"
	"strings"
)

var (
	// router = APIRouter(prefix="/api")   bp = Blueprint("x", __name__, url_prefix="/api")
	pyRouterDecl = regexp.MustCompile(`(\w+)\s*=\s*(?:\w+\.)?(?:APIRouter|Blueprint)\s*\(`)
	pyPrefixKw   = regexp.MustCompile(`\b(?:url_)?prefix\s*=\s*["']([^"']*)["']`)
	// app.include_router(router, prefix="/v1")   app.register_blueprint(bp, url_prefix="/v1")
	pyMountCall = regexp.MustCompile(`\.(?:include_router|register_blueprint)\s*\(\s*(\w+)\s*,`)
)

// pyRouterPrefixes maps a router variable to the URL prefix its routes get,
// from APIRouter(prefix=...) / Blueprint(url_prefix=...) in the same file,
// plus a prefix added where the same file mounts it. A router mounted from
// another file is not followed (its prefix stays unknown).
func pyRouterPrefixes(src string, cm codeMap) (map[string]string, map[string]bool) {
	prefix := map[string]string{}
	mountedHere := map[string]bool{}
	for _, m := range pyRouterDecl.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		if kw := pyPrefixKw.FindStringSubmatch(callArgs(src, m[1]-1)); kw != nil {
			prefix[src[m[2]:m[3]]] = kw[1]
		} else {
			prefix[src[m[2]:m[3]]] = ""
		}
	}
	for _, m := range pyMountCall.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		name := src[m[2]:m[3]]
		if _, known := prefix[name]; !known {
			continue
		}
		mountedHere[name] = true
		if kw := pyPrefixKw.FindStringSubmatch(callArgs(src, strings.Index(src[m[0]:], "(")+m[0])); kw != nil {
			prefix[name] = kw[1] + prefix[name]
		}
	}
	// routers declared here that this file never mounts: whoever mounts them
	// (another file) decides their real URL.
	open := map[string]bool{}
	for name := range prefix {
		if !mountedHere[name] {
			open[name] = true
		}
	}
	return prefix, open
}

// pyReceiver is the variable a route decorator hangs on: "router" in "@router.get(".
func pyReceiver(decorator string) string {
	decorator = strings.TrimPrefix(decorator, "@")
	if i := strings.Index(decorator, "."); i > 0 {
		return decorator[:i]
	}
	return ""
}

// withPrefix puts prefix in front of a literal path argument; a path that
// isn't a plain literal is returned as it is (it stays unresolved).
func withPrefix(arg, prefix string) string {
	if prefix == "" {
		return arg
	}
	path, ok := literalPath(arg)
	if !ok {
		return arg
	}
	return `"` + strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(path, "/") + `"`
}
