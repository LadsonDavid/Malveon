package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// More ways frontend code asks a server for something, beyond a literal
// fetch(...) and axios.x(...): a data-fetching hook, an HTML form, a call
// through an axios.create() instance, and a call through a helper that hands
// its URL parameter to fetch (api('/a')).

var (
	swrCall = regexp.MustCompile(`\b(?:useSWR|useSWRImmutable|useSWRInfinite|useQuery|\$fetch|useFetch|useLazyFetch)\s*\(`)
	// () => `/api/x/${id}` — a URL that follows reactive state (Nuxt's useFetch, SWR)
	arrowURL = regexp.MustCompile(`(?s)^(?:async\s*)?\(\s*\)\s*=>\s*\(?\s*(.+?)\s*\)?$`)
	// <form action="/api/e" method="post">
	formTag    = regexp.MustCompile(`(?is)<form\b[^>]*>`)
	formAction = regexp.MustCompile(`(?is)\baction\s*=\s*["']([^"']+)["']`)
	formMethod = regexp.MustCompile(`(?is)\bmethod\s*=\s*["'](\w+)["']`)
	clientCall = regexp.MustCompile(`(?i)\b(\w+)\.(get|post|put|delete|patch)\s*\(`)
)

func (in *jsInfo) isClient(name string) bool { return in != nil && in.clients[name] }

func scanJSShapes(src string, cm codeMap, consts map[string]string, info *jsInfo) []callSite {
	var sites []callSite

	// useSWR('/api/x'), useQuery('/api/x'), $fetch('/api/x'): a GET unless the options say otherwise.
	for _, m := range swrCall.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		args := readArgsList(src, m[1]-1)
		arg := withConsts(readFirstArg(src, m[1]-1), consts)
		if a := arrowURL.FindStringSubmatch(arg); a != nil {
			arg = withConsts(a[1], consts)
		}
		if _, ok := literalPath(arg); !ok {
			continue // useQuery(['key'], fn): not a URL
		}
		sent, _ := jsSentFieldsFromCallArgs(args, false)
		sites = append(sites, callSite{kind: graph.NetworkCall, method: fetchMethod(args), line: lineOf(src, m[0]), argRaw: arg, bodyFields: sent})
	}

	// <form action="/api/e" method="post">: the browser sends it.
	for _, m := range formTag.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		tag := src[m[0]:m[1]]
		a := formAction.FindStringSubmatch(tag)
		if a == nil || !strings.HasPrefix(a[1], "/") {
			continue
		}
		method := "GET" // the HTML default
		if mm := formMethod.FindStringSubmatch(tag); mm != nil {
			method = strings.ToUpper(mm[1])
		}
		sites = append(sites, callSite{kind: graph.NetworkCall, method: method, line: lineOf(src, m[0]), argRaw: `"` + a[1] + `"`})
	}

	// api.get('/x') where api = axios.create(...).
	if info != nil && len(info.clients) > 0 {
		for _, m := range clientCall.FindAllStringSubmatchIndex(src, -1) {
			if !cm.codeAt(src, m[0]) || !info.isClient(src[m[2]:m[3]]) {
				continue
			}
			open := m[1] - 1
			sent, _ := jsSentFieldsFromCallArgs(readArgsList(src, open), true)
			sites = append(sites, callSite{
				kind: graph.NetworkCall, method: strings.ToUpper(src[m[4]:m[5]]), line: lineOf(src, m[0]),
				argRaw: withConsts(readFirstArg(src, open), consts), bodyFields: sent,
			})
		}
	}

	// api('/a') where api hands its parameter to fetch.
	if info != nil && len(info.wrappers) > 0 {
		names := make([]string, 0, len(info.wrappers))
		for n := range info.wrappers {
			names = append(names, regexp.QuoteMeta(n))
		}
		call := regexp.MustCompile(`\b(` + strings.Join(names, "|") + `)\s*\(`)
		for _, m := range call.FindAllStringSubmatchIndex(src, -1) {
			if !cm.codeAt(src, m[0]) || declaredHere(src, m[0]) {
				continue
			}
			w := info.wrappers[src[m[2]:m[3]]]
			argsList := readArgsList(src, m[1]-1)
			args := splitTopLevelArgs(argsList)
			if w.param >= len(args) {
				continue
			}
			arg := withConsts(args[w.param], consts)
			if p, ok := literalPath(arg); !ok || !strings.HasPrefix(p, "/") {
				continue
			}
			method := w.method
			if mm := fetchMethodFieldPattern.FindStringSubmatch(argsList); mm != nil {
				method = strings.ToUpper(mm[1])
			}
			sites = append(sites, callSite{kind: graph.NetworkCall, method: method, line: lineOf(src, m[0]), argRaw: arg})
		}
	}
	return sites
}

// declaredHere: the match is the helper's own declaration (function api(...)),
// not a call to it.
func declaredHere(src string, at int) bool {
	before := strings.TrimRight(src[:at], " \t")
	return strings.HasSuffix(before, "function") || strings.HasSuffix(before, "function*") || strings.HasSuffix(before, ".")
}

// exportedDecl: the function is exported, so something outside this project
// may call it (a framework, another package) — never called "unused".
func exportedDecl(src, name string) bool {
	q := regexp.QuoteMeta(name)
	return regexp.MustCompile(`(?m)^\s*export\s+(?:default\s+)?(?:async\s+)?(?:function\s*\*?\s*|(?:const|let|var)\s+)`+q+`\b`).MatchString(src) ||
		regexp.MustCompile(`export\s*\{[^}]*\b`+q+`\b`).MatchString(src) ||
		regexp.MustCompile(`export\s+default\s+`+q+`\b`).MatchString(src) ||
		regexp.MustCompile(`(?:module\.)?exports(?:\.\w+)?\s*=.*\b`+q+`\b`).MatchString(src)
}
